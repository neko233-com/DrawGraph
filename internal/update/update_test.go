package update_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/update"
)

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.1.0", 0},
		{"v0.1.0", "0.1.0-dev", 1},
		{"0.2.0", "0.1.9", 1},
		{"0.1.10", "0.1.9", 1},
		{"0.1.0-beta", "0.1.0-alpha", 1},
		{"0.1.0-dev", "0.1.0", -1},
	}
	for _, tc := range cases {
		if got := update.Compare(tc.a, tc.b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestTargetAssetNamePlatforms(t *testing.T) {
	if name, err := update.TargetAssetName("windows", "amd64"); err != nil || name != "DrawGraph-windows-amd64.msi" {
		t.Fatalf("windows/amd64 got (%q, %v)", name, err)
	}
	if name, err := update.TargetAssetName("darwin", "arm64"); err != nil || name != "DrawGraph-darwin-arm64.dmg" {
		t.Fatalf("darwin/arm64 got (%q, %v)", name, err)
	}
	if _, err := update.TargetAssetName("linux", "amd64"); err == nil {
		t.Fatal("expected error for unsupported linux/amd64 release target")
	}
}

func TestCheckDownloadVerifyChecksumAndPrereleaseChannel(t *testing.T) {
	winPayload := []byte("fake windows msi bytes")
	winSum := sha256.Sum256(winPayload)
	macPayload := []byte("fake macos dmg bytes")
	macSum := sha256.Sum256(macPayload)

	var srvURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/neko233-com/DrawGraph/releases/latest":
			fmt.Fprintf(w, `{
  "tag_name": "v0.2.0",
  "body": "Stable release",
  "html_url": "https://example.test/v0.2.0",
  "assets": [
    {"name": "DrawGraph-windows-amd64.msi", "browser_download_url": "%s/win.msi"},
    {"name": "DrawGraph-windows-amd64.msi.sha256", "browser_download_url": "%s/win.sha256"},
    {"name": "DrawGraph-darwin-arm64.dmg", "browser_download_url": "%s/mac.dmg"},
    {"name": "DrawGraph-darwin-arm64.dmg.sha256", "browser_download_url": "%s/mac.sha256"}
  ]
}`, srvURL, srvURL, srvURL, srvURL)
		case "/repos/neko233-com/DrawGraph/releases":
			fmt.Fprintf(w, `[
  {"tag_name": "v0.3.0-rc1", "prerelease": true, "body": "RC", "assets": [
    {"name": "DrawGraph-darwin-arm64.dmg", "browser_download_url": "%s/mac.dmg"},
    {"name": "DrawGraph-darwin-arm64.dmg.sha256", "browser_download_url": "%s/mac.sha256"}
  ]},
  {"tag_name": "v0.2.0", "prerelease": false, "assets": []}
]`, srvURL, srvURL)
		case "/win.msi":
			_, _ = w.Write(winPayload)
		case "/win.sha256":
			fmt.Fprintf(w, "%s  DrawGraph-windows-amd64.msi\n", hex.EncodeToString(winSum[:]))
		case "/mac.dmg":
			_, _ = w.Write(macPayload)
		case "/mac.sha256":
			fmt.Fprintf(w, "%s  DrawGraph-darwin-arm64.dmg\n", hex.EncodeToString(macSum[:]))
		case "/bad.sha256":
			fmt.Fprintln(w, strings.Repeat("0", 64)+"  bad.msi")
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	srvURL = ts.URL

	// 1. Check & download Windows MSI
	relWin, err := update.Check(context.Background(), update.CheckOptions{
		Current: "0.1.0", Channel: "stable", APIBase: ts.URL, GOOS: "windows", GOARCH: "amd64",
	})
	if err != nil || relWin == nil || relWin.Version != "0.2.0" {
		t.Fatalf("Check windows failed: rel=%+v err=%v", relWin, err)
	}
	dir := t.TempDir()
	var gotProgress int64
	msiPath, err := update.Download(context.Background(), relWin, dir, func(done, total int64) {
		gotProgress = done
	})
	if err != nil || gotProgress != int64(len(winPayload)) {
		t.Fatalf("Download win failed: path=%s prog=%d err=%v", msiPath, gotProgress, err)
	}
	if filepath.Base(msiPath) != "DrawGraph-0.2.0-windows-amd64.msi" {
		t.Fatalf("unexpected downloaded MSI name: %s", filepath.Base(msiPath))
	}

	// 2. Cached reuse on second Download call
	msiAgain, err := update.Download(context.Background(), relWin, dir, nil)
	if err != nil || msiAgain != msiPath {
		t.Fatalf("expected cached reuse, got (%s, %v)", msiAgain, err)
	}

	// 3. Checksum mismatch rejected cleanly
	badRel := *relWin
	badRel.SHA256URL = ts.URL + "/bad.sha256"
	badRel.Version = "0.2.1"
	if _, err := update.Download(context.Background(), &badRel, dir, nil); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("expected checksum mismatch error, got %v", err)
	}

	// 4. Prerelease channel picks v0.3.0-rc1 for macOS arm64
	relMac, err := update.Check(context.Background(), update.CheckOptions{
		Current: "0.2.0", Channel: "prerelease", APIBase: ts.URL, GOOS: "darwin", GOARCH: "arm64",
	})
	if err != nil || relMac == nil || relMac.Version != "0.3.0-rc1" {
		t.Fatalf("Check mac prerelease failed: rel=%+v err=%v", relMac, err)
	}
	dmgPath, err := update.Download(context.Background(), relMac, dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dmgPath)
	if string(b) != string(macPayload) {
		t.Fatalf("downloaded dmg mismatch")
	}

	// 5. Up to date returns nil, nil
	upToDate, err := update.Check(context.Background(), update.CheckOptions{
		Current: "0.2.0", Channel: "stable", APIBase: ts.URL, GOOS: "windows", GOARCH: "amd64",
	})
	if err != nil || upToDate != nil {
		t.Fatalf("expected up to date nil, got (%+v, %v)", upToDate, err)
	}
}
