package app

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/action"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/render"
)

func (a *App) canvasArea(c *ui.Context, tk *theme.Tokens) {
	wrap := ui.Box(c.Key("canvas-wrap")).Grow(1).Basis(0).MinHeight(0).Clip()
	dragOver := wrap.FileDragOver()
	dropped := wrap.DroppedFiles()
	wrap.Children(func() {
		cv := ui.Box(c.Key("canvas")).Fill().
			Focusable().FocusRing(false).
			Bind(&a.CanvasHandle).
			Cursor(a.canvasCursor())
		cv.Label("Canvas")
		cv.Draw(func(p *ui.Painter, r ui.Rect) {
			t0 := time.Now()
			var preview *[2]geo.Vec
			if a.drag == dragConnect && a.connectSrcID != "" {
				if src := a.Page().NodeByID(a.connectSrcID); src != nil {
					p0, _ := geo.AnchorPoint(src.Box(), geo.AnchorAuto, a.connectEndPt)
					pv := [2]geo.Vec{p0, a.connectEndPt}
					preview = &pv
				}
			}
			a.LastStats = render.PaintSceneWithStats(p, r, render.SceneState{
				Doc:            a.Doc(),
				Page:           a.Page(),
				Icons:          a.Icons,
				ResolveMedia:   a.resolveMedia,
				ActiveVideos:   a.activeVideoMap(),
				Cam:            a.Cam,
				Dark:           a.DarkActive,
				ShowGrid:       a.ShowGrid,
				SelectedNodes:  a.SelectedNodes,
				SelectedEdges:  a.SelectedEdges,
				HoveredNodeID:  a.HoveredNodeID,
				ConnectPreview: preview,
				MarqueeBox:     a.marqueeBox,
				GuideLines:     a.GuideLines,
			})
			a.LastPaintMicros = time.Since(t0).Microseconds()
		})
		cv.HandleInput(func(ev ui.InputEvent) bool {
			return a.handleCanvasInput(c, ev)
		})

		a.videoOverlayLayer(c, tk)

		if len(a.Page().Nodes) == 0 && a.drag == dragNone && !dragOver {
			a.emptyHeroOverlay(c, tk)
		}
		if a.Tool == ToolShape {
			a.shapeModeBanner(c, tk)
		}

		if dragOver {
			ui.Box(c.Key("drop-overlay")).Absolute().Top(12).Left(12).Right(12).Bottom(12).Radius(12).
				Background(vercelBlue.Alpha(0.10)).BorderWidth(2).BorderColor(vercelBlue).Center().Children(func() {
				ui.Box(c).Padding(12, 20).Radius(10).Background(tk.BgElevated).BorderWidth(1).BorderColor(vercelBlue).Children(func() {
					ui.Text(c, a.T("canvas.dropHint")).FontSize(14).FontWeight(600).TextColor(vercelBlue)
				})
			})
		}

		a.inlineTextEditor(c, tk)
		a.selectionQuickBar(c, tk)
		a.contextMenuOverlay(c, tk)
		if a.MinimapOpen {
			a.minimapOverlay(c, tk)
		}
	})

	if len(dropped) > 0 {
		localRect := ui.Rect{X: 0, Y: 0, W: max(1, a.CanvasRect.W), H: max(1, a.CanvasRect.H)}
		px, py := a.lastPointerLocal[0], a.lastPointerLocal[1]
		if px <= 0 && py <= 0 {
			px, py = localRect.W/2, localRect.H/2
		}
		dropWorld := a.Cam.ScreenToWorld(localRect, px, py)
		a.DropMediaFiles(dropped, &dropWorld)
	}
}

func (a *App) canvasCursor() ui.Cursor {
	if a.spaceHeld || a.Tool == ToolHand {
		if a.drag == dragPan {
			return ui.CursorGrabbing
		}
		return ui.CursorGrab
	}
	if a.Tool == ToolShape || a.Tool == ToolConnect {
		return ui.CursorCrosshair
	}
	return ui.CursorDefault
}

func (a *App) handleCanvasInput(c *ui.Context, ev ui.InputEvent) bool {
	if a.CanvasRect.W <= 0 {
		a.CanvasRect = ui.Rect{X: 0, Y: 0, W: 900, H: 600}
	}
	// InputEvent.X/Y are relative to the canvas element box!
	localRect := ui.Rect{X: 0, Y: 0, W: a.CanvasRect.W, H: a.CanvasRect.H}
	worldPt := a.Cam.ScreenToWorld(localRect, ev.X, ev.Y)

	switch ev.Kind {
	case ui.InputKeyDown:
		if ev.Key == ui.KeySpace {
			a.spaceHeld = true
			return true
		}
	case ui.InputKeyUp:
		if ev.Key == ui.KeySpace {
			a.spaceHeld = false
			return true
		}
	case ui.InputScroll:
		if ev.Mods&ui.Cmd != 0 || ev.Mods&ui.Ctrl != 0 || (a.WheelMode == "zoom" && ev.Mods == 0 && ev.DY != 0 && ev.DX == 0) {
			factor := math.Exp(-float64(ev.DY) * 0.004)
			a.zoomAt(float64(ev.X), float64(ev.Y), factor)
		} else {
			z := math.Max(a.Cam.Zoom, 0.2)
			a.Cam.X -= float64(ev.DX) / z
			a.Cam.Y -= float64(ev.DY) / z
		}
		return true
	case ui.InputZoom:
		a.zoomAt(float64(ev.X), float64(ev.Y), 1+float64(ev.Scale))
		return true

	case ui.InputPointerMove:
		a.lastPointerLocal = [2]float32{ev.X, ev.Y}
		if a.drag == dragNone {
			if hit := a.Page().HitNode(worldPt); hit != nil {
				a.HoveredNodeID = hit.ID
			} else {
				a.HoveredNodeID = ""
			}
			return false
		}
		a.handleDragMove(ev, worldPt)
		return true

	case ui.InputPointerDown:
		a.CanvasHandle.Focus(c)
		a.ContextMenuPos = nil
		// Right-click opens canvas / selection context menu cleanly.
		if ev.Button == 1 {
			if hit := a.Page().HitNode(worldPt); hit != nil && !a.SelectedNodes[hit.ID] {
				a.SelectedNodes = map[string]bool{hit.ID: true}
				a.SelectedEdges = map[string]bool{}
			}
			pos := [2]float32{ev.X, ev.Y}
			a.ContextMenuPos = &pos
			return true
		}
		// Middle button, Space+click, or Hand tool -> pan.
		if ev.Button == 2 || a.spaceHeld || a.Tool == ToolHand {
			a.drag = dragPan
			a.dragStartDip = [2]float32{ev.X, ev.Y}
			a.dragStartCam = a.Cam
			return true
		}
		if ev.Button != 0 {
			return false
		}

		// ToolShape drops the chosen shape at clicked coordinates.
		if a.Tool == ToolShape {
			n := a.AddShapeAt(a.Shape, "", "", worldPt.X-80, worldPt.Y-36, true)
			a.Tool = ToolSelect
			a.EditingNodeID = n.ID
			a.EditingEdgeID = ""
			a.EditingText = n.Label
			a.EditFocusSeq++
			return true
		}

		// Check resize corner handles on single selected node.
		if sel := a.singleSelectedNode(); sel != nil && a.Tool == ToolSelect {
			if corner := hitCornerHandle(sel, localRect, a.Cam, ev.X, ev.Y); corner >= 0 {
				a.Hist.Snapshot()
				a.drag = dragResizeNode
				a.dragCorner = corner
				a.dragStartWorld = worldPt
				a.dragNodeOrig = map[string]geo.Box{sel.ID: sel.Box()}
				return true
			}
			// Check side port handle -> start rubber-band edge connection!
			if hitSidePort(sel, localRect, a.Cam, ev.X, ev.Y) {
				a.drag = dragConnect
				a.connectSrcID = sel.ID
				a.connectEndPt = worldPt
				return true
			}
		}

		// Connect tool starts connector from clicked node.
		if a.Tool == ToolConnect {
			if hit := a.Page().HitNode(worldPt); hit != nil {
				a.drag = dragConnect
				a.connectSrcID = hit.ID
				a.connectEndPt = worldPt
				a.SelectedNodes = map[string]bool{hit.ID: true}
				return true
			}
		}

		// Double-click opens inline text editor for node or connector label.
		if ev.Clicks >= 2 {
			if hit := a.Page().HitNode(worldPt); hit != nil {
				a.SelectedNodes = map[string]bool{hit.ID: true}
				a.SelectedEdges = map[string]bool{}
				a.EditingNodeID = hit.ID
				a.EditingEdgeID = ""
				a.EditingText = hit.Label
				a.EditFocusSeq++
				return true
			}
			if edge := a.Page().HitEdge(worldPt, 12/math.Max(a.Cam.Zoom, 0.25)); edge != nil {
				a.SelectedNodes = map[string]bool{}
				a.SelectedEdges = map[string]bool{edge.ID: true}
				a.EditingNodeID = ""
				a.EditingEdgeID = edge.ID
				a.EditingText = edge.Label
				a.EditFocusSeq++
				return true
			}
		}

		// Single click selects node or edge, and starts node move or marquee drag.
		if hit := a.Page().HitNode(worldPt); hit != nil {
			shift := ev.Mods&ui.Shift != 0
			if !a.SelectedNodes[hit.ID] {
				if !shift {
					a.SelectedNodes = map[string]bool{}
					a.SelectedEdges = map[string]bool{}
				}
				a.SelectedNodes[hit.ID] = true
			} else if shift {
				delete(a.SelectedNodes, hit.ID)
			}
			a.Hist.Snapshot()
			a.drag = dragMoveNodes
			a.dragStartWorld = worldPt
			a.dragNodeOrig = map[string]geo.Box{}
			// Include children inside selected containers/swimlanes so moving a group moves its members cleanly!
			queue := make([]string, 0, len(a.SelectedNodes))
			for id := range a.SelectedNodes {
				queue = append(queue, id)
			}
			for len(queue) > 0 {
				curID := queue[0]
				queue = queue[1:]
				n := a.Page().NodeByID(curID)
				if n == nil {
					continue
				}
				a.dragNodeOrig[n.ID] = n.Box()
				if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
					for _, ch := range a.Page().Nodes {
						if ch.ParentID == n.ID {
							if _, seen := a.dragNodeOrig[ch.ID]; !seen {
								queue = append(queue, ch.ID)
							}
						}
					}
				}
			}
			return true
		}

		if edge := a.Page().HitEdge(worldPt, 10/math.Max(a.Cam.Zoom, 0.25)); edge != nil {
			a.SelectedNodes = map[string]bool{}
			a.SelectedEdges = map[string]bool{edge.ID: true}
			return true
		}

		// Empty canvas press -> clear selection and begin marquee box.
		a.SelectedNodes = map[string]bool{}
		a.SelectedEdges = map[string]bool{}
		a.EditingNodeID = ""
		a.EditingEdgeID = ""
		a.drag = dragMarquee
		a.dragStartWorld = worldPt
		b := geo.B(worldPt.X, worldPt.Y, 0, 0)
		a.marqueeBox = &b
		return true

	case ui.InputPointerUp:
		if a.drag == dragNone {
			return false
		}
		a.handleDragEnd(worldPt)
		return true
	}
	return false
}

func (a *App) handleDragMove(ev ui.InputEvent, worldPt geo.Vec) {
	switch a.drag {
	case dragPan:
		z := math.Max(a.Cam.Zoom, 0.2)
		a.Cam.X = a.dragStartCam.X + float64(ev.X-a.dragStartDip[0])/z
		a.Cam.Y = a.dragStartCam.Y + float64(ev.Y-a.dragStartDip[1])/z
	case dragMoveNodes:
		dx := worldPt.X - a.dragStartWorld.X
		dy := worldPt.Y - a.dragStartWorld.Y
		grid := a.Doc().GridSize
		a.GuideLines = nil
		movingSet := make(map[string]bool, len(a.dragNodeOrig))
		var unionBox geo.Box
		hasBox := false
		for id, orig := range a.dragNodeOrig {
			movingSet[id] = true
			b := geo.B(orig.X+dx, orig.Y+dy, orig.W, orig.H)
			if a.Doc().SnapToGrid {
				b.X = geo.Snap(b.X, grid)
				b.Y = geo.Snap(b.Y, grid)
			}
			if !hasBox {
				unionBox = b
				hasBox = true
			} else {
				unionBox = unionBox.Union(b)
			}
		}
		var snapDX, snapDY float64
		if hasBox && ev.Mods&ui.Alt == 0 {
			tol := 7.0 / math.Max(a.Cam.Zoom, 0.3)
			var guides []graph.SnapGuide
			snapDX, snapDY, guides = a.Page().EnsureIndex().FindSnapGuides(movingSet, unionBox, tol)
			a.GuideLines = guides
		}
		for id, orig := range a.dragNodeOrig {
			n := a.Page().NodeByID(id)
			if n == nil || n.Locked {
				continue
			}
			nx := orig.X + dx
			ny := orig.Y + dy
			if a.Doc().SnapToGrid {
				nx = geo.Snap(nx, grid)
				ny = geo.Snap(ny, grid)
			}
			n.X = nx + snapDX
			n.Y = ny + snapDY
		}
		a.Page().MarkDirty()
	case dragResizeNode:
		dx := worldPt.X - a.dragStartWorld.X
		dy := worldPt.Y - a.dragStartWorld.Y
		for id, orig := range a.dragNodeOrig {
			n := a.Page().NodeByID(id)
			if n == nil || n.Locked {
				continue
			}
			const minSz = 40.0
			switch a.dragCorner {
			case 0: // top-left
				n.W = math.Max(minSz, orig.W-dx)
				n.H = math.Max(minSz, orig.H-dy)
				n.X = orig.X + orig.W - n.W
				n.Y = orig.Y + orig.H - n.H
			case 1: // top-right
				n.W = math.Max(minSz, orig.W+dx)
				n.H = math.Max(minSz, orig.H-dy)
				n.Y = orig.Y + orig.H - n.H
			case 2: // bottom-right
				n.W = math.Max(minSz, orig.W+dx)
				n.H = math.Max(minSz, orig.H+dy)
			case 3: // bottom-left
				n.W = math.Max(minSz, orig.W-dx)
				n.H = math.Max(minSz, orig.H+dy)
				n.X = orig.X + orig.W - n.W
			}
			if a.Doc().SnapToGrid && n.Kind != graph.KindImage && n.Kind != graph.KindVideo {
				g := a.Doc().GridSize
				n.W = math.Max(minSz, geo.Snap(n.W, g))
				n.H = math.Max(minSz, geo.Snap(n.H, g))
			}
			// Preserve natural aspect ratio when resizing media nodes unless Shift is held.
			if (n.Kind == graph.KindImage || n.Kind == graph.KindVideo) && ev.Mods&ui.Shift == 0 && orig.H > 0 {
				ratio := orig.W / orig.H
				n.H = math.Max(minSz, math.Round(n.W/ratio))
				if a.dragCorner == 0 || a.dragCorner == 1 {
					n.Y = orig.Y + orig.H - n.H
				}
			}
		}
	case dragConnect:
		a.connectEndPt = worldPt
	case dragMarquee:
		b := geo.B(a.dragStartWorld.X, a.dragStartWorld.Y, worldPt.X-a.dragStartWorld.X, worldPt.Y-a.dragStartWorld.Y)
		a.marqueeBox = &b
	}
}

func (a *App) handleDragEnd(worldPt geo.Vec) {
	switch a.drag {
	case dragMoveNodes, dragResizeNode:
		a.Page().MarkDirty()
		a.notifyChanged()
	case dragConnect:
		if hit := a.Page().HitNode(worldPt); hit != nil && hit.ID != a.connectSrcID {
			a.Hist.Snapshot()
			e := a.Habits.NewEdgeFromHabits(a.connectSrcID, hit.ID, "")
			if err := a.Page().AddEdge(e); err == nil {
				a.SelectedNodes = map[string]bool{}
				a.SelectedEdges = map[string]bool{e.ID: true}
				a.Habits.RecordEdgeHabit(e)
				a.notifyChanged()
			}
		}
		a.connectSrcID = ""
		if a.Tool == ToolConnect {
			a.Tool = ToolSelect
		}
	case dragMarquee:
		if a.marqueeBox != nil {
			norm := a.marqueeBox.Normalized()
			if norm.W > 6 || norm.H > 6 {
				a.SelectedNodes = map[string]bool{}
				for _, n := range a.Page().EnsureIndex().QueryRect(norm) {
					if norm.Overlaps(n.Box()) {
						a.SelectedNodes[n.ID] = true
					}
				}
			}
		}
		a.marqueeBox = nil
	}
	a.GuideLines = nil
	a.drag = dragNone
	a.dragNodeOrig = nil
}

func (a *App) inlineTextEditor(c *ui.Context, tk *theme.Tokens) {
	if (a.EditingNodeID == "" && a.EditingEdgeID == "") || a.CanvasRect.W <= 0 {
		return
	}
	localRect := ui.Rect{X: 0, Y: 0, W: a.CanvasRect.W, H: a.CanvasRect.H}
	var sx, sy, sw float32
	if a.EditingNodeID != "" {
		n := a.Page().NodeByID(a.EditingNodeID)
		if n == nil {
			a.EditingNodeID = ""
			return
		}
		sx, sy = a.Cam.WorldToScreen(localRect, geo.V(n.X, n.Y))
		sy += float32(n.H*a.Cam.Zoom)/2 - 18
		sw = max(float32(140), float32(n.W*a.Cam.Zoom))
	} else {
		e := a.Page().EdgeByID(a.EditingEdgeID)
		if e == nil {
			a.EditingEdgeID = ""
			return
		}
		mid := geo.PolylineMidpoint(a.Page().EdgePoints(e))
		sx, sy = a.Cam.WorldToScreen(localRect, mid)
		sx -= 80
		sy -= 18
		sw = 160
	}
	box := ui.Box(c.Key("inline-edit")).Absolute().
		Left(sx).Top(sy).
		Width(sw).Padding(4).Radius(8).
		Background(tk.BgElevated).
		BorderWidth(1.5).BorderColor(tk.Primary)
	box.Children(func() {
		inp := field.Input(c, &a.EditingText, field.InputProps{
			Variant:     field.InputBorderless,
			Size:        theme.Small,
			Placeholder: a.T("insp.label") + "...",
		})
		inp.FillWidth()
		inp.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEnter && k.Mods == 0 {
				a.commitInlineEdit()
				return true
			}
			if k.Key == ui.KeyEscape {
				a.EditingNodeID = ""
				a.EditingEdgeID = ""
				return true
			}
			return false
		})
	})
}

func (a *App) commitInlineEdit() {
	if a.EditingNodeID != "" {
		if n := a.Page().NodeByID(a.EditingNodeID); n != nil {
			a.Hist.Snapshot()
			n.Label = a.EditingText
			a.Page().MarkDirty()
			a.notifyChanged()
		}
	} else if a.EditingEdgeID != "" {
		if e := a.Page().EdgeByID(a.EditingEdgeID); e != nil {
			a.Hist.Snapshot()
			e.Label = a.EditingText
			a.Page().MarkDirty()
			a.notifyChanged()
		}
	}
	a.EditingNodeID = ""
	a.EditingEdgeID = ""
}

func (a *App) selectionQuickBar(c *ui.Context, tk *theme.Tokens) {
	if len(a.SelectedNodes) == 0 || a.drag != dragNone || a.EditingNodeID != "" || a.CanvasRect.W <= 0 {
		return
	}
	var selBox geo.Box
	first := true
	lockedAny := false
	for id := range a.SelectedNodes {
		if n := a.Page().NodeByID(id); n != nil {
			if first {
				selBox = n.Box()
				first = false
			} else {
				selBox = selBox.Union(n.Box())
			}
			if n.Locked {
				lockedAny = true
			}
		}
	}
	if first {
		return
	}
	localRect := ui.Rect{X: 0, Y: 0, W: a.CanvasRect.W, H: a.CanvasRect.H}
	sx, sy := a.Cam.WorldToScreen(localRect, geo.V(selBox.X+selBox.W/2, selBox.Y))
	barY := max(float32(8), sy-44)
	barX := max(float32(8), min(a.CanvasRect.W-340, sx-160))

	bar := ui.Row(c.Key("sel-quick-bar")).Absolute().Left(barX).Top(barY).Height(34).Gap(4).Padding(0, 8).
		Radius(9).AlignItems(ui.Center).Background(tk.BgElevated).
		BorderWidth(1).BorderColor(vercelBlue.Alpha(0.35))
	bar.Children(func() {
		if vercelTintPill(c, tk, vercelBlue, nil, a.T("ctx.duplicate"), false).Clicked() {
			a.DuplicateSelection()
		}
		if len(a.SelectedNodes) >= 2 {
			if vercelTintPill(c, tk, vercelViolet, nil, a.T("ctx.group"), false).Clicked() {
				a.GroupSelection()
			}
		} else if n := a.singleSelectedNode(); n != nil && (n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane) {
			if vercelTintPill(c, tk, vercelViolet, nil, a.T("ctx.ungroup"), false).Clicked() {
				a.UngroupSelection()
			}
		}
		if vercelTintPill(c, tk, vercelCyan, nil, a.T("ctx.front"), false).Clicked() {
			a.ReorderSelection(graph.ZOrderFront)
		}
		if vercelTintPill(c, tk, vercelIndigo, nil, a.T("ctx.back"), false).Clicked() {
			a.ReorderSelection(graph.ZOrderBack)
		}
		lockLbl := a.T("ctx.lock")
		if lockedAny {
			lockLbl = a.T("ctx.unlock")
		}
		if vercelTintPill(c, tk, vercelAmber, nil, lockLbl, lockedAny).Clicked() {
			a.ToggleLockSelection()
		}
		if vercelTintPill(c, tk, vercelPink, nil, a.T("ctx.delete"), false).Clicked() {
			a.DeleteSelection()
		}
	})
}

func (a *App) shapeModeBanner(c *ui.Context, tk *theme.Tokens) {
	cw := a.CanvasRect.W
	if cw <= 0 {
		cw = 760
	}
	shapeName := a.T("shape." + string(a.Shape))
	if shapeName == "shape."+string(a.Shape) {
		shapeName = string(a.Shape)
	}
	accent := shapeCategoryColor(a.Shape)
	x := max(float32(16), cw/2-170)
	ui.Row(c.Key("shape-mode-banner")).Absolute().Left(x).Top(14).Padding(6, 14).Radius(99).
		Gap(8).AlignItems(ui.Center).Background(tk.BgElevated).
		BorderWidth(1.5).BorderColor(accent.Alpha(0.7)).Children(func() {
		ui.Box(c).Size(8, 8).Radius(4).Background(accent)
		ui.Text(c, a.T("tb.placeShape", shapeName)).FontSize(12).FontWeight(600).TextColor(accent)
	})
}

func (a *App) emptyHeroOverlay(c *ui.Context, tk *theme.Tokens) {
	if a.CanvasRect.W > 0 && (a.CanvasRect.W < 420 || a.CanvasRect.H < 320) {
		return
	}
	cw := a.CanvasRect.W
	if cw <= 0 {
		cw = 760
	}
	w := min(float32(560), cw-48)
	left := (cw - w) / 2
	// Positioned cleanly in lower band so center-canvas mouse/drop tests remain unobstructed.
	card := ui.Column(c.Key("hero-empty-card")).Absolute().Left(left).Bottom(44).Width(w).
		Radius(12).Background(tk.BgElevated.Alpha(0.95)).
		BorderWidth(1).BorderColor(tk.BorderSecondary).Clip()
	card.Children(func() {
		vercelSpectrumBar(c, 3)
		ui.Column(c).Padding(14, 16).Gap(10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Box(c).Size(10, 10).Radius(5).LinearGradient(ui.LinearGradient{From: vercelPink, To: vercelBlue, Angle: 90, Oklab: true})
				ui.Text(c, a.T("hero.title")).FontSize(14).FontWeight(700).TextColor(tk.Text)
			})
			ui.Text(c, a.T("hero.subtitle")).FontSize(11).LineHeight(1.45).TextColor(tk.TextSecondary)

			ui.Row(c).Gap(6).Wrap().Children(func() {
				if vercelGradientButton(c, nil, a.T("tpl.arch"), true).Clicked() {
					a.ApplyBuiltinTemplate("arch")
				}
				if vercelTintPill(c, tk, vercelEmerald, nil, a.T("tpl.flow"), false).Clicked() {
					a.ApplyBuiltinTemplate("flow")
				}
				if vercelTintPill(c, tk, vercelViolet, nil, a.T("hero.openMermaid"), false).Clicked() {
					a.MermaidOpen = true
				}
				if vercelTintPill(c, tk, vercelCyan, nil, a.T("hero.openMedia"), false).Clicked() {
					a.ImportMediaFromDialog()
				}
				if vercelTintPill(c, tk, vercelPink, nil, a.T("hero.shortcutsBtn"), false).Clicked() {
					a.ShortcutsOpen = true
				}
			})

			mod := platformMod()
			ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
				kbdBadge(c, tk, "V "+a.T("keys.select"), vercelBlue)
				kbdBadge(c, tk, "C "+a.T("keys.connect"), vercelViolet)
				kbdBadge(c, tk, "R/O/D", vercelPink)
				kbdBadge(c, tk, mod+"+K", vercelCyan)
				kbdBadge(c, tk, mod+"+Shift+F", vercelEmerald)
				kbdBadge(c, tk, "? "+a.T("keys.shortcutsModal"), vercelAmber)
			})
		})
	})
}

func (a *App) contextMenuOverlay(c *ui.Context, tk *theme.Tokens) {
	if a.ContextMenuPos == nil || a.CanvasRect.W <= 0 {
		return
	}
	mx := max(float32(8), min(a.CanvasRect.W-190, a.ContextMenuPos[0]))
	my := max(float32(8), min(a.CanvasRect.H-240, a.ContextMenuPos[1]))
	menu := ui.Column(c.Key("canvas-ctx-menu")).Absolute().Left(mx).Top(my).Width(178).Padding(6).Gap(2).
		Radius(10).Background(tk.BgElevated).BorderWidth(1).BorderColor(tk.BorderSecondary)
	menu.Children(func() {
		btn := func(label string, fn func()) {
			b := action.Button(c, label, action.ButtonProps{Variant: action.Text, Size: theme.Small})
			b.FillWidth()
			if b.Clicked() {
				a.ContextMenuPos = nil
				fn()
			}
		}
		if len(a.SelectedNodes) > 0 {
			btn(a.T("ctx.duplicate"), func() { a.DuplicateSelection() })
			if len(a.SelectedNodes) >= 2 {
				btn(a.T("ctx.group"), func() { a.GroupSelection() })
			}
			btn(a.T("ctx.front"), func() { a.ReorderSelection(graph.ZOrderFront) })
			btn(a.T("ctx.back"), func() { a.ReorderSelection(graph.ZOrderBack) })
			btn(a.T("ctx.lock"), func() { a.ToggleLockSelection() })
			btn(a.T("ctx.delete"), func() { a.DeleteSelection() })
		} else {
			btn(a.T("ctx.formatSel"), func() { a.FormatOneClick("") })
			btn(a.T("tb.zoomFit"), func() { a.ZoomToFit() })
			btn(a.T("status.origin"), func() { a.ResetOrigin() })
			btn(a.T("menu.view.commandPalette"), func() { a.PaletteOpen = true; a.PaletteQuery = "" })
		}
	})
	_ = sort.Strings
}

func (a *App) videoOverlayLayer(c *ui.Context, tk *theme.Tokens) {
	if a.Host == nil || a.Host.Window() == nil || a.CanvasRect.W <= 0 {
		return
	}
	localRect := ui.Rect{X: 0, Y: 0, W: a.CanvasRect.W, H: a.CanvasRect.H}
	z := float32(a.Cam.Zoom)
	for _, n := range a.Page().Nodes {
		if n.Kind != graph.KindVideo || n.Media == nil {
			continue
		}
		vp := a.ensureVideoPlayer(n)
		if vp == nil || vp.player == nil || !vp.player.Ready() {
			continue
		}
		sx, sy := a.Cam.WorldToScreen(localRect, geo.V(n.X, n.Y))
		sw, sh := float32(n.W)*z, float32(n.H)*z
		if sx+sw < 0 || sy+sh < 0 || sx > localRect.W || sy > localRect.H {
			continue
		}
		fit := ui.Contain
		if n.Media.Fit == "cover" {
			fit = ui.Cover
		} else if n.Media.Fit == "fill" {
			fit = ui.FillBox
		}
		rad := float32(n.Style.Radius) * z
		if rad == 0 {
			rad = 10 * z
		}
		box := ui.Box(c.Key("vid-box-" + n.ID)).Absolute().
			Left(sx).Top(sy).Width(sw).Height(sh).
			Radius(rad).Clip().Background(ui.Hex("#0F172A"))
		box.Children(func() {
			ui.Video(c.Key("vid-"+n.ID), vp.player).Fill().Fit(fit)
			if a.SelectedNodes[n.ID] || a.HoveredNodeID == n.ID {
				lbl := "Pause"
				if !vp.player.Playing() {
					lbl = "Play"
				}
				ctrl := ui.ButtonBase(c.Key("vid-toggle-" + n.ID)).Absolute().Left(8).Bottom(8).
					Height(24).PaddingX(8).Radius(6).Background(ui.RGBA(15, 23, 42, 0.78)).
					Children(func() {
						ui.Text(c, lbl).FontSize(11).TextColor(ui.Hex("#F8FAFC"))
					})
				if ctrl.Clicked() {
					if vp.player.Playing() {
						vp.player.Pause()
					} else {
						vp.player.Play()
					}
				}
			}
		})
	}
}

func (a *App) minimapOverlay(c *ui.Context, tk *theme.Tokens) {
	if len(a.Page().Nodes) == 0 {
		return
	}
	mm := ui.Box(c.Key("minimap")).Absolute().Right(12).Bottom(12).
		Width(164).Height(104).Radius(8).Clip().
		Background(tk.BgElevated.Alpha(0.94)).
		BorderWidth(1).BorderColor(vercelBlue.Alpha(0.32))
	mm.HandleInput(func(ev ui.InputEvent) bool {
		if a.CanvasRect.W <= 0 {
			return false
		}
		box, ok := a.Page().Bounds()
		if !ok {
			return false
		}
		box = box.Expand(64)
		const mw, mh = 164.0, 104.0
		s := math.Min((mw-12)/box.W, (mh-12)/box.H)
		ox := (mw - box.W*s) / 2
		oy := (mh - box.H*s) / 2
		switch ev.Kind {
		case ui.InputPointerDown:
			if ev.Button == 0 {
				a.minimapDrag = true
			}
		case ui.InputPointerUp:
			a.minimapDrag = false
		}
		if a.minimapDrag && (ev.Kind == ui.InputPointerDown || ev.Kind == ui.InputPointerMove) {
			wx := box.X + (float64(ev.X)-ox)/s
			wy := box.Y + (float64(ev.Y)-oy)/s
			z := math.Max(a.Cam.Zoom, 0.05)
			a.Cam.X = float64(a.CanvasRect.W)/(2*z) - wx
			a.Cam.Y = float64(a.CanvasRect.H)/(2*z) - wy
			return true
		}
		return false
	})
	mm.Draw(func(p *ui.Painter, r ui.Rect) {
		p.Clip(r, 8, func() {
			box, ok := a.Page().Bounds()
			if !ok {
				return
			}
			box = box.Expand(32)
			s := math.Min(float64(r.W-12)/box.W, float64(r.H-12)/box.H)
			ox := float64(r.X) + (float64(r.W)-box.W*s)/2 - box.X*s
			oy := float64(r.Y) + (float64(r.H)-box.H*s)/2 - box.Y*s
			for _, n := range a.Page().Nodes {
				nr := ui.Rect{
					X: float32(ox + n.X*s),
					Y: float32(oy + n.Y*s),
					W: max(3, float32(n.W*s)),
					H: max(2, float32(n.H*s)),
				}
				col := tk.TextQuaternary.Alpha(0.55)
				if a.SelectedNodes[n.ID] {
					col = vercelBlue
				}
				p.Fill(nr, col, 1)
			}
			// Viewport box
			if a.CanvasRect.W > 0 {
				vx := -a.Cam.X
				vy := -a.Cam.Y
				vw := float64(a.CanvasRect.W) / a.Cam.Zoom
				vh := float64(a.CanvasRect.H) / a.Cam.Zoom
				vr := ui.Rect{
					X: float32(ox + vx*s),
					Y: float32(oy + vy*s),
					W: float32(vw * s),
					H: float32(vh * s),
				}
				p.Stroke(vr, vercelBlue, 4, 1.4)
			}
		})
	})
}

func hitCornerHandle(n *graph.Node, r ui.Rect, cam render.Camera, sx, sy float32) int {
	z := float32(cam.Zoom)
	x0, y0 := cam.WorldToScreen(r, geo.V(n.X, n.Y))
	w, h := float32(n.W)*z, float32(n.H)*z
	corners := [4][2]float32{
		{x0, y0},
		{x0 + w, y0},
		{x0 + w, y0 + h},
		{x0, y0 + h},
	}
	for idx, pt := range corners {
		if math.Abs(float64(sx-pt[0])) <= 7 && math.Abs(float64(sy-pt[1])) <= 7 {
			return idx
		}
	}
	return -1
}

func hitSidePort(n *graph.Node, r ui.Rect, cam render.Camera, sx, sy float32) bool {
	z := float32(cam.Zoom)
	x0, y0 := cam.WorldToScreen(r, geo.V(n.X, n.Y))
	w, h := float32(n.W)*z, float32(n.H)*z
	ports := [4][2]float32{
		{x0 + w/2, y0},
		{x0 + w, y0 + h/2},
		{x0 + w/2, y0 + h},
		{x0, y0 + h/2},
	}
	for _, pt := range ports {
		if math.Hypot(float64(sx-pt[0]), float64(sy-pt[1])) <= 8 {
			return true
		}
	}
	return false
}

var _ = fmt.Sprint
