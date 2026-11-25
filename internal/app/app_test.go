package app_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/app"
	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/icons"
)

type testHost struct {
	mu      sync.Mutex
	changes int
	last    *graph.Document
}

func (h *testHost) Changed(d *graph.Document) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.changes++
	h.last = d.Clone()
}

func (h *testHost) Window() *mygo.Window { return nil }

func (h *testHost) Update(fn func()) {
	fn()
}

func newTestWorkbench(t *testing.T) (*app.App, *ui.Tester, *testHost) {
	t.Helper()
	tmp := t.TempDir()
	hb, err := habits.OpenStore(filepath.Join(tmp, "habits.json"))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := icons.OpenRegistry(filepath.Join(tmp, "icons"))
	if err != nil {
		t.Fatal(err)
	}
	doc := graph.NewDocument("UI Test")
	hb.ApplyToDocument(doc)
	host := &testHost{}
	a := app.New(doc, reg, hb, host)
	tt := ui.NewTester(a.View, 1360, 860)
	tt.Frame()
	return a, tt, host
}

func TestWorkbenchToolbarSidebarDrawersAndShortcuts(t *testing.T) {
	a, tt, _ := newTestWorkbench(t)

	// 1. Click a remembered favorite icon or scroll/click an icon in the sidebar!
	a.IconSearch = "server"
	tt.Frame()
	if err := tt.Click("Icon builtin/server"); err != nil {
		t.Fatalf("click sidebar icon: %v", err)
	}
	a.IconSearch = ""
	tt.Frame()
	if len(a.Page().Nodes) != 1 {
		t.Fatalf("expected 1 node after sidebar icon click, got %d", len(a.Page().Nodes))
	}
	// Verify persistent habit memory recorded the icon use!
	if a.Habits.Snapshot().IconUsage["builtin/server"] < 1 {
		t.Fatalf("expected IconUsage to record builtin/server")
	}

	// 2. Use keyboard tool 'R' + canvas click to place a second shape!
	tt.Key(0, ui.KeyR)
	tt.Frame()
	if a.Tool != app.ToolShape {
		t.Fatalf("expected ToolShape after pressing R, got %s", a.Tool)
	}
	cv, ok := tt.Find("Canvas")
	if !ok {
		t.Fatal("Canvas element not found")
	}
	tt.ClickAt(cv.X+520, cv.Y+280)
	tt.Frame()
	if len(a.Page().Nodes) != 2 {
		t.Fatalf("expected 2 nodes after canvas shape placement, got %d", len(a.Page().Nodes))
	}
	// Commit inline edit opened after placing shape
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// 3. Connect node 0 -> node 1 using Connect tool ('C') and pointer drag!
	n0 := a.Page().Nodes[0]
	n1 := a.Page().Nodes[1]
	p0x, p0y := a.Cam.WorldToScreen(cv, geo.V(n0.X+n0.W/2, n0.Y+n0.H/2))
	p1x, p1y := a.Cam.WorldToScreen(cv, geo.V(n1.X+n1.W/2, n1.Y+n1.H/2))
	tt.Key(0, ui.KeyC)
	tt.Frame()
	tt.Press(p0x, p0y)
	tt.Move((p0x+p1x)/2, (p0y+p1y)/2)
	tt.Release(p1x, p1y)
	tt.Frame()
	if len(a.Page().Edges) != 1 {
		t.Fatalf("expected 1 edge after Connect drag, got %d", len(a.Page().Edges))
	}

	// 4. Click "Format" primary button on toolbar -> arranges nodes cleanly!
	if err := tt.Click("Format"); err != nil {
		t.Fatalf("click Format button: %v", err)
	}
	tt.Frame()
	if !(a.Page().Nodes[0].X < a.Page().Nodes[1].X) {
		t.Fatalf("expected Format LR to place source left of target: n0=%v n1=%v",
			a.Page().Nodes[0].X, a.Page().Nodes[1].X)
	}

	// 5. Undo and Redo via keyboard shortcuts!
	prevX := a.Page().Nodes[1].X
	tt.Key(ui.Cmd, ui.KeyZ)
	tt.Frame()
	if a.Page().Nodes[1].X == prevX {
		t.Fatalf("expected Undo to revert Format position")
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	tt.Frame()
	if a.Page().Nodes[1].X != prevX {
		t.Fatalf("expected Redo to restore Format position")
	}

	// 6. Duplicate selection and delete!
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Frame()
	tt.Key(ui.Cmd, ui.KeyD)
	tt.Frame()
	if len(a.Page().Nodes) != 4 {
		t.Fatalf("expected 4 nodes after Cmd+A + Cmd+D, got %d", len(a.Page().Nodes))
	}
	tt.Key(0, ui.KeyDelete)
	tt.Frame()
	if len(a.Page().Nodes) != 2 {
		t.Fatalf("expected 2 nodes after deleting duplicates, got %d", len(a.Page().Nodes))
	}

	// 7. Open Icon Plugins drawer & Habits drawer!
	if err := tt.Click("Icon Plugins"); err != nil {
		t.Fatalf("click Icon Plugins: %v", err)
	}
	tt.Frame()
	if !a.PluginsOpen || !tt.HasText("Install from Git URL") {
		t.Fatalf("Icon Plugins drawer did not open cleanly")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	if err := tt.Click("Habits"); err != nil {
		t.Fatalf("click Habits: %v", err)
	}
	tt.Frame()
	if !a.HabitsOpen || !tt.HasText("Preferred Layout Direction") {
		t.Fatalf("Habits drawer did not open cleanly")
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// 8. Verify GPU-rendered offscreen frame image is non-empty!
	img := tt.Image()
	if img == nil || img.Bounds().Dx() < 800 || img.Bounds().Dy() < 600 {
		t.Fatalf("unexpected rendered UI image: %v", img)
	}
}

func TestLiveIPCDocumentReplacement(t *testing.T) {
	a, tt, host := newTestWorkbench(t)
	incoming := graph.NewDocument("Agent Built")
	_ = incoming.ActivePage().AddNode(&graph.Node{ID: "bot", Label: "AI Agent", X: 100, Y: 100, W: 160, H: 72})
	if err := a.ReplaceDocument(incoming, "agent batch"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.Doc().Title != "Agent Built" || a.Page().NodeByID("bot") == nil || host.changes == 0 {
		t.Fatalf("live document replacement did not apply cleanly")
	}
}

func TestCanvasDragResizeInlineEditAndMultiPage(t *testing.T) {
	a, tt, _ := newTestWorkbench(t)
	n := a.AddShapeAt(graph.KindRounded, "Start", "", 100, 100, true)
	tt.Frame()

	cv, ok := tt.Find("Canvas")
	if !ok {
		t.Fatal("Canvas element not found")
	}
	// Drag node by +60, +40 (snapped to 20px grid)
	sx, sy := a.Cam.WorldToScreen(cv, geo.V(n.X+n.W/2, n.Y+n.H/2))
	tt.Press(sx, sy)
	tt.Move(sx+60, sy+40)
	tt.Release(sx+60, sy+40)
	tt.Frame()
	if n.X != 160 || n.Y != 140 {
		t.Fatalf("expected dragged node at (160,140), got (%v,%v)", n.X, n.Y)
	}

	// Open inline text editor via F2 and commit with Enter
	a.SelectedNodes = map[string]bool{n.ID: true}
	tt.Frame()
	tt.Key(0, ui.KeyF2)
	tt.Frame()
	if a.EditingNodeID != n.ID {
		t.Fatalf("expected inline editor open for %s", n.ID)
	}
	a.EditingText = "Updated Title"
	tt.Key(0, ui.KeyEnter)
	tt.Frame()
	if n.Label != "Updated Title" || a.EditingNodeID != "" {
		t.Fatalf("inline edit commit failed: label=%q editing=%q", n.Label, a.EditingNodeID)
	}

	// Add a second page via the bottom PageBar button
	if err := tt.Click("Add Page"); err != nil {
		t.Fatalf("click Add Page: %v", err)
	}
	tt.Frame()
	if len(a.Doc().Pages) != 2 {
		t.Fatalf("expected 2 pages after Add Page, got %d", len(a.Doc().Pages))
	}
}

func TestInfiniteCanvasDropImagesVideosAndNativeResize(t *testing.T) {
	a, tt, _ := newTestWorkbench(t)
	tmp := t.TempDir()

	// Write a 1920x1080 HD image and an .mp4 video file on disk.
	imgPath := filepath.Join(tmp, "design-hd.png")
	srcImg := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y += 20 {
		for x := 0; x < 1920; x += 20 {
			srcImg.Set(x, y, color.RGBA{R: 59, G: 130, B: 246, A: 255})
		}
	}
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, srcImg)
	_ = os.WriteFile(imgPath, pngBuf.Bytes(), 0o644)

	vidPath := filepath.Join(tmp, "preview.mp4")
	_ = os.WriteFile(vidPath, []byte("ftypisom"), 0o644)

	// Pan deep into negative infinite canvas coordinates and zoom way out & way in!
	a.Cam.X = -4500
	a.Cam.Y = -2800
	a.Cam.Zoom = 0.05
	tt.Frame()
	if a.Cam.Zoom != 0.05 {
		t.Fatalf("expected infinite canvas zoom 0.05, got %v", a.Cam.Zoom)
	}
	tt.Key(ui.Cmd|ui.Shift, ui.KeyH) // Reset Origin
	tt.Frame()
	if a.Cam.Zoom != 1 || a.Cam.X != 24 || a.Cam.Y != 24 {
		t.Fatalf("ResetOrigin failed: %+v", a.Cam)
	}

	cv, ok := tt.Find("Canvas")
	if !ok {
		t.Fatal("Canvas element not found")
	}

	// Drag-and-drop both HD image and video directly onto Canvas!
	tt.DropFiles(cv.X+300, cv.Y+240, imgPath, vidPath)
	tt.Frame()
	if len(a.Page().Nodes) != 2 {
		t.Fatalf("expected 2 media nodes after DropFiles, got %d", len(a.Page().Nodes))
	}
	imgNode := a.Page().Nodes[0]
	vidNode := a.Page().Nodes[1]
	if imgNode.Kind != graph.KindImage || imgNode.W != 480 || imgNode.H != 270 {
		t.Fatalf("dropped HD image node mismatch: %+v", imgNode)
	}
	if a.Doc().Assets[imgNode.Media.AssetID].NaturalW != 1920 || a.Doc().Assets[imgNode.Media.AssetID].NaturalH != 1080 {
		t.Fatalf("lost full 1920x1080 asset metadata")
	}
	if vidNode.Kind != graph.KindVideo {
		t.Fatalf("expected dropped video node, got %s", vidNode.Kind)
	}

	// Select only image node -> click Inspector "1:1 (1920x1080)" -> switches to native HD pixels!
	a.SelectedNodes = map[string]bool{imgNode.ID: true}
	tt.Frame()
	if err := tt.Click("1:1 (1920x1080)"); err != nil {
		t.Fatalf("click 1:1 native resize: %v", err)
	}
	tt.Frame()
	if imgNode.W != 1920 || imgNode.H != 1080 {
		t.Fatalf("expected 1:1 native 1920x1080, got %gx%g", imgNode.W, imgNode.H)
	}
	if err := tt.Click("HD Fit"); err != nil {
		t.Fatalf("click HD Fit: %v", err)
	}
	tt.Frame()
	if imgNode.W != 480 || imgNode.H != 270 {
		t.Fatalf("expected HD Fit 480x270, got %gx%g", imgNode.W, imgNode.H)
	}

	// Verify GPU image renders cleanly in offscreen UI tester.
	frame := tt.Image()
	if frame == nil || frame.Bounds().Dx() < 800 {
		t.Fatalf("unexpected rendered UI image: %v", frame)
	}
}
