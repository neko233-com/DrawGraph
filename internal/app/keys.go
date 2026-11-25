package app

import (
	"github.com/egoist/mygo/ui"
	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

func (a *App) handleShortcuts(c *ui.Context) {
	if a.EditingNodeID != "" {
		if c.Shortcut(0, ui.KeyEscape) {
			a.EditingNodeID = ""
		} else if c.Shortcut(0, ui.KeyEnter) {
			a.commitInlineEdit()
		}
		return
	}
	if a.PluginsOpen || a.HabitsOpen {
		if c.Shortcut(0, ui.KeyEscape) {
			a.PluginsOpen = false
			a.HabitsOpen = false
		}
		return
	}

	cmd := ui.Cmd
	shift := ui.Shift

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
	if c.Shortcut(0, ui.KeyR) {
		a.Tool = ToolShape
		a.Shape = graph.KindRounded
	}
	if c.Shortcut(0, ui.KeyO) {
		a.Tool = ToolShape
		a.Shape = graph.KindEllipse
	}
	if c.Shortcut(0, ui.KeyD) {
		a.Tool = ToolShape
		a.Shape = graph.KindDiamond
	}
	if c.Shortcut(0, ui.KeyT) {
		a.Tool = ToolShape
		a.Shape = graph.KindText
	}

	// Undo / Redo / File / Edit
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
	if c.Shortcut(cmd, ui.KeyV) {
		a.PasteClipboardMedia()
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
		a.Tool = ToolSelect
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
	}
	if c.Shortcut(0, ui.KeyEnter) || c.Shortcut(0, ui.KeyF2) {
		if sel := a.singleSelectedNode(); sel != nil {
			a.EditingNodeID = sel.ID
			a.EditingText = sel.Label
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
