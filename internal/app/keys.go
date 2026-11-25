package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

func (a *App) handleShortcuts(c *ui.Context) {
	if a.EditingNodeID != "" || a.EditingEdgeID != "" {
		if c.Shortcut(0, ui.KeyEscape) {
			a.EditingNodeID = ""
			a.EditingEdgeID = ""
		} else if c.Shortcut(0, ui.KeyEnter) {
			a.commitInlineEdit()
		}
		return
	}
	if a.PluginsOpen || a.HabitsOpen || a.UpdateOpen || a.PaletteOpen || a.MermaidOpen || a.ShortcutsOpen {
		if c.Shortcut(0, ui.KeyEscape) {
			a.PluginsOpen = false
			a.HabitsOpen = false
			a.UpdateOpen = false
			a.PaletteOpen = false
			a.MermaidOpen = false
			a.ShortcutsOpen = false
		}
		return
	}

	cmd := ui.Cmd
	shift := ui.Shift

	// Shortcuts Cheat-Sheet Modal (? or Cmd/Ctrl+/)
	if c.Shortcut(cmd, ui.KeySlash) || c.Shortcut(shift, ui.KeySlash) {
		a.ShortcutsOpen = !a.ShortcutsOpen
		return
	}

	// Command Palette & Search (Cmd/Ctrl+K or Cmd/Ctrl+F)
	if c.Shortcut(cmd, ui.KeyK) || c.Shortcut(cmd, ui.KeyF) {
		a.PaletteQuery = ""
		a.PaletteOpen = true
		return
	}

	// Panel toggles
	if c.Shortcut(cmd, ui.KeyB) {
		a.SidebarOpen = !a.SidebarOpen
	}
	if c.Shortcut(cmd, ui.KeyBackslash) {
		a.InspectorOpen = !a.InspectorOpen
	}
	if c.Shortcut(cmd, ui.KeyM) {
		a.MinimapOpen = !a.MinimapOpen
	}
	if c.Shortcut(cmd|shift, ui.KeyM) {
		a.MermaidOpen = true
	}

	// Tools
	if c.Shortcut(0, ui.KeyV) {
		a.Tool = ToolSelect
	}
	if c.Shortcut(0, ui.KeyH) {
		a.Tool = ToolHand
	}
	if c.Shortcut(0, ui.KeyC) {
		a.Tool = ToolConnect
	}
	if c.Shortcut(0, ui.KeyR) || c.Shortcut(0, ui.Key1) {
		a.Tool = ToolShape
		a.Shape = graph.KindRounded
	}
	if c.Shortcut(0, ui.KeyO) || c.Shortcut(0, ui.Key2) {
		a.Tool = ToolShape
		a.Shape = graph.KindEllipse
	}
	if c.Shortcut(0, ui.KeyD) || c.Shortcut(0, ui.Key3) {
		a.Tool = ToolShape
		a.Shape = graph.KindDiamond
	}
	if c.Shortcut(0, ui.KeyT) || c.Shortcut(0, ui.Key4) {
		a.Tool = ToolShape
		a.Shape = graph.KindText
	}
	if c.Shortcut(0, ui.KeyG) {
		a.ShowGrid = !a.ShowGrid
	}
	if c.Shortcut(0, ui.KeyF) || c.Shortcut(shift, ui.Key1) {
		a.ZoomToFit()
	}
	if c.Shortcut(shift, ui.Key2) {
		a.ZoomToSelection()
	}

	// Quick 6-strategy layout shortcuts (Alt+1..6)
	for i, strat := range []string{"layered", "tree", "grid", "radial", "swimlane", "compact"} {
		if c.Shortcut(ui.Alt, ui.Key1+ui.Key(i)) {
			a.FormatWithStrategy(strat, "")
		}
	}

	// Z-order shortcuts (Cmd+] / Cmd+[)
	if c.Shortcut(cmd, ui.KeyBracketRight) {
		a.ReorderSelection(graph.ZOrderFront)
	}
	if c.Shortcut(cmd, ui.KeyBracketLeft) {
		a.ReorderSelection(graph.ZOrderBack)
	}

	// Undo / Redo / File / Clipboard / Edit
	if c.Shortcut(cmd, ui.KeyZ) {
		a.Undo()
	}
	if c.Shortcut(cmd|shift, ui.KeyZ) || c.Shortcut(cmd, ui.KeyY) {
		a.Redo()
	}
	if c.Shortcut(cmd, ui.KeyS) {
		a.SaveToCurrentOrDialog(false)
	}
	if c.Shortcut(cmd|shift, ui.KeyS) {
		a.SaveToCurrentOrDialog(true)
	}
	if c.Shortcut(cmd, ui.KeyO) {
		a.OpenFromDialog()
	}
	if c.Shortcut(cmd, ui.KeyU) {
		a.ImportMediaFromDialog()
	}
	if c.Shortcut(cmd, ui.KeyC) {
		a.CopySelection()
	}
	if c.Shortcut(cmd, ui.KeyX) {
		a.CutSelection()
	}
	if c.Shortcut(cmd, ui.KeyV) {
		a.PasteClipboard()
	}
	if c.Shortcut(cmd, ui.KeyG) {
		a.GroupSelection()
	}
	if c.Shortcut(cmd|shift, ui.KeyG) {
		a.UngroupSelection()
	}
	if c.Shortcut(cmd, ui.KeyL) {
		a.ToggleLockSelection()
	}
	if c.Shortcut(cmd|shift, ui.KeyH) {
		a.ResetOrigin()
	}
	if c.Shortcut(cmd, ui.KeyN) {
		a.NewBlankDocument()
	}
	if c.Shortcut(cmd, ui.KeyA) {
		a.SelectAll()
	}
	if c.Shortcut(cmd, ui.KeyD) {
		a.DuplicateSelection()
	}
	if c.Shortcut(0, ui.KeyDelete) || c.Shortcut(0, ui.KeyBackspace) {
		a.DeleteSelection()
	}
	if c.Shortcut(0, ui.KeyEscape) {
		a.ContextMenuPos = nil
		a.Tool = ToolSelect
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
	}
	if c.Shortcut(0, ui.KeyEnter) || c.Shortcut(0, ui.KeyF2) {
		if sel := a.singleSelectedNode(); sel != nil {
			a.EditingNodeID = sel.ID
			a.EditingText = sel.Label
			a.EditFocusSeq++
		} else if es := a.singleSelectedEdge(); es != nil {
			a.EditingEdgeID = es.ID
			a.EditingText = es.Label
			a.EditFocusSeq++
		}
	}

	// One-click elegant formatting shortcut!
	if c.Shortcut(cmd|shift, ui.KeyF) {
		a.FormatOneClick("")
	}

	// Zoom shortcuts
	if c.Shortcut(cmd, ui.KeyEqual) {
		a.ZoomIn()
	}
	if c.Shortcut(cmd, ui.KeyMinus) {
		a.ZoomOut()
	}
	if c.Shortcut(cmd, ui.Key0) {
		a.ResetZoom()
	}
	if c.Shortcut(cmd|shift, ui.Key0) {
		a.ZoomToFit()
	}

	// Nudge selected nodes
	nudge := func(dx, dy float64) {
		if len(a.SelectedNodes) == 0 {
			return
		}
		a.Hist.Snapshot()
		for id := range a.SelectedNodes {
			if n := a.Page().NodeByID(id); n != nil && !n.Locked {
				n.X += dx
				n.Y += dy
			}
		}
		a.Page().MarkDirty()
		a.notifyChanged()
	}
	step := a.Doc().GridSize
	if step <= 0 {
		step = 10
	}
	if c.Shortcut(0, ui.KeyLeft) {
		nudge(-step, 0)
	}
	if c.Shortcut(0, ui.KeyRight) {
		nudge(step, 0)
	}
	if c.Shortcut(0, ui.KeyUp) {
		nudge(0, -step)
	}
	if c.Shortcut(0, ui.KeyDown) {
		nudge(0, step)
	}
	_ = geo.V
}
