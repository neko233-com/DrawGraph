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
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/ipc"
)

type windowHost struct {
	win *mygo.Window
}

func (h *windowHost) Changed(_ *graph.Document) {}
func (h *windowHost) Window() *mygo.Window      { return h.win }
func (h *windowHost) Update(fn func()) {
	if h.win != nil {
		h.win.Update(fn)
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
		{Label: "File", Submenu: []*mygo.MenuItem{
			{Label: "New Diagram", Accelerator: "CmdOrCtrl+N", Click: act(func() { a.NewBlankDocument() })},
			{Label: "Open…", Accelerator: "CmdOrCtrl+O", Click: act(func() { a.OpenFromDialog() })},
			mygo.Separator(),
			{Label: "Save", Accelerator: "CmdOrCtrl+S", Click: act(func() { a.SaveToCurrentOrDialog(false) })},
			{Label: "Save As…", Accelerator: "CmdOrCtrl+Shift+S", Click: act(func() { a.SaveToCurrentOrDialog(true) })},
			mygo.Separator(),
			{Label: "Export SVG…", Click: act(func() { a.ExportDialog("svg") })},
			{Label: "Export PNG…", Click: act(func() { a.ExportDialog("png") })},
			{Label: "Export draw.io…", Click: act(func() { a.ExportDialog("drawio") })},
			mygo.Separator(),
			{Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
		{Label: "Format", Submenu: []*mygo.MenuItem{
			{Label: "One-Click Format", Accelerator: "CmdOrCtrl+Shift+F", Click: act(func() { a.FormatOneClick("") })},
			{Label: "Layout Left to Right", Click: act(func() { a.FormatOneClick("LR") })},
			{Label: "Layout Top to Bottom", Click: act(func() { a.FormatOneClick("TB") })},
			mygo.Separator(),
			{Label: "Icon Plugins (Git)…", Click: act(func() { a.PluginsOpen = true })},
			{Label: "Persistent User Habits…", Click: act(func() { a.HabitsOpen = true })},
		}},
		{Label: "View", Submenu: []*mygo.MenuItem{
			{Label: "Zoom In", Accelerator: "CmdOrCtrl+=", Click: act(func() { a.ZoomIn() })},
			{Label: "Zoom Out", Accelerator: "CmdOrCtrl+-", Click: act(func() { a.ZoomOut() })},
			{Label: "Zoom to 100%", Accelerator: "CmdOrCtrl+0", Click: act(func() { a.ResetZoom() })},
			{Label: "Zoom to Fit", Accelerator: "CmdOrCtrl+Shift+0", Click: act(func() { a.ZoomToFit() })},
			mygo.Separator(),
			{Label: "Toggle Sidebar", Click: act(func() { a.SidebarOpen = !a.SidebarOpen })},
			{Label: "Toggle Inspector", Click: act(func() { a.InspectorOpen = !a.InspectorOpen })},
			{Label: "Toggle Minimap", Click: act(func() { a.MinimapOpen = !a.MinimapOpen })},
			{Role: mygo.RoleToggleFullScreen},
		}},
		{Label: "Help", Submenu: []*mygo.MenuItem{
			{Label: "Check for Updates…", Click: act(func() { a.UpdateOpen = true; a.CheckUpdates(true) })},
			{Label: "Software Update Settings…", Click: act(func() { a.UpdateOpen = true })},
		}},
		{Role: mygo.RoleWindowMenu},
	})
}
