// Package update checks GitHub Releases for newer DrawGraph installers (Windows amd64 MSI
// or macOS arm64 DMG), verifies SHA-256 integrity, and applies updates cleanly.
package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/DrawGraph/internal/version"
)

// Default settings (overridable in tests or custom enterprise mirrors).
var (
	Repo    = version.Repo
	APIBase = "https://api.github.com"
)

// ErrNoRelease reports that no matching release has been published yet.
var ErrNoRelease = errors.New("no published release found")

// CheckOptions configures release discovery.
type CheckOptions struct {
	Current string
	Channel string // "stable" (default) or "prerelease"
	Repo    string
	APIBase string
	GOOS    string
	GOARCH  string
}

// Release describes a downloadable newer version for the target platform.
type Release struct {
	Version    string    `json:"version"`
	Tag        string    `json:"tag"`
	Channel    string    `json:"channel"`
	Notes      string    `json:"notes,omitempty"`
	Published  time.Time `json:"publishedAt"`
	AssetName  string    `json:"assetName"`
	AssetURL   string    `json:"assetUrl"`
	SHA256URL  string    `json:"sha256Url"`
	HTMLURL    string    `json:"htmlUrl"`
	Prerelease bool      `json:"prerelease,omitempty"`
}

// ApplyOptions configures installer execution.
type ApplyOptions struct {
	Silent   bool
	Relaunch bool
}

type apiRelease struct {
	TagName     string    `json:"tag_name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	HTMLURL     string    `json:"html_url"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
	Draft      bool `json:"draft"`
	Prerelease bool `json:"prerelease"`
}

// TargetAssetName returns the canonical GitHub release asset filename for goos/goarch.
func TargetAssetName(goos, goarch string) (string, error) {
	if goos == "" {
		goos = runtime.GOOS
	}
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	switch {
	case goos == "windows" && goarch == "amd64":
		return "DrawGraph-windows-amd64.msi", nil
	case goos == "darwin" && goarch == "arm64":
		return "DrawGraph-darwin-arm64.dmg", nil
	default:
		return "", fmt.Errorf("unsupported release target %s/%s (supported: windows/amd64, darwin/arm64)", goos, goarch)
	}
}

// Check queries GitHub Releases and returns a newer Release than opts.Current, or nil if up to date.
func Check(ctx context.Context, opts CheckOptions) (*Release, error) {
	repo := strings.TrimSpace(opts.Repo)
	if repo == "" {
		repo = Repo
	}
	base := strings.TrimRight(strings.TrimSpace(opts.APIBase), "/")
	if base == "" {
		base = APIBase
	}
	channel := strings.ToLower(strings.TrimSpace(opts.Channel))
	if channel == "" {
		channel = "stable"
	}
	cur := opts.Current
	if cur == "" {
		cur = version.Version
	}
	assetName, err := TargetAssetName(opts.GOOS, opts.GOARCH)
	if err != nil {
		return nil, err
	}
	shaName := assetName + ".sha256"

	var candidates []apiRelease
	if channel == "prerelease" {
		endpoint := fmt.Sprintf("%s/repos/%s/releases?per_page=10", base, repo)
		if err := fetchJSON(ctx, endpoint, cur, &candidates); err != nil {
			return nil, err
		}
	} else {
		endpoint := fmt.Sprintf("%s/repos/%s/releases/latest", base, repo)
		var single apiRelease
		if err := fetchJSON(ctx, endpoint, cur, &single); err != nil {
			return nil, err
		}
		candidates = []apiRelease{single}
	}

	var best *Release
	for _, rel := range candidates {
		if rel.Draft {
			continue
		}
		if channel == "stable" && rel.Prerelease {
			continue
		}
		ver := normalizeVersion(rel.TagName)
		if ver == "" {
			continue
		}
		if Compare(ver, cur) <= 0 {
			continue
		}
		if best != nil && Compare(ver, best.Version) <= 0 {
			continue
		}
		out := &Release{
			Version:    ver,
			Tag:        rel.TagName,
			Channel:    channel,
			Notes:      rel.Body,
			Published:  rel.PublishedAt,
			AssetName:  assetName,
			HTMLURL:    rel.HTMLURL,
			Prerelease: rel.Prerelease,
		}
		for _, a := range rel.Assets {
			switch a.Name {
			case assetName:
				out.AssetURL = a.BrowserDownloadURL
			case shaName:
				out.SHA256URL = a.BrowserDownloadURL
			}
		}
		if out.AssetURL == "" || out.SHA256URL == "" {
			continue
		}
		best = out
	}
	if best == nil {
		if len(candidates) == 0 {
			return nil, ErrNoRelease
		}
		// If we saw releases but none had our platform assets and were newer, check if any newer lacked assets.
		for _, rel := range candidates {
			if rel.Draft || (channel == "stable" && rel.Prerelease) {
				continue
			}
			ver := normalizeVersion(rel.TagName)
			if ver != "" && Compare(ver, cur) > 0 {
				return nil, fmt.Errorf("release %s does not include %s and %s", rel.TagName, assetName, shaName)
			}
		}
		return nil, nil
	}
	return best, nil
}

// Download fetches rel's platform installer into dir and validates its SHA-256 checksum.
// Reuses an already-downloaded verified file if present.
func Download(ctx context.Context, rel *Release, dir string, progress func(done, total int64)) (string, error) {
	if rel == nil || rel.AssetURL == "" || rel.SHA256URL == "" {
		return "", errors.New("update: missing release asset or checksum URL")
	}
	wantSum, err := fetchChecksum(ctx, rel.SHA256URL)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	destName := fmt.Sprintf("DrawGraph-%s-%s", rel.Version, strings.TrimPrefix(rel.AssetName, "DrawGraph-"))
	dest := filepath.Join(dir, destName)
	if ok, _ := fileMatchesSHA256(dest, wantSum); ok {
		if st, err := os.Stat(dest); err == nil && progress != nil {
			progress(st.Size(), st.Size())
		}
		return dest, nil
	}

	tmp := dest + ".part"
	_ = os.Remove(tmp)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rel.AssetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "DrawGraph/"+normalizeVersion(version.Version))
	client := &http.Client{Timeout: 30 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update download: http %s", resp.Status)
	}

	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64<<10)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := mw.Write(buf[:n]); werr != nil {
				_ = f.Close()
				_ = os.Remove(tmp)
				return "", werr
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = f.Close()
			_ = os.Remove(tmp)
			return "", readErr
		}
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	gotSum := hex.EncodeToString(hasher.Sum(nil))
	if !strings.EqualFold(gotSum, wantSum) {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("update checksum mismatch: got %s, want %s", gotSum, wantSum)
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	return dest, nil
}

// DefaultCacheDir returns the OS cache directory for staged update installers.
func DefaultCacheDir() string {
	if home := strings.TrimSpace(os.Getenv("DRAWGRAPH_HOME")); home != "" {
		return filepath.Join(home, "updates")
	}
	base, err := os.UserCacheDir()
	if err != nil || base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "DrawGraph", "updates")
}

func fetchJSON(ctx context.Context, url, cur string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "DrawGraph/"+normalizeVersion(cur))
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNoRelease
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("update check: http %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(dst); err != nil {
		return fmt.Errorf("update check: %w", err)
	}
	return nil
}

func fetchChecksum(ctx context.Context, url string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "DrawGraph")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("update checksum: http %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 || len(fields[0]) != 64 {
		return "", errors.New("update checksum: malformed SHA-256 file")
	}
	return strings.ToLower(fields[0]), nil
}

func fileMatchesSHA256(path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want), nil
}

func normalizeVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	return v
}

// Compare returns -1, 0, or 1 as version a is older than, equal to, or newer than b.
// Stable releases are considered newer than prereleases (e.g. 0.1.0 > 0.1.0-dev).
func Compare(a, b string) int {
	av, apre := parseVersion(a)
	bv, bpre := parseVersion(b)
	for i := 0; i < len(av) || i < len(bv); i++ {
		var x, y int
		if i < len(av) {
			x = av[i]
		}
		if i < len(bv) {
			y = bv[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	switch {
	case apre == "" && bpre != "":
		return 1
	case apre != "" && bpre == "":
		return -1
	case apre < bpre:
		return -1
	case apre > bpre:
		return 1
	}
	return 0
}

func parseVersion(v string) ([]int, string) {
	v = normalizeVersion(v)
	pre := ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, _ := strconv.Atoi(part)
		out = append(out, n)
	}
	return out, pre
}
