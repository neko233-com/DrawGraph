// Package icons implements a user-extensible SVG icon plugin registry supporting
// built-in diagram icons, Git repository icon packs, and local SVG collections.
package icons

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo/ui"
)

//go:embed builtin/*.svg
var builtinFS embed.FS

// IconEntry describes a single SVG icon belonging to an IconPack.
type IconEntry struct {
	PackID   string   `json:"packId"`
	Name     string   `json:"name"`
	Title    string   `json:"title,omitempty"`
	Category string   `json:"category,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	RelPath  string   `json:"relPath,omitempty"`
}

// Ref returns the canonical "packId/name" reference.
func (e IconEntry) Ref() string {
	return e.PackID + "/" + e.Name
}

// IconPack describes an installed or built-in collection of SVG icons.
type IconPack struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Version     string      `json:"version,omitempty"`
	Author      string      `json:"author,omitempty"`
	Description string      `json:"description,omitempty"`
	Source      string      `json:"source"`           // "builtin", git URL, or local directory
	SourceKind  string      `json:"sourceKind"`       // "builtin", "git", "local"
	Ref         string      `json:"ref,omitempty"`    // git branch/tag
	Subdir      string      `json:"subdir,omitempty"` // optional subdirectory inside repo
	Commit      string      `json:"commit,omitempty"`
	Enabled     bool        `json:"enabled"`
	Builtin     bool        `json:"builtin,omitempty"`
	LocalDir    string      `json:"localDir,omitempty"`
	InstalledAt time.Time   `json:"installedAt,omitempty"`
	UpdatedAt   time.Time   `json:"updatedAt,omitempty"`
	Icons       []IconEntry `json:"icons"`
}

// PackManifest is the optional drawgraph-icons.json file inside an icon pack repository.
type PackManifest struct {
	SchemaVersion int    `json:"schemaVersion,omitempty"`
	MinAppVersion string `json:"minAppVersion,omitempty"`
	ID            string `json:"id,omitempty"`
	Name          string `json:"name,omitempty"`
	Version       string `json:"version,omitempty"`
	Author        string `json:"author,omitempty"`
	License       string `json:"license,omitempty"`
	Homepage      string `json:"homepage,omitempty"`
	Description   string `json:"description,omitempty"`
	IconsDir      string `json:"iconsDir,omitempty"`
	Icons         []struct {
		Name     string   `json:"name"`
		Title    string   `json:"title,omitempty"`
		Category string   `json:"category,omitempty"`
		Tags     []string `json:"tags,omitempty"`
		Path     string   `json:"path"`
	} `json:"icons,omitempty"`
}

// GitInstallOptions controls installing an icon pack from a Git repository URL.
type GitInstallOptions struct {
	URL    string
	ID     string
	Name   string
	Ref    string
	Subdir string
}

type registryFile struct {
	Version int         `json:"version"`
	Packs   []*IconPack `json:"packs"`
}

// Registry manages built-in and persistent user-installed icon plugins.
type Registry struct {
	mu       sync.RWMutex
	rootDir  string // e.g. %LOCALAPPDATA%/DrawGraph/icons
	packs    []*IconPack
	svgBytes map[string][]byte
	parsed   map[string]*ui.SVG
}

// DefaultRootDir returns the persistent user icon plugin directory across all projects.
func DefaultRootDir() string {
	if home := strings.TrimSpace(os.Getenv("DRAWGRAPH_HOME")); home != "" {
		return filepath.Join(home, "icons")
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, _ = os.UserHomeDir()
		base = filepath.Join(base, ".drawgraph")
	} else {
		base = filepath.Join(base, "DrawGraph")
	}
	return filepath.Join(base, "icons")
}

// OpenRegistry loads built-in icons and any installed external/Git icon packs under rootDir.
func OpenRegistry(rootDir string) (*Registry, error) {
	if strings.TrimSpace(rootDir) == "" {
		rootDir = DefaultRootDir()
	}
	r := &Registry{
		rootDir:  rootDir,
		svgBytes: map[string][]byte{},
		parsed:   map[string]*ui.SVG{},
	}
	if err := r.loadBuiltin(); err != nil {
		return nil, err
	}
	if err := r.loadExternal(); err != nil {
		return nil, err
	}
	return r, nil
}

// RootDir returns the directory storing user icon packs and registry.json.
func (r *Registry) RootDir() string { return r.rootDir }

func (r *Registry) registryPath() string {
	return filepath.Join(r.rootDir, "registry.json")
}

func (r *Registry) loadBuiltin() error {
	entries, err := fs.ReadDir(builtinFS, "builtin")
	if err != nil {
		return err
	}
	pack := &IconPack{
		ID:          "builtin",
		Name:        "DrawGraph Core",
		Version:     "1.0.0",
		Author:      "DrawGraph",
		Description: "Core architecture, cloud, flowchart, and system diagram icons.",
		Source:      "builtin",
		SourceKind:  "builtin",
		Enabled:     true,
		Builtin:     true,
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".svg") {
			continue
		}
		name := strings.TrimSuffix(ent.Name(), ".svg")
		raw, err := builtinFS.ReadFile("builtin/" + ent.Name())
		if err != nil {
			return err
		}
		cat, tags := builtinMeta(name)
		pack.Icons = append(pack.Icons, IconEntry{
			PackID:   pack.ID,
			Name:     name,
			Title:    humanizeName(name),
			Category: cat,
			Tags:     tags,
			RelPath:  "builtin/" + ent.Name(),
		})
		r.svgBytes[pack.ID+"/"+name] = raw
	}
	slices.SortFunc(pack.Icons, func(a, b IconEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	r.packs = append(r.packs, pack)
	return nil
}

func (r *Registry) loadExternal() error {
	data, err := os.ReadFile(r.registryPath())
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var reg registryFile
	if err := json.Unmarshal(data, &reg); err != nil {
		return fmt.Errorf("parse icon registry: %w", err)
	}
	for _, p := range reg.Packs {
		if p == nil || p.Builtin || p.ID == "builtin" {
			continue
		}
		if p.LocalDir != "" {
			if _, statErr := os.Stat(p.LocalDir); statErr == nil {
				_ = r.indexPackDir(p)
			}
		}
		r.packs = append(r.packs, p)
	}
	return nil
}

func (r *Registry) saveExternalLocked() error {
	if err := os.MkdirAll(r.rootDir, 0o755); err != nil {
		return err
	}
	reg := registryFile{Version: 1}
	for _, p := range r.packs {
		if !p.Builtin {
			reg.Packs = append(reg.Packs, p)
		}
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := r.registryPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.registryPath())
}

// InstallGit clones a Git repository containing SVG icons (with optional drawgraph-icons.json
// manifest or automatic recursive SVG discovery) and registers it globally.
func (r *Registry) InstallGit(ctx context.Context, opts GitInstallOptions) (*IconPack, error) {
	rawURL, urlRef, urlSubdir := parseGitSourceURL(opts.URL)
	if rawURL == "" {
		return nil, fmt.Errorf("git url is required")
	}
	ref := firstNonEmpty(opts.Ref, urlRef)
	subdir := firstNonEmpty(opts.Subdir, urlSubdir)
	packID := sanitizePackID(firstNonEmpty(opts.ID, inferPackIDFromURL(rawURL)))
	if packID == "" || packID == "builtin" {
		packID = "custom-git"
	}

	packDir := filepath.Join(r.rootDir, "packs", packID)
	if err := os.MkdirAll(filepath.Dir(packDir), 0o755); err != nil {
		return nil, err
	}
	_ = os.RemoveAll(packDir)

	args := []string{"clone", "--depth", "1"}
	if ref != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, rawURL, packDir)
	if out, err := runGit(ctx, "", args...); err != nil {
		return nil, fmt.Errorf("git clone %s: %w (%s)", rawURL, err, strings.TrimSpace(out))
	}

	commit, _ := runGit(ctx, packDir, "rev-parse", "--short", "HEAD")
	commit = strings.TrimSpace(commit)

	now := time.Now().UTC()
	pack := &IconPack{
		ID:          packID,
		Name:        firstNonEmpty(opts.Name, humanizeName(packID)),
		Source:      rawURL,
		SourceKind:  "git",
		Ref:         ref,
		Subdir:      subdir,
		Commit:      commit,
		Enabled:     true,
		LocalDir:    packDir,
		InstalledAt: now,
		UpdatedAt:   now,
	}
	if err := r.indexPackDir(pack); err != nil {
		_ = os.RemoveAll(packDir)
		return nil, err
	}
	if len(pack.Icons) == 0 {
		_ = os.RemoveAll(packDir)
		return nil, fmt.Errorf("no valid .svg icons found in %s", rawURL)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.upsertPackLocked(pack)
	if err := r.saveExternalLocked(); err != nil {
		return nil, err
	}
	return clonePack(pack), nil
}

// InstallLocal copies a local directory of SVG icons (or manifest pack) into the global plugin store.
func (r *Registry) InstallLocal(srcDir, id, name, subdir string) (*IconPack, error) {
	info, err := os.Stat(srcDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("icon folder %q not found", srcDir)
	}
	packID := sanitizePackID(firstNonEmpty(id, filepath.Base(srcDir)))
	if packID == "" || packID == "builtin" {
		packID = "local-icons"
	}
	packDir := filepath.Join(r.rootDir, "packs", packID)
	_ = os.RemoveAll(packDir)
	if err := copyDir(srcDir, packDir); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	pack := &IconPack{
		ID:          packID,
		Name:        firstNonEmpty(name, humanizeName(packID)),
		Source:      srcDir,
		SourceKind:  "local",
		Subdir:      subdir,
		Enabled:     true,
		LocalDir:    packDir,
		InstalledAt: now,
		UpdatedAt:   now,
	}
	if err := r.indexPackDir(pack); err != nil {
		_ = os.RemoveAll(packDir)
		return nil, err
	}
	if len(pack.Icons) == 0 {
		_ = os.RemoveAll(packDir)
		return nil, fmt.Errorf("no valid .svg icons found in %s", srcDir)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	r.upsertPackLocked(pack)
	if err := r.saveExternalLocked(); err != nil {
		return nil, err
	}
	return clonePack(pack), nil
}

// ImportSVG adds a single SVG file into a user pack (defaulting to "custom").
func (r *Registry) ImportSVG(svgPath, packID, iconName, category string) (IconEntry, error) {
	raw, err := os.ReadFile(svgPath)
	if err != nil {
		return IconEntry{}, err
	}
	if _, err := ui.ParseSVG(raw); err != nil {
		return IconEntry{}, fmt.Errorf("invalid SVG %s: %w", svgPath, err)
	}
	packID = sanitizePackID(firstNonEmpty(packID, "custom"))
	if packID == "" || packID == "builtin" {
		packID = "custom"
	}
	iconName = sanitizeIconName(firstNonEmpty(iconName, strings.TrimSuffix(filepath.Base(svgPath), filepath.Ext(svgPath))))
	if iconName == "" {
		return IconEntry{}, fmt.Errorf("invalid icon name")
	}
	packDir := filepath.Join(r.rootDir, "packs", packID)
	if err := os.MkdirAll(packDir, 0o755); err != nil {
		return IconEntry{}, err
	}
	dest := filepath.Join(packDir, iconName+".svg")
	if err := os.WriteFile(dest, raw, 0o644); err != nil {
		return IconEntry{}, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	var pack *IconPack
	for _, p := range r.packs {
		if p.ID == packID {
			pack = p
			break
		}
	}
	now := time.Now().UTC()
	if pack == nil {
		pack = &IconPack{
			ID:          packID,
			Name:        humanizeName(packID),
			Source:      packDir,
			SourceKind:  "local",
			Enabled:     true,
			LocalDir:    packDir,
			InstalledAt: now,
			UpdatedAt:   now,
		}
		r.packs = append(r.packs, pack)
	} else {
		pack.UpdatedAt = now
	}
	if err := r.indexPackDir(pack); err != nil {
		return IconEntry{}, err
	}
	if err := r.saveExternalLocked(); err != nil {
		return IconEntry{}, err
	}
	return IconEntry{
		PackID:   packID,
		Name:     iconName,
		Title:    humanizeName(iconName),
		Category: firstNonEmpty(category, "custom"),
		RelPath:  iconName + ".svg",
	}, nil
}

// SyncPack pulls latest commits for a Git-installed pack (or re-indexes a local pack).
func (r *Registry) SyncPack(ctx context.Context, packID string) (*IconPack, error) {
	r.mu.RLock()
	var target *IconPack
	for _, p := range r.packs {
		if p.ID == packID {
			target = clonePack(p)
			break
		}
	}
	r.mu.RUnlock()
	if target == nil {
		return nil, fmt.Errorf("icon pack %q not found", packID)
	}
	if target.Builtin {
		return target, nil
	}
	if target.SourceKind == "git" {
		args := []string{"fetch", "--depth", "1", "origin"}
		if target.Ref != "" {
			args = append(args, target.Ref)
		}
		if out, err := runGit(ctx, target.LocalDir, args...); err != nil {
			return nil, fmt.Errorf("git fetch %s: %w (%s)", target.ID, err, strings.TrimSpace(out))
		}
		if out, err := runGit(ctx, target.LocalDir, "reset", "--hard", "FETCH_HEAD"); err != nil {
			return nil, fmt.Errorf("git reset %s: %w (%s)", target.ID, err, strings.TrimSpace(out))
		}
		commit, _ := runGit(ctx, target.LocalDir, "rev-parse", "--short", "HEAD")
		target.Commit = strings.TrimSpace(commit)
	}
	target.UpdatedAt = time.Now().UTC()
	if err := r.indexPackDir(target); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.upsertPackLocked(target)
	if err := r.saveExternalLocked(); err != nil {
		return nil, err
	}
	return clonePack(target), nil
}

// RemovePack uninstalls a non-builtin icon pack and removes its cached files.
func (r *Registry) RemovePack(packID string) error {
	if packID == "builtin" {
		return fmt.Errorf("cannot remove builtin icon pack")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	idx := -1
	var localDir string
	for i, p := range r.packs {
		if p.ID == packID {
			idx = i
			localDir = p.LocalDir
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("icon pack %q not found", packID)
	}
	r.packs = append(r.packs[:idx], r.packs[idx+1:]...)
	prefix := packID + "/"
	for k := range r.svgBytes {
		if strings.HasPrefix(k, prefix) {
			delete(r.svgBytes, k)
		}
	}
	for k := range r.parsed {
		if strings.HasPrefix(k, prefix) {
			delete(r.parsed, k)
		}
	}
	if localDir != "" {
		_ = os.RemoveAll(localDir)
	}
	return r.saveExternalLocked()
}

// SetPackEnabled enables or disables an installed icon pack.
func (r *Registry) SetPackEnabled(packID string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.packs {
		if p.ID == packID {
			if p.Builtin && !enabled {
				return fmt.Errorf("cannot disable builtin pack")
			}
			p.Enabled = enabled
			return r.saveExternalLocked()
		}
	}
	return fmt.Errorf("icon pack %q not found", packID)
}

// Packs returns copies of all registered icon packs.
func (r *Registry) Packs() []*IconPack {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*IconPack, 0, len(r.packs))
	for _, p := range r.packs {
		out = append(out, clonePack(p))
	}
	return out
}

// Search searches across enabled icon packs matching query, optional packID, and optional category.
func (r *Registry) Search(query, packID, category string) []IconEntry {
	q := strings.ToLower(strings.TrimSpace(query))
	catFilter := strings.ToLower(strings.TrimSpace(category))
	r.mu.RLock()
	defer r.mu.RUnlock()
	var results []IconEntry
	for _, p := range r.packs {
		if !p.Enabled {
			continue
		}
		if packID != "" && p.ID != packID {
			continue
		}
		for _, ic := range p.Icons {
			if catFilter != "" && strings.ToLower(ic.Category) != catFilter {
				continue
			}
			if q == "" || matchesIconQuery(ic, q) {
				results = append(results, ic)
			}
		}
	}
	return results
}

// SVGBytes returns the raw SVG bytes for ref ("packId/name").
func (r *Registry) SVGBytes(ref string) ([]byte, bool) {
	if !strings.Contains(ref, "/") && ref != "" {
		ref = "builtin/" + ref
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.svgBytes[ref]
	return b, ok
}

// ParsedSVG returns a cached *ui.SVG for GPU rendering of ref ("packId/name").
func (r *Registry) ParsedSVG(ref string) (*ui.SVG, bool) {
	if !strings.Contains(ref, "/") && ref != "" {
		ref = "builtin/" + ref
	}
	r.mu.RLock()
	if s, ok := r.parsed[ref]; ok {
		r.mu.RUnlock()
		return s, true
	}
	raw, ok := r.svgBytes[ref]
	r.mu.RUnlock()
	if !ok {
		return nil, false
	}
	parsed, err := ui.ParseSVG(raw)
	if err != nil {
		return nil, false
	}
	r.mu.Lock()
	r.parsed[ref] = parsed
	r.mu.Unlock()
	return parsed, true
}

func (r *Registry) upsertPackLocked(pack *IconPack) {
	for i, existing := range r.packs {
		if existing.ID == pack.ID {
			r.packs[i] = pack
			return
		}
	}
	r.packs = append(r.packs, pack)
}

func (r *Registry) indexPackDir(pack *IconPack) error {
	manifestPath := filepath.Join(pack.LocalDir, "drawgraph-icons.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		alt := filepath.Join(pack.LocalDir, ".drawgraph", "icons.json")
		if _, err2 := os.Stat(alt); err2 == nil {
			manifestPath = alt
		}
	}
	var manifest *PackManifest
	if data, err := os.ReadFile(manifestPath); err == nil {
		var m PackManifest
		if err := json.Unmarshal(data, &m); err == nil {
			manifest = &m
			if m.Name != "" {
				pack.Name = m.Name
			}
			if m.Version != "" {
				pack.Version = m.Version
			}
			if m.Author != "" {
				pack.Author = m.Author
			}
			if m.Description != "" {
				pack.Description = m.Description
			}
		}
	}

	pack.Icons = nil
	seen := map[string]bool{}
	loadedBytes := map[string][]byte{}

	if manifest != nil && len(manifest.Icons) > 0 {
		for _, spec := range manifest.Icons {
			full := filepath.Join(pack.LocalDir, filepath.FromSlash(spec.Path))
			raw, err := os.ReadFile(full)
			if err != nil {
				continue
			}
			if _, err := ui.ParseSVG(raw); err != nil {
				continue
			}
			name := sanitizeIconName(spec.Name)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			pack.Icons = append(pack.Icons, IconEntry{
				PackID:   pack.ID,
				Name:     name,
				Title:    firstNonEmpty(spec.Title, humanizeName(name)),
				Category: firstNonEmpty(spec.Category, "general"),
				Tags:     spec.Tags,
				RelPath:  filepath.ToSlash(spec.Path),
			})
			loadedBytes[pack.ID+"/"+name] = raw
		}
	} else {
		scanRoot := pack.LocalDir
		sub := pack.Subdir
		if sub == "" && manifest != nil && manifest.IconsDir != "" {
			sub = manifest.IconsDir
		}
		if sub != "" {
			candidate := filepath.Join(pack.LocalDir, filepath.FromSlash(sub))
			if st, err := os.Stat(candidate); err == nil && st.IsDir() {
				scanRoot = candidate
			}
		}
		_ = filepath.WalkDir(scanRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				base := d.Name()
				if base == ".git" || base == "node_modules" {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".svg") {
				return nil
			}
			raw, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if _, err := ui.ParseSVG(raw); err != nil {
				return nil
			}
			rel, _ := filepath.Rel(scanRoot, path)
			relSlash := filepath.ToSlash(rel)
			cat := "general"
			if dir := filepath.Dir(rel); dir != "." {
				cat = strings.Split(filepath.ToSlash(dir), "/")[0]
			}
			base := strings.TrimSuffix(d.Name(), filepath.Ext(d.Name()))
			name := sanitizeIconName(base)
			if name == "" {
				return nil
			}
			if seen[name] && cat != "general" {
				name = sanitizeIconName(cat + "-" + base)
			}
			if seen[name] {
				return nil
			}
			seen[name] = true
			pack.Icons = append(pack.Icons, IconEntry{
				PackID:   pack.ID,
				Name:     name,
				Title:    humanizeName(base),
				Category: cat,
				Tags:     strings.Split(strings.ReplaceAll(strings.ToLower(name), "-", " "), " "),
				RelPath:  relSlash,
			})
			loadedBytes[pack.ID+"/"+name] = raw
			return nil
		})
	}

	slices.SortFunc(pack.Icons, func(a, b IconEntry) int {
		return strings.Compare(a.Name, b.Name)
	})
	for k, v := range loadedBytes {
		r.svgBytes[k] = v
		delete(r.parsed, k)
	}
	return nil
}

func matchesIconQuery(ic IconEntry, q string) bool {
	if strings.Contains(strings.ToLower(ic.Name), q) ||
		strings.Contains(strings.ToLower(ic.Title), q) ||
		strings.Contains(strings.ToLower(ic.Category), q) ||
		strings.Contains(strings.ToLower(ic.PackID), q) {
		return true
	}
	for _, tag := range ic.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	return false
}

func clonePack(p *IconPack) *IconPack {
	if p == nil {
		return nil
	}
	cp := *p
	cp.Icons = slices.Clone(p.Icons)
	return &cp
}

func parseGitSourceURL(raw string) (gitURL, ref, subdir string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", ""
	}
	// Support shorthand "https://github.com/owner/repo#ref:subdir" or "#subdir".
	if idx := strings.IndexByte(raw, '#'); idx > 0 {
		frag := raw[idx+1:]
		raw = raw[:idx]
		if r, s, ok := strings.Cut(frag, ":"); ok {
			ref, subdir = r, s
		} else if strings.Contains(frag, "/") {
			subdir = frag
		} else {
			ref = frag
		}
	}
	return raw, ref, subdir
}

func inferPackIDFromURL(raw string) string {
	trimmed := strings.TrimSuffix(strings.TrimRight(raw, "/"), ".git")
	trimmed = strings.ReplaceAll(trimmed, "\\", "/")
	if idx := strings.LastIndexByte(trimmed, '/'); idx >= 0 {
		return trimmed[idx+1:]
	}
	return trimmed
}

func sanitizePackID(s string) string {
	return sanitizeSlug(s)
}

func sanitizeIconName(s string) string {
	return sanitizeSlug(s)
}

func sanitizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == ' ':
			if !prevDash && b.Len() > 0 {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func humanizeName(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' })
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	return strings.Join(parts, " ")
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

func builtinMeta(name string) (category string, tags []string) {
	switch name {
	case "server", "database", "cache", "queue", "gateway", "load-balancer", "firewall", "storage":
		return "infrastructure", []string{"infra", "backend", "system", name}
	case "cloud", "container", "pod", "service", "lambda":
		return "cloud", []string{"cloud", "k8s", "docker", "microservice", name}
	case "user", "users", "browser", "mobile":
		return "clients", []string{"client", "actor", "ui", name}
	case "bot", "brain", "pipeline", "git", "terminal", "webhook", "monitor", "lock":
		return "dev-ai", []string{"agent", "devops", "security", name}
	default:
		return "general", []string{name}
	}
}
