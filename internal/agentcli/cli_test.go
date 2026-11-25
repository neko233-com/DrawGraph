package agentcli_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/agentcli"
	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/ipc"
	"github.com/neko233-com/DrawGraph/internal/layout"
	"github.com/neko233-com/DrawGraph/internal/render"
	"github.com/neko233-com/DrawGraph/internal/update"
)

func runCLI(t *testing.T, home string, stdin string, args ...string) (int, string, string) {
	t.Helper()
	t.Setenv("DRAWGRAPH_HOME", home)
	var out, errBuf bytes.Buffer
	code := agentcli.Run(args, strings.NewReader(stdin), &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestAgentCLIWorkflowBatchDSLExportAndHabits(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	diag := filepath.Join(tmp, "system.drawgraph")

	// 1. Discover schema
	code, out, errStr := runCLI(t, home, "", "schema", "--json")
	if code != 0 {
		t.Fatalf("schema failed (%d): %s %s", code, out, errStr)
	}
	var env agentcli.Envelope
	if err := json.Unmarshal([]byte(out), &env); err != nil || !env.OK {
		t.Fatalf("invalid schema envelope: %v\n%s", err, out)
	}

	// 2. Persist user habits across projects
	code, out, errStr = runCLI(t, home, "", "habits", "set", "--direction", "TB", "--preset", "blueprint", "--routing", "curved", "--json")
	if code != 0 {
		t.Fatalf("habits set failed (%d): %s %s", code, out, errStr)
	}

	// 3. Draw whole architecture via DSL + auto-format (inheriting TB + blueprint + curved!)
	dsl := `
container vpc "Production VPC"
node gw "API Gateway" icon:builtin/gateway parent:vpc
node svc "Order Service" icon:builtin/server parent:vpc
node db "Orders DB" kind:cylinder icon:builtin/database parent:vpc
gw -> svc : "gRPC"
svc -> db : "SQL"
`
	code, out, errStr = runCLI(t, home, dsl, "dsl", diag, "--stdin", "--format", "--json")
	if code != 0 {
		t.Fatalf("dsl failed (%d): %s %s", code, out, errStr)
	}

	doc, err := graph.LoadFile(diag)
	if err != nil {
		t.Fatal(err)
	}
	if doc.StylePreset != "blueprint" || doc.DefaultRouting != graph.RouteCurved {
		t.Fatalf("expected DSL doc to inherit blueprint/curved habits, got preset=%s routing=%s", doc.StylePreset, doc.DefaultRouting)
	}
	p := doc.ActivePage()
	if len(p.Nodes) != 4 || len(p.Edges) != 2 {
		t.Fatalf("expected 4 nodes & 2 edges, got %d & %d", len(p.Nodes), len(p.Edges))
	}
	// Verify TB layout ordering: gw.Y < svc.Y < db.Y
	if !(p.NodeByID("gw").Y < p.NodeByID("svc").Y && p.NodeByID("svc").Y < p.NodeByID("db").Y) {
		t.Fatalf("expected TB layering gw < svc < db, got gw=%v svc=%v db=%v",
			p.NodeByID("gw").Y, p.NodeByID("svc").Y, p.NodeByID("db").Y)
	}

	// 4. Transactional batch dry-run vs real batch
	batchOps := `[{"op":"add_node","id":"cache","label":"Redis","icon":"builtin/cache","parentId":"vpc"},{"op":"connect","from":"svc","to":"cache","label":"GET"}]`
	code, _, errStr = runCLI(t, home, batchOps, "batch", diag, "--stdin", "--dry-run", "--json")
	if code != 0 {
		t.Fatalf("batch dry-run failed: %s", errStr)
	}
	docAfterDry, _ := graph.LoadFile(diag)
	if len(docAfterDry.ActivePage().Nodes) != 4 {
		t.Fatalf("dry-run mutated file")
	}

	code, _, errStr = runCLI(t, home, batchOps, "batch", diag, "--stdin", "--format", "--json")
	if code != 0 {
		t.Fatalf("batch failed: %s", errStr)
	}
	docAfterBatch, _ := graph.LoadFile(diag)
	if len(docAfterBatch.ActivePage().Nodes) != 5 || len(docAfterBatch.ActivePage().Edges) != 3 {
		t.Fatalf("batch did not persist changes")
	}

	// 5. Validate diagram
	code, out, _ = runCLI(t, home, "", "validate", diag, "--json")
	if code != 0 {
		t.Fatalf("expected valid diagram, got code=%d out=%s", code, out)
	}

	// 6. Export SVG, PNG, and draw.io XML, then re-import draw.io XML!
	svgPath := filepath.Join(tmp, "out.svg")
	pngPath := filepath.Join(tmp, "out.png")
	dioPath := filepath.Join(tmp, "out.drawio")
	reimported := filepath.Join(tmp, "reimported.drawgraph")
	for _, target := range []string{svgPath, pngPath, dioPath} {
		if code, _, errStr := runCLI(t, home, "", "export", diag, "--to", target, "--json"); code != 0 {
			t.Fatalf("export %s failed: %s", target, errStr)
		}
		if st, err := os.Stat(target); err != nil || st.Size() < 100 {
			t.Fatalf("exported %s missing or empty: %v", target, err)
		}
	}
	if code, _, errStr := runCLI(t, home, "", "import", dioPath, "--to", reimported, "--json"); code != 0 {
		t.Fatalf("import drawio failed: %s", errStr)
	}
	reDoc, err := graph.LoadFile(reimported)
	if err != nil {
		t.Fatal(err)
	}
	if len(reDoc.ActivePage().Nodes) != 5 || len(reDoc.ActivePage().Edges) != 3 {
		t.Fatalf("reimported drawio counts mismatch: nodes=%d edges=%d", len(reDoc.ActivePage().Nodes), len(reDoc.ActivePage().Edges))
	}
}

func TestCLIGitIconInstallAndLiveIPC(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found")
	}
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")

	// Create a mock Git icon pack repository
	repo := filepath.Join(tmp, "team-icons")
	_ = os.MkdirAll(repo, 0o755)
	_ = os.WriteFile(filepath.Join(repo, "kafka.svg"), []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><circle cx="12" cy="12" r="8"/></svg>`), 0o644)
	gitRun := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, string(out))
		}
	}
	gitRun("init", "-b", "main")
	gitRun("config", "user.name", "Test")
	gitRun("config", "user.email", "test@example.com")
	gitRun("add", ".")
	gitRun("commit", "-m", "init")

	code, out, errStr := runCLI(t, home, "", "icon", "install", repo, "--id", "team", "--json")
	if code != 0 {
		t.Fatalf("icon install failed: %s %s", out, errStr)
	}
	code, out, _ = runCLI(t, home, "", "icon", "search", "kafka", "--json")
	if code != 0 || !strings.Contains(out, "team") {
		t.Fatalf("expected installed kafka icon in search output: %s", out)
	}

	// Start Live IPC server and push via `--live`
	t.Setenv("DRAWGRAPH_HOME", home)
	liveDoc := graph.NewDocument("Live Canvas")
	var replaced *graph.Document
	srv, err := ipc.Start("", &fakeIPCHandler{
		get: func() *graph.Document { return liveDoc },
		put: func(d *graph.Document) { replaced = d },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	code, out, errStr = runCLI(t, home, "", "live", "status", "--json")
	if code != 0 || !strings.Contains(out, `"connected": true`) {
		t.Fatalf("live status failed: %s %s", out, errStr)
	}

	f := filepath.Join(tmp, "live.drawgraph")
	code, out, errStr = runCLI(t, home, "node k \"Kafka\" icon:team/kafka\n", "dsl", f, "--stdin", "--live", "--json")
	if code != 0 {
		t.Fatalf("dsl --live failed: %s %s", out, errStr)
	}
	if replaced == nil || replaced.ActivePage().NodeByID("k") == nil {
		t.Fatalf("expected live IPC handler to receive updated document")
	}
}

type fakeIPCHandler struct {
	get func() *graph.Document
	put func(*graph.Document)
}

func (f *fakeIPCHandler) GetDocument() *graph.Document { return f.get() }
func (f *fakeIPCHandler) ReplaceDocument(d *graph.Document, _ string) error {
	f.put(d)
	return nil
}
func (f *fakeIPCHandler) ReloadIcons() error { return nil }

func TestCLIMediaCommandsAndDSLMedia(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")
	diag := filepath.Join(tmp, "media.drawgraph")

	imgPath := filepath.Join(tmp, "1080p.png")
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1920, 1080)))
	_ = os.WriteFile(imgPath, pngBuf.Bytes(), 0o644)

	vidPath := filepath.Join(tmp, "walkthrough.mp4")
	_ = os.WriteFile(vidPath, []byte("ftypmp42"), 0o644)

	code, out, errStr := runCLI(t, home, "", "add-image", diag, "--id", "hero", "--src", imgPath, "--json")
	if code != 0 {
		t.Fatalf("add-image failed: %s %s", out, errStr)
	}
	code, out, errStr = runCLI(t, home, "", "add-video", diag, "--id", "clip", "--src", vidPath, "--x", "700", "--json")
	if code != 0 {
		t.Fatalf("add-video failed: %s %s", out, errStr)
	}
	code, out, errStr = runCLI(t, home, "hero -> clip : \"Demo\"\n", "dsl", diag, "--stdin", "--format", "--json")
	if code != 0 {
		t.Fatalf("dsl format with media failed: %s %s", out, errStr)
	}

	doc, err := graph.LoadFile(diag)
	if err != nil {
		t.Fatal(err)
	}
	hero := doc.ActivePage().NodeByID("hero")
	clip := doc.ActivePage().NodeByID("clip")
	if hero == nil || hero.W != 480 || hero.H != 270 || doc.Assets[hero.Media.AssetID].NaturalW != 1920 {
		t.Fatalf("hero image sizing/resolution mismatch: %+v", hero)
	}
	if clip == nil || clip.Kind != graph.KindVideo {
		t.Fatalf("clip video node missing: %+v", clip)
	}
}

func TestCLIUpdateAndExtensionRegistries(t *testing.T) {
	tmp := t.TempDir()
	home := filepath.Join(tmp, "home")

	// Register extensible shape, layout strategy, and export driver.
	render.RegisterShape("custom-badge", func(ctx render.ShapePaintContext) {
		ctx.Painter.Fill(ctx.Box, ctx.Fill, ctx.Radius)
	})
	layout.RegisterStrategy("custom-grid", func(_ *graph.Document, p *graph.Page, _ layout.Options) layout.Report {
		return layout.Report{Direction: "GRID", Layers: len(p.Nodes)}
	})
	export.RegisterDriver(".txt", func(doc *graph.Document, _ string, _ *icons.Registry, _ bool, _ float64) ([]byte, error) {
		return []byte(doc.Title), nil
	})
	code, out, errStr := runCLI(t, home, "", "schema", "--json")
	if code != 0 || !strings.Contains(out, "custom-badge") || !strings.Contains(out, "custom-grid") || !strings.Contains(out, ".txt") {
		t.Fatalf("schema missing registered extensions (%d): %s %s", code, out, errStr)
	}

	// Update config persistence
	code, out, errStr = runCLI(t, home, "", "update", "config", "--auto-check", "true", "--auto-download", "true", "--channel", "prerelease", "--interval-hours", "12", "--json")
	if code != 0 || !strings.Contains(out, `"autoDownloadUpdates": true`) || !strings.Contains(out, `"updateChannel": "prerelease"`) {
		t.Fatalf("update config failed (%d): %s %s", code, out, errStr)
	}

	// Mock GitHub release server for CLI update check & download
	payload := []byte("msi-bytes-for-cli")
	sum := sha256.Sum256(payload)
	var srvURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/neko233-com/DrawGraph/releases/latest":
			fmt.Fprintf(w, `{"tag_name":"v9.9.9","assets":[
				{"name":"DrawGraph-windows-amd64.msi","browser_download_url":"%s/a.msi"},
				{"name":"DrawGraph-windows-amd64.msi.sha256","browser_download_url":"%s/a.sha"},
				{"name":"DrawGraph-darwin-arm64.dmg","browser_download_url":"%s/a.dmg"},
				{"name":"DrawGraph-darwin-arm64.dmg.sha256","browser_download_url":"%s/a.sha"}
			]}`, srvURL, srvURL, srvURL, srvURL)
		case "/a.msi", "/a.dmg":
			_, _ = w.Write(payload)
		case "/a.sha":
			fmt.Fprintf(w, "%s  asset\n", hex.EncodeToString(sum[:]))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	srvURL = ts.URL

	oldBase := update.APIBase
	update.APIBase = ts.URL
	defer func() { update.APIBase = oldBase }()

	code, out, errStr = runCLI(t, home, "", "update", "check", "--channel", "stable", "--current", "0.1.0", "--json")
	if code != 0 || !strings.Contains(out, `"version": "9.9.9"`) {
		t.Fatalf("update check failed (%d): %s %s", code, out, errStr)
	}
	dlDir := filepath.Join(tmp, "dl")
	code, out, errStr = runCLI(t, home, "", "update", "download", "--channel", "stable", "--current", "0.1.0", "--dir", dlDir, "--json")
	if code != 0 || !strings.Contains(out, "9.9.9") {
		t.Fatalf("update download failed (%d): %s %s", code, out, errStr)
	}
}
