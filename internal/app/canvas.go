package app

import (
	"fmt"
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/lobehub/lobe-mygo/pkg/field"
	"github.com/lobehub/lobe-mygo/pkg/theme"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/render"
)

func (a *App) canvasArea(c *ui.Context, tk *theme.Tokens) {
	wrap := ui.Box(c.Key("canvas-wrap")).Grow(1).Basis(0).MinHeight(0)
	dragOver := wrap.FileDragOver()
	dropped := wrap.DroppedFiles()
	wrap.Children(func() {
		cv := ui.Box(c.Key("canvas")).Fill().
			Focusable().FocusRing(false).
			Bind(&a.CanvasHandle).
			Cursor(a.canvasCursor())
		cv.Label("Canvas")
		cv.Draw(func(p *ui.Painter, r ui.Rect) {
			var preview *[2]geo.Vec
			if a.drag == dragConnect && a.connectSrcID != "" {
				if src := a.Page().NodeByID(a.connectSrcID); src != nil {
					p0, _ := geo.AnchorPoint(src.Box(), geo.AnchorAuto, a.connectEndPt)
					pv := [2]geo.Vec{p0, a.connectEndPt}
					preview = &pv
				}
			}
			render.PaintScene(p, r, render.SceneState{
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
			})
		})
		cv.HandleInput(func(ev ui.InputEvent) bool {
			return a.handleCanvasInput(c, ev)
		})

		a.videoOverlayLayer(c, tk)

		if dragOver {
			ui.Box(c.Key("drop-overlay")).Absolute().Top(12).Left(12).Right(12).Bottom(12).Radius(12).
				Background(tk.Primary.Alpha(0.10)).BorderWidth(2).BorderColor(tk.Primary).Center().Children(func() {
				ui.Box(c).Padding(12, 20).Radius(10).Background(tk.BgElevated).BorderWidth(1).BorderColor(tk.Primary).Children(func() {
					ui.Text(c, "Drop images or videos to create high-resolution nodes").FontSize(14).FontWeight(600).TextColor(tk.Primary)
				})
			})
		}

		a.inlineTextEditor(c, tk)
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
		if ev.Mods&ui.Cmd != 0 || ev.Mods&ui.Ctrl != 0 {
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

		// Double-click opens inline text editor.
		if ev.Clicks >= 2 {
			if hit := a.Page().HitNode(worldPt); hit != nil {
				a.SelectedNodes = map[string]bool{hit.ID: true}
				a.SelectedEdges = map[string]bool{}
				a.EditingNodeID = hit.ID
				a.EditingText = hit.Label
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
			for id := range a.SelectedNodes {
				if n := a.Page().NodeByID(id); n != nil {
					a.dragNodeOrig[id] = n.Box()
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
			n.X = nx
			n.Y = ny
		}
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
				for _, n := range a.Page().Nodes {
					if norm.Overlaps(n.Box()) {
						a.SelectedNodes[n.ID] = true
					}
				}
			}
		}
		a.marqueeBox = nil
	}
	a.drag = dragNone
	a.dragNodeOrig = nil
}

func (a *App) inlineTextEditor(c *ui.Context, tk *theme.Tokens) {
	if a.EditingNodeID == "" || a.CanvasRect.W <= 0 {
		return
	}
	n := a.Page().NodeByID(a.EditingNodeID)
	if n == nil {
		a.EditingNodeID = ""
		return
	}
	localRect := ui.Rect{X: 0, Y: 0, W: a.CanvasRect.W, H: a.CanvasRect.H}
	sx, sy := a.Cam.WorldToScreen(localRect, geo.V(n.X, n.Y))
	sw := float32(n.W * a.Cam.Zoom)
	if sw < 140 {
		sw = 140
	}
	box := ui.Box(c.Key("inline-edit")).Absolute().
		Left(sx).Top(sy + float32(n.H*a.Cam.Zoom)/2 - 18).
		Width(sw).Padding(4).Radius(8).
		Background(tk.BgElevated).
		BorderWidth(1.5).BorderColor(tk.Primary)
	box.Children(func() {
		inp := field.Input(c, &a.EditingText, field.InputProps{
			Variant:     field.InputBorderless,
			Size:        theme.Small,
			Placeholder: "Node label...",
		})
		inp.FillWidth()
		inp.OnTextKey(func(k ui.TextKey) bool {
			if k.Key == ui.KeyEnter && k.Mods == 0 {
				a.commitInlineEdit()
				return true
			}
			if k.Key == ui.KeyEscape {
				a.EditingNodeID = ""
				return true
			}
			return false
		})
	})
}

func (a *App) commitInlineEdit() {
	if n := a.Page().NodeByID(a.EditingNodeID); n != nil {
		a.Hist.Snapshot()
		n.Label = a.EditingText
		a.notifyChanged()
	}
	a.EditingNodeID = ""
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
		Width(164).Height(104).Radius(8).
		Background(tk.BgElevated.Alpha(0.92)).
		BorderWidth(1).BorderColor(tk.BorderSecondary)
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
				col = tk.Primary
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
			p.Stroke(vr, tk.Primary, 2, 1.2)
		}
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
