package icons_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/icons"
)

func TestBuiltinIconsAllParseAndSearch(t *testing.T) {
	reg, err := icons.OpenRegistry(filepath.Join(t.TempDir(), "icons"))
	if err != nil {
		t.Fatal(err)
	}
	packs := reg.Packs()
	if len(packs) == 0 || packs[0].ID != "builtin" {
		t.Fatalf("expected builtin pack first, got %+v", packs)
	}
	if len(packs[0].Icons) < 20 {
		t.Fatalf("expected >= 20 builtin icons, got %d", len(packs[0].Icons))
	}
	for _, ic := range packs[0].Icons {
		if _, ok := reg.ParsedSVG(ic.Ref()); !ok {
			t.Fatalf("builtin icon %s failed ui.ParseSVG", ic.Ref())
		}
	}
	hits := reg.Search("database", "", "")
	if len(hits) == 0 || hits[0].Name != "database" {
		t.Fatalf("expected database hit, got %+v", hits)
	}
}

func TestInstallAndSyncFromRealGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}
	tmp := t.TempDir()
	remoteDir := filepath.Join(tmp, "remote-icons")
	if err := os.MkdirAll(filepath.Join(remoteDir, "cloud"), 0o755); err != nil {
		t.Fatal(err)
	}
	svgA := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="9"/></svg>`
	if err := os.WriteFile(filepath.Join(remoteDir, "cloud", "edge-pop.svg"), []byte(svgA), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = remoteDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	run("init", "-b", "main")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")
	run("add", ".")
	run("commit", "-m", "init icons")

	storeDir := filepath.Join(tmp, "drawgraph-icons")
	reg, err := icons.OpenRegistry(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	pack, err := reg.InstallGit(context.Background(), icons.GitInstallOptions{
		URL: remoteDir,
		ID:  "acme-cloud",
	})
	if err != nil {
		t.Fatal(err)
	}
	if pack.ID != "acme-cloud" || len(pack.Icons) != 1 || pack.Icons[0].Category != "cloud" {
		t.Fatalf("unexpected installed git pack: %+v", pack)
	}
	if _, ok := reg.ParsedSVG("acme-cloud/edge-pop"); !ok {
		t.Fatal("failed to parse installed git SVG icon")
	}

	// Commit a new icon upstream and SyncPack!
	svgB := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><rect x="4" y="4" width="16" height="16"/></svg>`
	if err := os.WriteFile(filepath.Join(remoteDir, "cloud", "kv-store.svg"), []byte(svgB), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", ".")
	run("commit", "-m", "add kv-store")

	synced, err := reg.SyncPack(context.Background(), "acme-cloud")
	if err != nil {
		t.Fatal(err)
	}
	if len(synced.Icons) != 2 {
		t.Fatalf("expected 2 icons after SyncPack, got %d", len(synced.Icons))
	}

	// Re-open Registry cleanly from disk to verify persistence!
	reloaded, err := icons.OpenRegistry(storeDir)
	if err != nil {
		t.Fatal(err)
	}
	if hits := reloaded.Search("kv-store", "acme-cloud", ""); len(hits) != 1 {
		t.Fatalf("expected persisted kv-store icon after reopen, got %+v", hits)
	}
	if err := reloaded.RemovePack("acme-cloud"); err != nil {
		t.Fatal(err)
	}
	if hits := reloaded.Search("kv-store", "", ""); len(hits) != 0 {
		t.Fatal("expected icon to disappear after RemovePack")
	}
}
