// Command drawgraph-gui launches the native GPU-rendered DrawGraph desktop application.
package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/app"
	"github.com/neko233-com/DrawGraph/internal/drawio"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/i18n"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/ipc"
)

type windowHost struct {
	win *mygo.Window
	app *app.App
}

func (h *windowHost) Changed(_ *graph.Document) {}
func (h *windowHost) Window() *mygo.Window      { return h.win }
func (h *windowHost) Update(fn func()) {
	if h.win != nil {
		h.win.Update(fn)
	}
}
func (h *windowHost) RebuildMenu() {
	if h.app == nil {
		return
	}
	m := buildMenu(h.app)
	mygo.App.SetMenu(m)
	if h.win != nil {
		h.win.SetMenu(m)
	}
}

func main() {
	hb, err := habits.OpenStore("")
	if err != nil {
		log.Fatalf("habits: %v", err)
	}
	reg, err := icons.OpenRegistry("")
	if err != nil {
		log.Fatalf("icons: %v", err)
	}

	var doc *graph.Document
	var filePath string
	if len(os.Args) > 1 && !strings.HasPrefix(os.Args[1], "-") {
		p := os.Args[1]
		if strings.EqualFold(filepath.Ext(p), ".drawio") || strings.EqualFold(filepath.Ext(p), ".xml") {
			doc, err = drawio.ImportFile(p)
		} else {
			doc, err = graph.LoadFile(p)
		}
		if err == nil {
			filePath = p
			hb.RecordRecentFile(p)
		}
	}
	if doc == nil {
		doc = graph.NewDocument("Untitled Diagram")
		hb.ApplyToDocument(doc)
	}

	host := &windowHost{}
	a := app.New(doc, reg, hb, host)
	a.FilePath = filePath
	host.app = a

	srv, err := ipc.Start("", a)
	if err != nil {
		log.Printf("drawgraph-gui: live IPC unavailable: %v", err)
	} else {
		defer srv.Close()
	}

	mygo.App.SetMenu(buildMenu(a))
	mygo.App.WhenReady(func() {
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:     a.WindowTitle(),
			Width:     1360,
			Height:    860,
			MinWidth:  880,
			MinHeight: 560,
			Content:   ui.View(a.View),
		})
		host.win = win
		a.StartUpdateSchedule()
	})

	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func buildMenu(a *app.App) *mygo.Menu {
	act := func(fn func()) func(*mygo.MenuItem, *mygo.Window) {
		return func(_ *mygo.MenuItem, win *mygo.Window) {
			if win != nil {
				win.Update(fn)
			} else if a.Host != nil {
				a.Host.Update(fn)
			}
		}
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: a.T("menu.file"), Submenu: []*mygo.MenuItem{
			{Label: a.T("menu.file.new"), Accelerator: "CmdOrCtrl+N", Click: act(func() { a.NewBlankDocument() })},
			{Label: a.T("menu.file.open"), Accelerator: "CmdOrCtrl+O", Click: act(func() { a.OpenFromDialog() })},
			{Label: a.T("menu.file.importMermaid"), Click: act(func() { a.MermaidOpen = true })},
			mygo.Separator(),
			{Label: a.T("menu.file.save"), Accelerator: "CmdOrCtrl+S", Click: act(func() { a.SaveToCurrentOrDialog(false) })},
			{Label: a.T("menu.file.saveAs"), Accelerator: "CmdOrCtrl+Shift+S", Click: act(func() { a.SaveToCurrentOrDialog(true) })},
			mygo.Separator(),
			{Label: a.T("menu.file.exportSVG"), Click: act(func() { a.ExportDialog("svg") })},
			{Label: a.T("menu.file.exportPNG"), Click: act(func() { a.ExportDialog("png") })},
			{Label: a.T("menu.file.exportDrawio"), Click: act(func() { a.ExportDialog("drawio") })},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: a.T("menu.format"), Submenu: []*mygo.MenuItem{
			{Label: a.T("menu.format.oneClick"), Accelerator: "CmdOrCtrl+Shift+F", Click: act(func() { a.FormatOneClick("") })},
			{Label: a.T("menu.format.layered"), Click: act(func() { a.FormatWithStrategy("layered", "") })},
			{Label: a.T("menu.format.tree"), Click: act(func() { a.FormatWithStrategy("tree", "") })},
			{Label: a.T("menu.format.grid"), Click: act(func() { a.FormatWithStrategy("grid", "") })},
			{Label: a.T("menu.format.radial"), Click: act(func() { a.FormatWithStrategy("radial", "") })},
			{Label: a.T("menu.format.swimlane"), Click: act(func() { a.FormatWithStrategy("swimlane", "") })},
			{Label: a.T("menu.format.compact"), Click: act(func() { a.FormatWithStrategy("compact", "") })},
			mygo.Separator(),
			{Label: a.T("menu.format.lr"), Click: act(func() { a.FormatOneClick("LR") })},
			{Label: a.T("menu.format.tb"), Click: act(func() { a.FormatOneClick("TB") })},
			mygo.Separator(),
			{Label: a.T("menu.format.icons"), Click: act(func() { a.PluginsOpen = true })},
			{Label: a.T("menu.format.habits"), Click: act(func() { a.HabitsOpen = true })},
		}},
		{Label: a.T("menu.view"), Submenu: []*mygo.MenuItem{
			{Label: a.T("menu.view.commandPalette"), Accelerator: "CmdOrCtrl+K", Click: act(func() { a.PaletteQuery = ""; a.PaletteOpen = true })},
			mygo.Separator(),
			{Label: a.T("menu.view.zoomIn"), Accelerator: "CmdOrCtrl+=", Click: act(func() { a.ZoomIn() })},
			{Label: a.T("menu.view.zoomOut"), Accelerator: "CmdOrCtrl+-", Click: act(func() { a.ZoomOut() })},
			{Label: a.T("menu.view.zoom100"), Accelerator: "CmdOrCtrl+0", Click: act(func() { a.ResetZoom() })},
			{Label: a.T("menu.view.zoomFit"), Accelerator: "CmdOrCtrl+Shift+0", Click: act(func() { a.ZoomToFit() })},
			mygo.Separator(),
			{Label: a.T("menu.view.sidebar"), Click: act(func() { a.SidebarOpen = !a.SidebarOpen })},
			{Label: a.T("menu.view.inspector"), Click: act(func() { a.InspectorOpen = !a.InspectorOpen })},
			{Label: a.T("menu.view.minimap"), Click: act(func() { a.MinimapOpen = !a.MinimapOpen })},
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: a.T("menu.language"), Submenu: languageMenuItems(a, act)},
		{Label: a.T("menu.help"), Submenu: []*mygo.MenuItem{
			{Label: a.T("menu.help.checkUpdates"), Click: act(func() { a.UpdateOpen = true; a.CheckUpdates(true) })},
			{Label: a.T("menu.help.updateSettings"), Click: act(func() { a.UpdateOpen = true })},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}

func languageMenuItems(a *app.App, act func(func()) func(*mygo.MenuItem, *mygo.Window)) []*mygo.MenuItem {
	locs := i18n.SupportedLocales()
	out := make([]*mygo.MenuItem, 0, len(locs))
	for _, loc := range locs {
		code := loc.Code
		out = append(out, &mygo.MenuItem{
			Label: loc.NativeName,
			Click: act(func() { a.SetLanguage(code) }),
		})
	}
	return out
}
