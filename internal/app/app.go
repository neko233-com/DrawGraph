// Package app implements the native MyGo GPU-rendered DrawGraph desktop workbench.
package app

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/feedback"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/drawio"
	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/layout"
	"github.com/neko233-com/DrawGraph/internal/render"
)

// Host abstracts window lifecycle and file persistence so App runs identically
// in the real MyGo window and inside headless ui.Tester tests.
type Host interface {
	Changed(doc *graph.Document)
	Update(fn func())
	Window() *mygo.Window
}

// Tool identifies the active pointer interaction mode on the canvas.
type Tool string

const (
	ToolSelect  Tool = "select"
	ToolHand    Tool = "hand"
	ToolConnect Tool = "connect"
	ToolShape   Tool = "shape"
)

type dragMode int

const (
	dragNone dragMode = iota
	dragPan
	dragMoveNodes
	dragResizeNode
	dragMarquee
	dragConnect
)

// App holds the full interactive state of a DrawGraph workbench window.
type App struct {
	Host   Host
	Hist   *graph.History
	Icons  *icons.Registry
	Habits *habits.Store

	FilePath string
	Cam      render.Camera
	Tool     Tool
	Shape    graph.NodeKind

	SelectedNodes map[string]bool
	SelectedEdges map[string]bool
	HoveredNodeID string

	CanvasHandle ui.Handle
	CanvasRect   ui.Rect
	Initialized  bool

	// Theme & view toggles
	ThemeMode     string // "auto", "light", "dark"
	DarkActive    bool
	ShowGrid      bool
	SidebarOpen   bool
	InspectorOpen bool
	MinimapOpen   bool

	// Drawers
	PluginsOpen bool
	HabitsOpen  bool

	// Inline text editing
	EditingNodeID string
	EditingText   string
	EditFocusSeq  int

	// Search & Icon Plugin Form
	IconSearch   string
	ActivePack   string
	GitURL       string
	GitRef       string
	GitSubdir    string
	GitPackID    string
	PluginBusy   bool
	PluginStatus string

	// Pointer drag tracking
	drag           dragMode
	dragStartWorld geo.Vec
	dragStartDip   [2]float32
	dragStartCam   render.Camera
	dragNodeOrig   map[string]geo.Box
	dragCorner     int
	connectSrcID   string
	connectEndPt   geo.Vec
	marqueeBox     *geo.Box
	spaceHeld      bool

	// Toast bridge
	toastSeq   int
	toastShown int
	toastType  feedback.ToastType
	toastTitle string
	toastDesc  string

	// Temporary fields bound to inspector widgets
	newPageName string
}

// New creates a DrawGraph App around doc, icon registry, and persistent habit store.
func New(doc *graph.Document, reg *icons.Registry, hb *habits.Store, host Host) *App {
	if hb == nil {
		hb, _ = habits.OpenStore("")
	}
	if reg == nil {
		reg, _ = icons.OpenRegistry("")
	}
	p := hb.Snapshot()
	if doc == nil {
		doc = graph.NewDocument("Untitled Diagram")
		hb.ApplyToDocument(doc)
	}
	doc.Normalize()

	return &App{
		Host:          host,
		Hist:          graph.NewHistory(doc),
		Icons:         reg,
		Habits:        hb,
		Cam:           render.Camera{X: 24, Y: 24, Zoom: 1},
		Tool:          ToolSelect,
		Shape:         p.DefaultNodeKind,
		SelectedNodes: map[string]bool{},
		SelectedEdges: map[string]bool{},
		ThemeMode:     p.ThemeMode,
		ShowGrid:      p.ShowGrid,
		SidebarOpen:   p.SidebarOpen,
		InspectorOpen: p.InspectorOpen,
		MinimapOpen:   p.MinimapOpen,
	}
}

// Doc returns the current mutable document snapshot.
func (a *App) Doc() *graph.Document { return a.Hist.Doc }

// Page returns the active page of the document.
func (a *App) Page() *graph.Page { return a.Hist.Doc.ActivePage() }

// View builds the complete Lobe UI + GPU Canvas workbench.
func (a *App) View(c *ui.Context) {
	theme.Apply(c)
	tk := theme.TokensOf(c)
	a.applyTheme(c)

	if b := a.CanvasHandle.Bounds(c); b.W > 0 {
		a.CanvasRect = b
		if !a.Initialized {
			a.Initialized = true
			if len(a.Page().Nodes) > 0 {
				a.ZoomToFit()
			}
		}
	}

	if a.toastSeq != a.toastShown {
		a.toastShown = a.toastSeq
		feedback.ShowToast(c, feedback.ToastProps{
			Type:        a.toastType,
			Title:       a.toastTitle,
			Description: a.toastDesc,
		})
	}

	a.handleShortcuts(c)

	ui.Column(c).Fill().Background(tk.BgLayout).Children(func() {
		a.toolbar(c, tk)
		ui.Row(c).Grow(1).Basis(0).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			if a.SidebarOpen {
				a.sidebar(c, tk)
			}
			ui.Column(c).Grow(1).Basis(0).MinHeight(0).Children(func() {
				a.canvasArea(c, tk)
				a.pageBar(c, tk)
			})
			if a.InspectorOpen {
				a.inspector(c, tk)
			}
		})
		a.statusBar(c, tk)
		a.pluginsDrawer(c, tk)
		a.habitsDrawer(c, tk)
		feedback.ToastHost(c, feedback.ToastHostProps{Placement: feedback.ToastBottomLeft})
	})
}

func (a *App) applyTheme(c *ui.Context) {
	switch a.ThemeMode {
	case "light":
		c.SetTheme(ui.LightTheme())
		a.DarkActive = false
	case "dark":
		c.SetTheme(ui.DarkTheme())
		a.DarkActive = true
	default:
		a.DarkActive = c.Theme().Dark
	}
}

func (a *App) notifyChanged() {
	if a.Host != nil {
		a.Host.Changed(a.Doc())
	}
}

func (a *App) notifyToast(kind feedback.ToastType, title, desc string) {
	a.toastType = kind
	a.toastTitle = title
	a.toastDesc = desc
	a.toastSeq++
}

// GetDocument implements ipc.Handler so CLI can inspect live open diagrams.
func (a *App) GetDocument() *graph.Document {
	return a.Doc().Clone()
}

// ReplaceDocument implements ipc.Handler so CLI --live commands update the open window immediately.
func (a *App) ReplaceDocument(doc *graph.Document, reason string) error {
	if doc == nil {
		return fmt.Errorf("nil document")
	}
	done := make(chan struct{})
	a.Host.Update(func() {
		a.Hist.Snapshot()
		a.Hist.Doc = doc.Clone()
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
		a.notifyChanged()
		if reason != "" {
			a.notifyToast(feedback.ToastInfo, "Live CLI Update", reason)
		}
		close(done)
	})
	<-done
	return nil
}

// ReloadIcons implements ipc.Handler so newly installed icon packs show up immediately.
func (a *App) ReloadIcons() error {
	reg, err := icons.OpenRegistry(a.Icons.RootDir())
	if err != nil {
		return err
	}
	done := make(chan struct{})
	a.Host.Update(func() {
		a.Icons = reg
		a.notifyToast(feedback.ToastSuccess, "Icons Reloaded", fmt.Sprintf("%d packs ready", len(reg.Packs())))
		close(done)
	})
	<-done
	return nil
}

// AddShapeAt adds a new node of kind at world coordinate (or automatic tidy spot when centered=true).
func (a *App) AddShapeAt(kind graph.NodeKind, label string, iconRef string, wx, wy float64, snap bool) *graph.Node {
	a.Hist.Snapshot()
	if label == "" {
		label = strings.ToUpper(string(kind[:1])) + string(kind[1:])
	}
	if snap && a.Doc().SnapToGrid {
		wx = geo.Snap(wx, a.Doc().GridSize)
		wy = geo.Snap(wy, a.Doc().GridSize)
	}
	n := a.Habits.NewNodeFromHabits(kind, label, wx, wy)
	if iconRef != "" {
		n.Icon = graph.ParseIconRef(iconRef)
		if kind == "" || kind == graph.KindIcon {
			n.Kind = graph.KindRounded
			n.W = 168
			n.H = 72
		}
		a.Habits.RecordIconUse(iconRef)
	}
	_ = a.Page().AddNode(n)
	a.SelectedNodes = map[string]bool{n.ID: true}
	a.SelectedEdges = map[string]bool{}
	a.Habits.RecordNodeHabit(n)
	a.notifyChanged()
	return n
}

// QuickInsertShape places a new shape cleanly inside the visible viewport without overlapping existing ones.
func (a *App) QuickInsertShape(kind graph.NodeKind, label, iconRef string) *graph.Node {
	p := a.Page()
	wx, wy := 120.0, 120.0
	if a.CanvasRect.W > 0 {
		center := a.Cam.ScreenToWorld(a.CanvasRect, a.CanvasRect.X+a.CanvasRect.W/2, a.CanvasRect.Y+a.CanvasRect.H/2)
		wx = center.X - 80
		wy = center.Y - 36
	}
	// Offset gracefully if a node is already near that spot.
	for step := 0; step < 12; step++ {
		cand := geo.B(wx, wy, 160, 72)
		collision := false
		for _, ex := range p.Nodes {
			if ex.Kind != graph.KindContainer && ex.Kind != graph.KindSwimlane && ex.Box().Overlaps(cand) {
				collision = true
				break
			}
		}
		if !collision {
			break
		}
		wx += 40
		wy += 40
	}
	return a.AddShapeAt(kind, label, iconRef, wx, wy, true)
}

// FormatOneClick applies layered DAG layout, orthogonal routing, container wrapping, and user palette habits.
func (a *App) FormatOneClick(dirOverride string) layout.Report {
	a.Hist.Snapshot()
	opts := layout.OptionsFromHabits(a.Habits.Snapshot(), a.DarkActive)
	if dirOverride == "LR" || dirOverride == "TB" {
		opts.Direction = dirOverride
	}
	rep := layout.FormatPage(a.Doc(), a.Page(), opts)
	a.Habits.RecordFormatHabit(opts.Direction, opts.Routing, opts.Preset, opts.GapX, opts.GapY, opts.GridSize)
	a.ZoomToFit()
	a.notifyChanged()
	a.notifyToast(feedback.ToastSuccess, "Formatted Diagram",
		fmt.Sprintf("%s layout · %d layers · %d nodes arranged", rep.Direction, rep.Layers, len(a.Page().Nodes)))
	return rep
}

// Undo reverts to the previous document snapshot.
func (a *App) Undo() {
	if a.Hist.Undo() {
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
		a.notifyChanged()
	}
}

// Redo re-applies an undone snapshot.
func (a *App) Redo() {
	if a.Hist.Redo() {
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
		a.notifyChanged()
	}
}

// DeleteSelection removes all selected nodes and edges.
func (a *App) DeleteSelection() {
	if len(a.SelectedNodes) == 0 && len(a.SelectedEdges) == 0 {
		return
	}
	a.Hist.Snapshot()
	p := a.Page()
	for id := range a.SelectedEdges {
		p.RemoveEdge(id)
	}
	for id := range a.SelectedNodes {
		p.RemoveNode(id)
	}
	a.SelectedNodes = map[string]bool{}
	a.SelectedEdges = map[string]bool{}
	a.notifyChanged()
}

// DuplicateSelection clones all selected nodes with a clean offset.
func (a *App) DuplicateSelection() {
	if len(a.SelectedNodes) == 0 {
		return
	}
	a.Hist.Snapshot()
	p := a.Page()
	nextSel := map[string]bool{}
	for id := range a.SelectedNodes {
		orig := p.NodeByID(id)
		if orig == nil {
			continue
		}
		cp := *orig
		cp.ID = p.NextNodeID("node")
		cp.X += 40
		cp.Y += 40
		if orig.Icon != nil {
			ic := *orig.Icon
			cp.Icon = &ic
		}
		_ = p.AddNode(&cp)
		nextSel[cp.ID] = true
	}
	a.SelectedNodes = nextSel
	a.notifyChanged()
}

// AlignSelection aligns 2+ selected nodes.
func (a *App) AlignSelection(mode graph.AlignMode) {
	if len(a.SelectedNodes) < 2 {
		return
	}
	ids := make([]string, 0, len(a.SelectedNodes))
	for id := range a.SelectedNodes {
		ids = append(ids, id)
	}
	a.Hist.Snapshot()
	if graph.AlignNodes(a.Page(), ids, mode) {
		a.notifyChanged()
	}
}

// SelectAll selects every node on the active page.
func (a *App) SelectAll() {
	a.SelectedNodes = map[string]bool{}
	a.SelectedEdges = map[string]bool{}
	for _, n := range a.Page().Nodes {
		a.SelectedNodes[n.ID] = true
	}
}

// ZoomIn zooms in around the canvas center.
func (a *App) ZoomIn() { a.zoomBy(1.2) }

// ZoomOut zooms out around the canvas center.
func (a *App) ZoomOut() { a.zoomBy(1 / 1.2) }

// ResetZoom sets zoom back to 100%.
func (a *App) ResetZoom() { a.Cam.Zoom = 1 }

// ZoomToFit frames all nodes cleanly inside the visible canvas.
func (a *App) ZoomToFit() {
	box, ok := a.Page().Bounds()
	if !ok || a.CanvasRect.W <= 40 || a.CanvasRect.H <= 40 {
		a.Cam = render.Camera{X: 24, Y: 24, Zoom: 1}
		return
	}
	box = box.Expand(64)
	zx := float64(a.CanvasRect.W) / box.W
	zy := float64(a.CanvasRect.H) / box.H
	z := math.Min(zx, zy)
	z = math.Max(0.25, math.Min(1.5, z))
	a.Cam.Zoom = z
	a.Cam.X = (float64(a.CanvasRect.W)/z-box.W)/2 - box.X
	a.Cam.Y = (float64(a.CanvasRect.H)/z-box.H)/2 - box.Y
}

func (a *App) zoomBy(factor float64) {
	cx := float64(a.CanvasRect.W) / 2
	cy := float64(a.CanvasRect.H) / 2
	if cx <= 0 {
		a.Cam.Zoom = clampZoom(a.Cam.Zoom * factor)
		return
	}
	a.zoomAt(cx, cy, factor)
}

func (a *App) zoomAt(localX, localY, factor float64) {
	oldZ := a.Cam.Zoom
	newZ := clampZoom(oldZ * factor)
	if oldZ <= 0 {
		oldZ = 1
	}
	wx := localX/oldZ - a.Cam.X
	wy := localY/oldZ - a.Cam.Y
	a.Cam.Zoom = newZ
	a.Cam.X = localX/newZ - wx
	a.Cam.Y = localY/newZ - wy
}

func clampZoom(z float64) float64 {
	return math.Max(0.2, math.Min(4.0, z))
}

func (a *App) singleSelectedNode() *graph.Node {
	if len(a.SelectedNodes) != 1 {
		return nil
	}
	for id := range a.SelectedNodes {
		return a.Page().NodeByID(id)
	}
	return nil
}

func (a *App) singleSelectedEdge() *graph.Edge {
	if len(a.SelectedEdges) != 1 {
		return nil
	}
	for id := range a.SelectedEdges {
		return a.Page().EdgeByID(id)
	}
	return nil
}

// InstallGitIconPack installs a Git repository icon pack asynchronously (or synchronously in tests).
func (a *App) InstallGitIconPack() {
	urlStr := strings.TrimSpace(a.GitURL)
	if urlStr == "" || a.PluginBusy {
		return
	}
	a.PluginBusy = true
	a.PluginStatus = "Cloning " + urlStr + "..."
	ref := strings.TrimSpace(a.GitRef)
	subdir := strings.TrimSpace(a.GitSubdir)
	packID := strings.TrimSpace(a.GitPackID)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		pack, err := a.Icons.InstallGit(ctx, icons.GitInstallOptions{
			URL: urlStr, ID: packID, Ref: ref, Subdir: subdir,
		})
		a.Host.Update(func() {
			a.PluginBusy = false
			if err != nil {
				a.PluginStatus = "Install failed: " + err.Error()
				a.notifyToast(feedback.ToastError, "Icon Pack Failed", err.Error())
				return
			}
			a.GitURL = ""
			a.GitRef = ""
			a.GitSubdir = ""
			a.GitPackID = ""
			a.ActivePack = pack.ID
			a.PluginStatus = fmt.Sprintf("Installed %s (%d icons)", pack.ID, len(pack.Icons))
			a.notifyToast(feedback.ToastSuccess, "Icon Pack Installed", a.PluginStatus)
		})
	}()
}

// NewBlankDocument starts a fresh document seeded by the user's persistent habits.
func (a *App) NewBlankDocument() {
	doc := graph.NewDocument("Untitled Diagram")
	a.Habits.ApplyToDocument(doc)
	a.Hist = graph.NewHistory(doc)
	a.FilePath = ""
	a.SelectedNodes = map[string]bool{}
	a.SelectedEdges = map[string]bool{}
	a.Cam = render.Camera{X: 24, Y: 24, Zoom: 1}
	a.notifyChanged()
}

// OpenFromDialog opens a .drawgraph or .drawio file via the native OS picker.
func (a *App) OpenFromDialog() {
	win := a.Host.Window()
	go func() {
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent: win,
			Title:  "Open Diagram",
			Filters: []mygo.FileFilter{
				{Name: "Diagrams (*.drawgraph, *.drawio)", Extensions: []string{"drawgraph", "json", "drawio", "xml"}},
			},
		})
		if err != nil || len(paths) == 0 || paths[0] == "" {
			return
		}
		p := paths[0]
		var doc *graph.Document
		if strings.EqualFold(filepath.Ext(p), ".drawio") || strings.EqualFold(filepath.Ext(p), ".xml") {
			doc, err = drawio.ImportFile(p)
		} else {
			doc, err = graph.LoadFile(p)
		}
		if err != nil {
			a.Host.Update(func() { a.notifyToast(feedback.ToastError, "Open Failed", err.Error()) })
			return
		}
		a.Host.Update(func() {
			a.Hist = graph.NewHistory(doc)
			a.FilePath = p
			a.SelectedNodes = map[string]bool{}
			a.SelectedEdges = map[string]bool{}
			a.Habits.RecordRecentFile(p)
			a.ZoomToFit()
			a.notifyChanged()
			a.notifyToast(feedback.ToastSuccess, "Opened", filepath.Base(p))
		})
	}()
}

// SaveToCurrentOrDialog saves to FilePath or opens a native Save dialog.
func (a *App) SaveToCurrentOrDialog(saveAs bool) {
	if !saveAs && a.FilePath != "" {
		var err error
		if strings.EqualFold(filepath.Ext(a.FilePath), ".drawio") {
			err = drawio.ExportFile(a.FilePath, a.Doc())
		} else {
			err = graph.SaveAtomic(a.FilePath, a.Doc())
		}
		if err != nil {
			a.notifyToast(feedback.ToastError, "Save Failed", err.Error())
		} else {
			a.Habits.RecordRecentFile(a.FilePath)
			a.notifyToast(feedback.ToastSuccess, "Saved", filepath.Base(a.FilePath))
		}
		return
	}
	win := a.Host.Window()
	docSnap := a.Doc().Clone()
	go func() {
		defName := docSnap.Title + ".drawgraph"
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       "Save Diagram",
			DefaultPath: defName,
			Filters: []mygo.FileFilter{
				{Name: "DrawGraph (*.drawgraph)", Extensions: []string{"drawgraph"}},
				{Name: "draw.io XML (*.drawio)", Extensions: []string{"drawio"}},
			},
		})
		if err != nil || p == "" {
			return
		}
		if strings.EqualFold(filepath.Ext(p), ".drawio") {
			err = drawio.ExportFile(p, docSnap)
		} else {
			err = graph.SaveAtomic(p, docSnap)
		}
		a.Host.Update(func() {
			if err != nil {
				a.notifyToast(feedback.ToastError, "Save Failed", err.Error())
				return
			}
			a.FilePath = p
			a.Habits.RecordRecentFile(p)
			a.notifyToast(feedback.ToastSuccess, "Saved", filepath.Base(p))
		})
	}()
}

// ExportDialog exports the active page as SVG, PNG, or .drawio XML.
func (a *App) ExportDialog(ext string) {
	win := a.Host.Window()
	docSnap := a.Doc().Clone()
	dark := a.DarkActive
	pageID := a.Page().ID
	reg := a.Icons
	go func() {
		p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{
			Parent:      win,
			Title:       "Export " + strings.ToUpper(ext),
			DefaultPath: docSnap.Title + "." + ext,
			Filters:     []mygo.FileFilter{{Name: strings.ToUpper(ext), Extensions: []string{ext}}},
		})
		if err != nil || p == "" {
			return
		}
		if ext == "drawio" {
			err = drawio.ExportFile(p, docSnap)
		} else {
			err = export.ExportFile(p, docSnap, pageID, reg, dark, 2)
		}
		a.Host.Update(func() {
			if err != nil {
				a.notifyToast(feedback.ToastError, "Export Failed", err.Error())
				return
			}
			a.notifyToast(feedback.ToastSuccess, "Exported", filepath.Base(p))
		})
	}()
}

// ImportIconFolderDialog lets the user pick a local folder of SVGs to register as an icon pack.
func (a *App) ImportIconFolderDialog() {
	win := a.Host.Window()
	go func() {
		dirs, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:    win,
			Title:     "Select SVG Icon Folder",
			Directory: true,
		})
		if err != nil || len(dirs) == 0 || dirs[0] == "" {
			return
		}
		dir := dirs[0]
		pack, err := a.Icons.InstallLocal(dir, "", filepath.Base(dir), "")
		a.Host.Update(func() {
			if err != nil {
				a.notifyToast(feedback.ToastError, "Import Failed", err.Error())
				return
			}
			a.ActivePack = pack.ID
			a.notifyToast(feedback.ToastSuccess, "Icon Pack Added", fmt.Sprintf("%s (%d icons)", pack.ID, len(pack.Icons)))
		})
	}()
}

// StartWindowTitle returns a clean title showing document title and path.
func (a *App) WindowTitle() string {
	t := a.Doc().Title
	if t == "" {
		t = "DrawGraph"
	} else {
		t += " — DrawGraph"
	}
	return t
}

// EnsureUnused compiles cleanly if os isn't needed elsewhere.
var _ = os.PathSeparator
