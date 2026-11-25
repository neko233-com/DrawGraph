// Package render paints DrawGraph diagrams onto a MyGo GPU ui.Painter.
// The same painter drives both the interactive desktop canvas and offscreen PNG export.
package render

import (
	"fmt"
	"math"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
)

// Camera maps world coordinates to DIP coordinates inside the canvas rectangle.
type Camera struct {
	X    float64 // pan offset in world units
	Y    float64
	Zoom float64
}

// WorldToScreen converts a world coordinate to painter DIPs relative to rect.
func (c Camera) WorldToScreen(r ui.Rect, p geo.Vec) (float32, float32) {
	z := c.Zoom
	if z <= 0 {
		z = 1
	}
	return r.X + float32((p.X+c.X)*z), r.Y + float32((p.Y+c.Y)*z)
}

// ScreenToWorld converts painter/window DIPs relative to rect into world coordinates.
func (c Camera) ScreenToWorld(r ui.Rect, sx, sy float32) geo.Vec {
	z := c.Zoom
	if z <= 0 {
		z = 1
	}
	return geo.Vec{
		X: float64(sx-r.X)/z - c.X,
		Y: float64(sy-r.Y)/z - c.Y,
	}
}

// SceneState supplies interactive overlays (selection, hover, live connector drag) when painting.
type SceneState struct {
	Doc            *graph.Document
	Page           *graph.Page
	Icons          *icons.Registry
	ResolveMedia   func(assetID string) ui.ImageSource
	ActiveVideos   map[string]bool // video node IDs currently rendered by a live ui.Video widget
	Cam            Camera
	Dark           bool
	ShowGrid       bool
	Export         bool
	SelectedNodes  map[string]bool
	SelectedEdges  map[string]bool
	HoveredNodeID  string
	ConnectPreview *[2]geo.Vec
	MarqueeBox     *geo.Box
	GuideLines     []graph.SnapGuide
}

// PaintStats reports virtualized canvas culling and LOD metrics for telemetry HUD & tests.
type PaintStats struct {
	TotalNodes   int
	VisibleNodes int
	TotalEdges   int
	VisibleEdges int
	LODMode      string
}

// ShapePaintContext passes resolved colors and geometry to custom registered shape plugins.
type ShapePaintContext struct {
	Painter     *ui.Painter
	Box         ui.Rect
	Node        *graph.Node
	Fill        ui.Color
	Stroke      ui.Color
	StrokeWidth float32
	Radius      float32
	Zoom        float32
	Dark        bool
}

// ShapeRenderer paints a custom NodeKind onto the GPU canvas.
type ShapeRenderer func(ctx ShapePaintContext)

var customShapes = map[graph.NodeKind]ShapeRenderer{}

// RegisterShape registers an extensible custom node shape renderer.
func RegisterShape(kind graph.NodeKind, fn ShapeRenderer) {
	if kind == "" || fn == nil {
		return
	}
	customShapes[kind] = fn
}

// RegisteredShapes returns all registered custom shape kinds.
func RegisteredShapes() []graph.NodeKind {
	out := make([]graph.NodeKind, 0, len(customShapes))
	for k := range customShapes {
		out = append(out, k)
	}
	return out
}

// Palette returns theme-aware canvas defaults.
type Palette struct {
	CanvasBG      ui.Color
	GridDot       ui.Color
	GridMajor     ui.Color
	NodeFill      ui.Color
	NodeStroke    ui.Color
	NodeText      ui.Color
	ContainerFill ui.Color
	EdgeStroke    ui.Color
	EdgeLabelBG   ui.Color
	EdgeLabelText ui.Color
	Accent        ui.Color
	Shadow        ui.Color
}

// PaletteFor returns light or dark canvas colors with Vercel Geist contrast & colorful accents.
func PaletteFor(dark bool) Palette {
	if dark {
		return Palette{
			CanvasBG:      ui.Hex("#07080D"),
			GridDot:       ui.RGBA(148, 163, 184, 0.14),
			GridMajor:     ui.RGBA(56, 189, 248, 0.24),
			NodeFill:      ui.Hex("#11131C"),
			NodeStroke:    ui.Hex("#3B82F6"),
			NodeText:      ui.Hex("#FAFAFA"),
			ContainerFill: ui.RGBA(30, 41, 59, 0.42),
			EdgeStroke:    ui.Hex("#38BDF8"),
			EdgeLabelBG:   ui.Hex("#0D1117"),
			EdgeLabelText: ui.Hex("#F4F4F5"),
			Accent:        ui.Hex("#0070F3"),
			Shadow:        ui.RGBA(0, 0, 0, 0.48),
		}
	}
	return Palette{
		CanvasBG:      ui.Hex("#FCFCFD"),
		GridDot:       ui.RGBA(100, 116, 139, 0.16),
		GridMajor:     ui.RGBA(0, 112, 243, 0.20),
		NodeFill:      ui.Hex("#FFFFFF"),
		NodeStroke:    ui.Hex("#CBD5E1"),
		NodeText:      ui.Hex("#09090B"),
		ContainerFill: ui.RGBA(239, 246, 255, 0.55),
		EdgeStroke:    ui.Hex("#0070F3"),
		EdgeLabelBG:   ui.Hex("#FFFFFF"),
		EdgeLabelText: ui.Hex("#09090B"),
		Accent:        ui.Hex("#0070F3"),
		Shadow:        ui.RGBA(0, 112, 243, 0.10),
	}
}

// PaintScene renders the diagram and interactive editing overlays into rect r.
func PaintScene(p *ui.Painter, r ui.Rect, st SceneState) {
	_ = PaintSceneWithStats(p, r, st)
}

// PaintSceneWithStats renders the visible viewport using spatial frustum culling and GPU LOD,
// returning live virtualization telemetry.
func PaintSceneWithStats(p *ui.Painter, r ui.Rect, st SceneState) PaintStats {
	var stats PaintStats
	if st.Page == nil {
		return stats
	}
	if st.Cam.Zoom <= 0 {
		st.Cam.Zoom = 1
	}
	pal := PaletteFor(st.Dark)
	bg := pal.CanvasBG
	if st.Page.Background != "" {
		if c, ok := parseColor(st.Page.Background); ok {
			bg = c
		}
	}
	p.Fill(r, bg, 0)

	margin := 80.0 / math.Max(st.Cam.Zoom, 0.05)
	viewWorld := geo.Box{
		X: -st.Cam.X - margin,
		Y: -st.Cam.Y - margin,
		W: float64(r.W)/st.Cam.Zoom + margin*2,
		H: float64(r.H)/st.Cam.Zoom + margin*2,
	}

	idx := st.Page.EnsureIndex()
	visibleNodes := idx.QueryRect(viewWorld)
	visibleEdges := idx.QueryEdges(st.Page, viewWorld)

	stats.TotalNodes = len(st.Page.Nodes)
	stats.VisibleNodes = len(visibleNodes)
	stats.TotalEdges = len(st.Page.Edges)
	stats.VisibleEdges = len(visibleEdges)
	switch {
	case st.Cam.Zoom < 0.08:
		stats.LODMode = "micro"
	case st.Cam.Zoom < 0.28:
		stats.LODMode = "low"
	default:
		stats.LODMode = "high"
	}

	p.Clip(r, 0, func() {
		if st.ShowGrid && !st.Export {
			paintGrid(p, r, st, pal)
		}

		// Render containers and swimlanes first, then visible edges, then regular visible nodes.
		for _, n := range visibleNodes {
			if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
				paintNode(p, r, st, pal, n)
			}
		}
		for _, ve := range visibleEdges {
			paintVisibleEdge(p, r, st, pal, ve)
		}
		for _, n := range visibleNodes {
			if n.Kind != graph.KindContainer && n.Kind != graph.KindSwimlane {
				paintNode(p, r, st, pal, n)
			}
		}

		if !st.Export {
			paintOverlays(p, r, st, pal, idx)
		}
	})
	return stats
}

func paintGrid(p *ui.Painter, r ui.Rect, st SceneState, pal Palette) {
	step := 20.0
	if st.Doc != nil && st.Doc.GridSize > 0 {
		step = st.Doc.GridSize
	}
	for step*st.Cam.Zoom < 14 {
		step *= 5
	}
	for step*st.Cam.Zoom > 90 && step >= 4 {
		step /= 2
	}
	major := step * 5
	view := geo.Box{
		X: -st.Cam.X,
		Y: -st.Cam.Y,
		W: float64(r.W) / st.Cam.Zoom,
		H: float64(r.H) / st.Cam.Zoom,
	}
	cols := int(view.W/step) + 1
	rows := int(view.H/step) + 1
	// Switch to major grid lines when zoom-out would otherwise emit >900 minor dot calls.
	if cols*rows > 900 || st.Cam.Zoom < 0.4 {
		mx0 := math.Floor(view.X/major) * major
		my0 := math.Floor(view.Y/major) * major
		lineCol := pal.GridDot.Alpha(0.7)
		for x := mx0; x <= view.MaxX(); x += major {
			sx := r.X + float32((x+st.Cam.X)*st.Cam.Zoom)
			p.Line(sx, r.Y, sx, r.Y+r.H, 1, lineCol)
		}
		for y := my0; y <= view.MaxY(); y += major {
			sy := r.Y + float32((y+st.Cam.Y)*st.Cam.Zoom)
			p.Line(r.X, sy, r.X+r.W, sy, 1, lineCol)
		}
	} else {
		x0 := math.Floor(view.X/step) * step
		y0 := math.Floor(view.Y/step) * step
		for y := y0; y <= view.MaxY(); y += step {
			sy := r.Y + float32((y+st.Cam.Y)*st.Cam.Zoom)
			isMajorY := math.Abs(math.Mod(math.Abs(y), major)) < 0.01
			for x := x0; x <= view.MaxX(); x += step {
				sx := r.X + float32((x+st.Cam.X)*st.Cam.Zoom)
				isMajor := isMajorY && math.Abs(math.Mod(math.Abs(x), major)) < 0.01
				col := pal.GridDot
				sz := float32(1.5)
				if isMajor {
					col = pal.GridMajor
					sz = 2.5
				}
				p.Fill(ui.Rect{X: sx - sz/2, Y: sy - sz/2, W: sz, H: sz}, col, sz/2)
			}
		}
	}
	// Draw colorful Vercel origin (0,0) crosshair when visible inside viewport.
	ox, oy := st.Cam.WorldToScreen(r, geo.V(0, 0))
	if ox >= r.X-20 && ox <= r.X+r.W+20 && oy >= r.Y-20 && oy <= r.Y+r.H+20 {
		p.Line(ox-14, oy, ox+14, oy, 1.4, ui.Hex("#FF0080").Alpha(0.45))
		p.Line(ox, oy-14, ox, oy+14, 1.4, ui.Hex("#0070F3").Alpha(0.45))
	}
}

func paintNode(p *ui.Painter, r ui.Rect, st SceneState, pal Palette, n *graph.Node) {
	z := float32(st.Cam.Zoom)
	sx, sy := st.Cam.WorldToScreen(r, geo.V(n.X, n.Y))
	sw, sh := float32(n.W)*z, float32(n.H)*z
	box := ui.Rect{X: sx, Y: sy, W: sw, H: sh}

	if box.X+box.W < r.X-60 || box.Y+box.H < r.Y-60 || box.X > r.X+r.W+60 || box.Y > r.Y+r.H+60 {
		return
	}

	op := float32(n.Style.Opacity)
	if op <= 0 || op > 1 {
		op = 1
	}

	fill := pal.NodeFill
	if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
		fill = pal.ContainerFill
	}
	if n.Style.Fill != "" {
		if c, ok := parseColor(n.Style.Fill); ok {
			fill = adjustFillForDark(c, n.Style.Fill, st.Dark)
		}
	}
	fill = fill.Alpha(float32(fill.A) / 255.0 * op)

	stroke := pal.NodeStroke
	if n.Style.Stroke != "" {
		if c, ok := parseColor(n.Style.Stroke); ok {
			stroke = c
		}
	}
	stroke = stroke.Alpha(float32(stroke.A) / 255.0 * op)

	// GPU Virtual Canvas LOD: bypass heavy path/text/shadow calls when nodes are tiny on screen.
	if !st.Export {
		if sw < 4 || sh < 4 || z < 0.08 {
			c := fill
			if c.A < 25 {
				c = stroke
			}
			p.Fill(ui.Rect{X: box.X, Y: box.Y, W: max(float32(2), box.W), H: max(float32(2), box.H)}, c, 0)
			return
		}
		if sw < 36 || sh < 20 || z < 0.28 {
			rad := min(float32(n.Style.Radius)*z, min(box.W, box.H)*0.25)
			p.Fill(box, fill, rad)
			if n.Kind != graph.KindText {
				p.Stroke(box, stroke, rad, 1)
			}
			return
		}
	}

	textCol := pal.NodeText
	if n.Style.TextColor != "" {
		if c, ok := parseColor(n.Style.TextColor); ok {
			textCol = adjustTextForDark(c, n.Style.TextColor, st.Dark)
		}
	}
	textCol = textCol.Alpha(op)
	swpx := float32(n.Style.StrokeWidth) * z
	if swpx < 1 {
		swpx = 1
	}

	rad := float32(n.Style.Radius) * z
	if n.Style.Shadow && n.Kind != graph.KindText && n.Kind != graph.KindContainer && n.Kind != graph.KindSwimlane {
		p.Shadow(box, rad, 0, 3*z, 10*z, 0, pal.Shadow)
	}

	switch n.Kind {
	case graph.KindText:
		// Transparent text box; optional dashed selection outline handled in overlays.
	case graph.KindRounded, graph.KindRect, graph.KindContainer, graph.KindIcon:
		if n.Kind == graph.KindRect {
			rad = 2 * z
		}
		if n.Kind != graph.KindIcon || n.Style.Fill != "none" {
			p.Fill(box, fill, rad)
			strokeRect(p, box, stroke, rad, swpx, n.Style.Dash)
		}
	case graph.KindSwimlane:
		p.Fill(box, fill, rad)
		hdrH := float32(28) * z
		if hdrH > box.H {
			hdrH = box.H
		}
		hdr := ui.Rect{X: box.X, Y: box.Y, W: box.W, H: hdrH}
		p.Fill(hdr, stroke.Alpha(0.18), rad)
		p.Line(box.X, box.Y+hdrH, box.X+box.W, box.Y+hdrH, swpx, stroke)
		strokeRect(p, box, stroke, rad, swpx, n.Style.Dash)
	case graph.KindEllipse:
		path := &ui.Path{}
		ellipsePath(path, box)
		p.FillPath(path, fill)
		applyDash(path, n.Style.Dash, swpx)
		p.StrokePath(path, swpx, stroke)
	case graph.KindDiamond:
		path := &ui.Path{}
		cx, cy := box.X+box.W/2, box.Y+box.H/2
		path.MoveTo(cx, box.Y).LineTo(box.X+box.W, cy).LineTo(cx, box.Y+box.H).LineTo(box.X, cy).Close()
		p.FillPath(path, fill)
		applyDash(path, n.Style.Dash, swpx)
		p.StrokePath(path, swpx, stroke)
	case graph.KindHexagon:
		path := &ui.Path{}
		inset := min(box.W*0.2, 22*z)
		cy := box.Y + box.H/2
		path.MoveTo(box.X+inset, box.Y).
			LineTo(box.X+box.W-inset, box.Y).
			LineTo(box.X+box.W, cy).
			LineTo(box.X+box.W-inset, box.Y+box.H).
			LineTo(box.X+inset, box.Y+box.H).
			LineTo(box.X, cy).Close()
		p.FillPath(path, fill)
		applyDash(path, n.Style.Dash, swpx)
		p.StrokePath(path, swpx, stroke)
	case graph.KindParallelogram:
		path := &ui.Path{}
		skew := min(box.W*0.18, 22*z)
		path.MoveTo(box.X+skew, box.Y).
			LineTo(box.X+box.W, box.Y).
			LineTo(box.X+box.W-skew, box.Y+box.H).
			LineTo(box.X, box.Y+box.H).Close()
		p.FillPath(path, fill)
		applyDash(path, n.Style.Dash, swpx)
		p.StrokePath(path, swpx, stroke)
	case graph.KindCylinder:
		paintCylinder(p, box, fill, stroke, swpx, z)
	case graph.KindCloud:
		paintCloud(p, box, fill, stroke, swpx)
	case graph.KindDocument:
		paintDocument(p, box, fill, stroke, swpx, z)
	case graph.KindNote:
		paintNote(p, box, fill, stroke, swpx, z)
	case graph.KindActor:
		paintActor(p, box, stroke, swpx, z)
	case graph.KindImage:
		paintImageNode(p, box, st, pal, n, fill, stroke, rad, swpx, z)
		return
	case graph.KindVideo:
		paintVideoCard(p, box, st, pal, n, stroke, rad, swpx, z)
		return
	default:
		if customFn, ok := customShapes[n.Kind]; ok {
			customFn(ShapePaintContext{
				Painter:     p,
				Box:         box,
				Node:        n,
				Fill:        fill,
				Stroke:      stroke,
				StrokeWidth: swpx,
				Radius:      rad,
				Zoom:        z,
				Dark:        st.Dark,
			})
		} else {
			p.Fill(box, fill, rad)
			strokeRect(p, box, stroke, rad, swpx, n.Style.Dash)
		}
	}

	// Render optional SVG icon from built-in or Git-installed icon pack.
	textArea := box
	if n.Icon.Valid() && st.Icons != nil {
		if svg, ok := st.Icons.ParsedSVG(n.Icon.String()); ok {
			tint := textCol
			if n.Icon.Tint != "" {
				if tc, ok := parseColor(n.Icon.Tint); ok {
					tint = tc
				}
			}
			iconSz := float32(n.Icon.Size)
			if iconSz <= 0 {
				if n.Kind == graph.KindIcon {
					iconSz = min(float32(n.W), float32(n.H)) * 0.52
				} else {
					iconSz = 22
				}
			}
			iconSz *= z
			pos := n.Icon.Position
			if pos == "" {
				if n.Kind == graph.KindIcon {
					pos = "center"
				} else {
					pos = "left"
				}
			}
			var ir ui.Rect
			switch pos {
			case "top", "center":
				ir = ui.Rect{X: box.X + (box.W-iconSz)/2, Y: box.Y + 10*z, W: iconSz, H: iconSz}
				if strings.TrimSpace(n.Label) == "" && pos == "center" {
					ir.Y = box.Y + (box.H-iconSz)/2
				}
				textArea = ui.Rect{X: box.X + 6*z, Y: ir.Y + iconSz + 2*z, W: box.W - 12*z, H: box.H - (iconSz + 14*z)}
			case "badge":
				ir = ui.Rect{X: box.X + box.W - iconSz - 8*z, Y: box.Y + 8*z, W: iconSz, H: iconSz}
			default: // "left"
				ir = ui.Rect{X: box.X + 12*z, Y: box.Y + (box.H-iconSz)/2, W: iconSz, H: iconSz}
				textArea = ui.Rect{X: ir.X + iconSz + 8*z, Y: box.Y + 4*z, W: box.W - (iconSz + 26*z), H: box.H - 8*z}
			}
			p.Icon(svg, ir, tint)
		}
	}

	// Render Label & Sublabel cleanly centered inside textArea.
	paintNodeText(p, box, textArea, n, textCol, z)
}

func paintNodeText(p *ui.Painter, box, textArea ui.Rect, n *graph.Node, col ui.Color, z float32) {
	label := strings.TrimSpace(n.Label)
	sub := strings.TrimSpace(n.Sublabel)
	if label == "" && sub == "" {
		return
	}
	fontSize := float32(n.Style.FontSize) * z
	if fontSize < 7 {
		fontSize = 7
	}
	if n.Kind == graph.KindSwimlane || n.Kind == graph.KindContainer {
		if label != "" {
			p.Text(box.X+10*z, box.Y+6*z, label, fontSize, col)
		}
		return
	}
	if n.Kind == graph.KindActor {
		if label != "" {
			w, _ := p.MeasureText(box.W, ui.Span{Text: label, Size: fontSize})
			p.Text(box.X+(box.W-w)/2, box.Y+box.H-fontSize-4*z, label, fontSize, col)
		}
		return
	}

	maxW := max(textArea.W-8*z, 24*z)
	spans := []ui.Span{}
	if label != "" {
		spans = append(spans, ui.Span{Text: label, Size: fontSize, Color: col})
	}
	if sub != "" {
		if label != "" {
			spans = append(spans, ui.Span{Text: "\n" + sub, Size: max(8, fontSize*0.82), Color: col.Alpha(0.72)})
		} else {
			spans = append(spans, ui.Span{Text: sub, Size: max(8, fontSize*0.82), Color: col.Alpha(0.72)})
		}
	}
	tw, th := p.MeasureText(maxW, spans...)
	tx := textArea.X + (textArea.W-tw)/2
	if n.Style.Align == "left" {
		tx = textArea.X + 8*z
	} else if n.Style.Align == "right" {
		tx = textArea.X + textArea.W - tw - 8*z
	}
	ty := textArea.Y + (textArea.H-th)/2
	if n.Style.VAlign == "top" {
		ty = textArea.Y + 6*z
	}
	p.RichText(tx, ty, maxW, spans...)
}

func paintVisibleEdge(p *ui.Painter, r ui.Rect, st SceneState, pal Palette, ve graph.VisibleEdge) {
	e := ve.Edge
	pts := ve.Points
	if e == nil || len(pts) < 2 {
		return
	}
	// Separate parallel connectors between the same endpoints cleanly along perpendicular offset.
	if ve.PairTotal > 1 && len(pts) == 2 {
		dx := pts[1].X - pts[0].X
		dy := pts[1].Y - pts[0].Y
		if l := math.Hypot(dx, dy); l > 1e-3 {
			slotOff := (float64(ve.PairSlot) - float64(ve.PairTotal-1)/2.0) * 14.0
			if e.SourceID > e.TargetID {
				slotOff = -slotOff
			}
			nx, ny := -dy/l*slotOff, dx/l*slotOff
			pts = []geo.Vec{
				{X: pts[0].X + nx*0.5, Y: pts[0].Y + ny*0.5},
				{X: (pts[0].X+pts[1].X)/2 + nx*1.4, Y: (pts[0].Y+pts[1].Y)/2 + ny*1.4},
				{X: pts[1].X + nx*0.5, Y: pts[1].Y + ny*0.5},
			}
		}
	}

	z := float32(st.Cam.Zoom)
	screen := make([][2]float32, len(pts))
	for i, pt := range pts {
		sx, sy := st.Cam.WorldToScreen(r, pt)
		screen[i] = [2]float32{sx, sy}
	}

	col := pal.EdgeStroke
	if e.Style.Stroke != "" {
		if c, ok := parseColor(e.Style.Stroke); ok {
			col = c
		}
	}
	if st.SelectedEdges[e.ID] && !st.Export {
		col = pal.Accent
	}
	sw := float32(e.Style.StrokeWidth) * z
	if sw < 1 {
		sw = 1
	}
	if st.SelectedEdges[e.ID] && !st.Export {
		sw += 0.75 * z
	}

	// Fast LOD when zoomed far out: hairline polyline only.
	if !st.Export && z < 0.22 {
		path := &ui.Path{}
		path.MoveTo(screen[0][0], screen[0][1])
		for _, pt := range screen[1:] {
			path.LineTo(pt[0], pt[1])
		}
		p.StrokePath(path, max(1, sw), col)
		return
	}

	path := &ui.Path{}
	if e.Routing == graph.RouteCurved && len(screen) == 2 {
		a, b := screen[0], screen[1]
		dx := (b[0] - a[0]) * 0.5
		dy := (b[1] - a[1]) * 0.5
		path.MoveTo(a[0], a[1])
		if math.Abs(float64(dx)) >= math.Abs(float64(dy)) {
			path.CubeTo(a[0]+dx, a[1], b[0]-dx, b[1], b[0], b[1])
		} else {
			path.CubeTo(a[0], a[1]+dy, b[0], b[1]-dy, b[0], b[1])
		}
		applyDash(path, e.Style.Dash, sw)
		p.StrokePath(path, sw, col)
	} else if e.Routing == graph.RouteCurved && len(screen) == 3 {
		path.MoveTo(screen[0][0], screen[0][1])
		path.QuadTo(screen[1][0], screen[1][1], screen[2][0], screen[2][1])
		applyDash(path, e.Style.Dash, sw)
		p.StrokePath(path, sw, col)
	} else {
		path.MoveTo(screen[0][0], screen[0][1])
		for _, pt := range screen[1:] {
			path.LineTo(pt[0], pt[1])
		}
		applyDash(path, e.Style.Dash, sw)
		p.StrokePath(path, sw, col)
	}

	if e.EndArrow != "" && e.EndArrow != graph.ArrowNone {
		n := len(screen)
		paintArrowhead(p, screen[n-2], screen[n-1], e.EndArrow, col, sw, z)
	}
	if e.StartArrow != "" && e.StartArrow != graph.ArrowNone {
		paintArrowhead(p, screen[1], screen[0], e.StartArrow, col, sw, z)
	}

	if strings.TrimSpace(e.Label) != "" && (st.Export || z >= 0.32) {
		midWorld := geo.PolylineMidpoint(pts)
		mx, my := st.Cam.WorldToScreen(r, midWorld)
		fs := float32(e.Style.FontSize) * z
		if fs < 8 {
			fs = 8
		}
		tw, th := p.MeasureText(240*z, ui.Span{Text: e.Label, Size: fs})
		pill := ui.Rect{
			X: mx - tw/2 - 6*z,
			Y: my - th/2 - 3*z,
			W: tw + 12*z,
			H: th + 6*z,
		}
		p.Fill(pill, pal.EdgeLabelBG, 5*z)
		p.Stroke(pill, col.Alpha(0.35), 5*z, 1)
		p.Text(pill.X+6*z, pill.Y+3*z, e.Label, fs, pal.EdgeLabelText)
	}
}

func paintArrowhead(p *ui.Painter, from, to [2]float32, kind graph.ArrowStyle, col ui.Color, sw, z float32) {
	dx := float64(to[0] - from[0])
	dy := float64(to[1] - from[1])
	l := math.Hypot(dx, dy)
	if l < 1e-3 {
		return
	}
	ux, uy := float32(dx/l), float32(dy/l)
	px, py := -uy, ux
	sz := 9 * z
	w := 4.5 * z

	switch kind {
	case graph.ArrowCircle:
		path := &ui.Path{}
		path.Circle(to[0]-ux*4*z, to[1]-uy*4*z, 4*z)
		p.FillPath(path, col)
	case graph.ArrowDiamond:
		path := &ui.Path{}
		tipX, tipY := to[0], to[1]
		midX, midY := to[0]-ux*sz*0.7, to[1]-uy*sz*0.7
		backX, backY := to[0]-ux*sz*1.4, to[1]-uy*sz*1.4
		path.MoveTo(tipX, tipY).LineTo(midX+px*w, midY+py*w).LineTo(backX, backY).LineTo(midX-px*w, midY-py*w).Close()
		p.FillPath(path, col)
	case graph.ArrowOpen:
		path := &ui.Path{}
		bx, by := to[0]-ux*sz, to[1]-uy*sz
		path.MoveTo(bx+px*w, by+py*w).LineTo(to[0], to[1]).LineTo(bx-px*w, by-py*w)
		p.StrokePath(path, sw, col)
	default: // ArrowClassic
		path := &ui.Path{}
		bx, by := to[0]-ux*sz, to[1]-uy*sz
		path.MoveTo(to[0], to[1]).LineTo(bx+px*w, by+py*w).LineTo(bx-px*w, by-py*w).Close()
		p.FillPath(path, col)
	}
}

func paintOverlays(p *ui.Painter, r ui.Rect, st SceneState, pal Palette, idx *graph.SpatialIndex) {
	z := float32(st.Cam.Zoom)
	// Smart alignment guide lines while dragging nodes
	if len(st.GuideLines) > 0 {
		guideCol := ui.Hex("#EF4444").Alpha(0.85)
		for _, g := range st.GuideLines {
			if g.Axis == "x" {
				ax, ay := st.Cam.WorldToScreen(r, geo.V(g.Pos, g.Start))
				bx, by := st.Cam.WorldToScreen(r, geo.V(g.Pos, g.End))
				p.Line(ax, ay, bx, by, 1.2, guideCol)
			} else {
				ax, ay := st.Cam.WorldToScreen(r, geo.V(g.Start, g.Pos))
				bx, by := st.Cam.WorldToScreen(r, geo.V(g.End, g.Pos))
				p.Line(ax, ay, bx, by, 1.2, guideCol)
			}
		}
	}

	paintNodeOverlay := func(n *graph.Node, selected bool) {
		sx, sy := st.Cam.WorldToScreen(r, geo.V(n.X, n.Y))
		box := ui.Rect{X: sx, Y: sy, W: float32(n.W) * z, H: float32(n.H) * z}
		if selected {
			p.Stroke(ExpandRect(box, 2), pal.Accent, 4, 1.8)
			hs := float32(7)
			corners := [4][2]float32{
				{box.X, box.Y},
				{box.X + box.W, box.Y},
				{box.X + box.W, box.Y + box.H},
				{box.X, box.Y + box.H},
			}
			for _, c := range corners {
				hr := ui.Rect{X: c[0] - hs/2, Y: c[1] - hs/2, W: hs, H: hs}
				p.Fill(hr, ui.Hex("#FFFFFF"), 2)
				p.Stroke(hr, pal.Accent, 2, 1.5)
			}
		} else {
			p.Stroke(ExpandRect(box, 1.5), pal.Accent.Alpha(0.55), 4, 1.2)
		}
		ports := [4][2]float32{
			{box.X + box.W/2, box.Y},
			{box.X + box.W, box.Y + box.H/2},
			{box.X + box.W/2, box.Y + box.H},
			{box.X, box.Y + box.H/2},
		}
		for _, pt := range ports {
			pr := ui.Rect{X: pt[0] - 3.5, Y: pt[1] - 3.5, W: 7, H: 7}
			p.Fill(pr, pal.Accent.Alpha(0.85), 3.5)
		}
	}

	if st.HoveredNodeID != "" && !st.SelectedNodes[st.HoveredNodeID] {
		if hn := idx.NodeByIDFast(st.HoveredNodeID); hn != nil {
			paintNodeOverlay(hn, false)
		}
	}
	for id := range st.SelectedNodes {
		if sn := idx.NodeByIDFast(id); sn != nil {
			paintNodeOverlay(sn, true)
		}
	}

	if st.ConnectPreview != nil {
		ax, ay := st.Cam.WorldToScreen(r, st.ConnectPreview[0])
		bx, by := st.Cam.WorldToScreen(r, st.ConnectPreview[1])
		p.Line(ax, ay, bx, by, 2, pal.Accent)
		paintArrowhead(p, [2]float32{ax, ay}, [2]float32{bx, by}, graph.ArrowClassic, pal.Accent, 2, z)
	}

	if st.MarqueeBox != nil {
		b := st.MarqueeBox.Normalized()
		sx, sy := st.Cam.WorldToScreen(r, geo.V(b.X, b.Y))
		mr := ui.Rect{X: sx, Y: sy, W: float32(b.W) * z, H: float32(b.H) * z}
		p.Fill(mr, pal.Accent.Alpha(0.12), 2)
		p.Stroke(mr, pal.Accent, 2, 1.2)
	}
}

func strokeRect(p *ui.Painter, box ui.Rect, stroke ui.Color, rad, sw float32, dash graph.DashStyle) {
	if dash == graph.DashDashed || dash == graph.DashDotted {
		p.StrokeDashed(box, stroke, rad, sw)
		return
	}
	p.Stroke(box, stroke, rad, sw)
}

func applyDash(path *ui.Path, dash graph.DashStyle, sw float32) {
	switch dash {
	case graph.DashDashed:
		path.Dash(0, sw*4, sw*3)
	case graph.DashDotted:
		path.Dash(0, sw*1.2, sw*2.4)
	}
}

func ellipsePath(path *ui.Path, box ui.Rect) {
	const k = float32(0.55228475)
	rx, ry := box.W/2, box.H/2
	cx, cy := box.X+rx, box.Y+ry
	ox, oy := rx*k, ry*k
	path.MoveTo(cx, box.Y).
		CubeTo(cx+ox, box.Y, cx+rx, cy-oy, cx+rx, cy).
		CubeTo(cx+rx, cy+oy, cx+ox, box.Y+box.H, cx, box.Y+box.H).
		CubeTo(cx-ox, box.Y+box.H, cx-rx, cy+oy, cx-rx, cy).
		CubeTo(cx-rx, cy-oy, cx-ox, box.Y, cx, box.Y).
		Close()
}

func paintCylinder(p *ui.Painter, box ui.Rect, fill, stroke ui.Color, sw, z float32) {
	capH := min(box.H*0.24, 16*z)
	body := &ui.Path{}
	body.MoveTo(box.X, box.Y+capH/2).
		LineTo(box.X, box.Y+box.H-capH/2).
		QuadTo(box.X+box.W/2, box.Y+box.H+capH*0.4, box.X+box.W, box.Y+box.H-capH/2).
		LineTo(box.X+box.W, box.Y+capH/2).
		QuadTo(box.X+box.W/2, box.Y-capH*0.4, box.X, box.Y+capH/2).
		Close()
	p.FillPath(body, fill)
	p.StrokePath(body, sw, stroke)

	topEllipse := &ui.Path{}
	ellipsePath(topEllipse, ui.Rect{X: box.X, Y: box.Y, W: box.W, H: capH})
	p.FillPath(topEllipse, fill)
	p.StrokePath(topEllipse, sw, stroke)
}

func paintCloud(p *ui.Painter, box ui.Rect, fill, stroke ui.Color, sw float32) {
	x, y, w, h := box.X, box.Y, box.W, box.H
	path := &ui.Path{}
	path.MoveTo(x+w*0.25, y+h*0.80).
		CubeTo(x+w*0.05, y+h*0.80, x+w*0.04, y+h*0.48, x+w*0.22, y+h*0.44).
		CubeTo(x+w*0.24, y+h*0.18, x+w*0.54, y+h*0.12, x+w*0.66, y+h*0.32).
		CubeTo(x+w*0.86, y+h*0.28, x+w*0.96, y+h*0.56, x+w*0.80, y+h*0.78).
		LineTo(x+w*0.25, y+h*0.80).Close()
	p.FillPath(path, fill)
	p.StrokePath(path, sw, stroke)
}

func paintDocument(p *ui.Painter, box ui.Rect, fill, stroke ui.Color, sw, z float32) {
	wave := min(box.H*0.18, 14*z)
	path := &ui.Path{}
	path.MoveTo(box.X, box.Y).
		LineTo(box.X+box.W, box.Y).
		LineTo(box.X+box.W, box.Y+box.H-wave).
		QuadTo(box.X+box.W*0.75, box.Y+box.H-wave*2, box.X+box.W*0.5, box.Y+box.H-wave).
		QuadTo(box.X+box.W*0.25, box.Y+box.H, box.X, box.Y+box.H-wave).
		Close()
	p.FillPath(path, fill)
	p.StrokePath(path, sw, stroke)
}

func paintNote(p *ui.Painter, box ui.Rect, fill, stroke ui.Color, sw, z float32) {
	fold := min(min(box.W, box.H)*0.24, 18*z)
	path := &ui.Path{}
	path.MoveTo(box.X, box.Y).
		LineTo(box.X+box.W-fold, box.Y).
		LineTo(box.X+box.W, box.Y+fold).
		LineTo(box.X+box.W, box.Y+box.H).
		LineTo(box.X, box.Y+box.H).Close()
	p.FillPath(path, fill)
	p.StrokePath(path, sw, stroke)

	crease := &ui.Path{}
	crease.MoveTo(box.X+box.W-fold, box.Y).
		LineTo(box.X+box.W-fold, box.Y+fold).
		LineTo(box.X+box.W, box.Y+fold)
	p.StrokePath(crease, sw, stroke)
}

func paintActor(p *ui.Painter, box ui.Rect, stroke ui.Color, sw, z float32) {
	cx := box.X + box.W/2
	headR := min(box.W, box.H) * 0.14
	head := &ui.Path{}
	head.Circle(cx, box.Y+headR+6*z, headR)
	p.StrokePath(head, sw, stroke)

	neckY := box.Y + headR*2 + 8*z
	hipY := box.Y + box.H*0.62
	armY := neckY + (hipY-neckY)*0.35
	armSpan := min(box.W*0.32, 24*z)
	legSpan := min(box.W*0.26, 20*z)
	footY := box.Y + box.H - 22*z
	body := &ui.Path{}
	body.MoveTo(cx, neckY).LineTo(cx, hipY).
		MoveTo(cx-armSpan, armY).LineTo(cx+armSpan, armY).
		MoveTo(cx, hipY).LineTo(cx-legSpan, footY).
		MoveTo(cx, hipY).LineTo(cx+legSpan, footY)
	p.StrokePath(body, sw, stroke)
}

func parseColor(s string) (ui.Color, bool) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return ui.Color{}, false
	}
	if strings.HasPrefix(s, "#") && (len(s) == 4 || len(s) == 7 || len(s) == 9) {
		return ui.Hex(s), true
	}
	return ui.Color{}, false
}

func adjustFillForDark(c ui.Color, raw string, dark bool) ui.Color {
	if !dark {
		return c
	}
	// If document uses default white/very light fill in dark mode, soften so text remains legible.
	if strings.EqualFold(raw, "#FFFFFF") || strings.EqualFold(raw, "#FAFAFA") {
		return ui.Hex("#1E293B")
	}
	return c
}

func adjustTextForDark(c ui.Color, raw string, dark bool) ui.Color {
	if !dark {
		return c
	}
	if strings.EqualFold(raw, "#0F172A") || strings.EqualFold(raw, "#1E293B") || strings.EqualFold(raw, "#18181B") {
		return ui.Hex("#F8FAFC")
	}
	return c
}

// Expand returns a rect grown by pad on all sides (helper used locally on ui.Rect).
func ExpandRect(r ui.Rect, pad float32) ui.Rect {
	return ui.Rect{X: r.X - pad, Y: r.Y - pad, W: r.W + pad*2, H: r.H + pad*2}
}

func paintImageNode(p *ui.Painter, box ui.Rect, st SceneState, pal Palette, n *graph.Node, fill, stroke ui.Color, rad, swpx, z float32) {
	if rad == 0 {
		rad = 8 * z
	}
	p.Fill(box, fill, rad)
	var src ui.ImageSource
	var asset *graph.MediaAsset
	if n.Media != nil && n.Media.AssetID != "" {
		if st.Doc != nil {
			asset = st.Doc.Assets[n.Media.AssetID]
		}
		if st.ResolveMedia != nil {
			src = st.ResolveMedia(n.Media.AssetID)
		}
	}
	mediaBox := box
	footerH := float32(0)
	if n.Media != nil && n.Media.ShowBar && strings.TrimSpace(n.Label) != "" {
		footerH = min(26*z, box.H*0.25)
		mediaBox = ui.Rect{X: box.X, Y: box.Y, W: box.W, H: box.H - footerH}
	}
	if src != nil {
		fit := ui.Contain
		if n.Media != nil {
			switch n.Media.Fit {
			case "cover":
				fit = ui.Cover
			case "fill":
				fit = ui.FillBox
			}
		}
		p.Clip(mediaBox, rad, func() {
			p.Image(src, mediaBox, fit)
		})
	} else {
		// Clean placeholder when image hasn't decoded or path is missing.
		p.Fill(mediaBox, pal.ContainerFill, rad)
		fs := max(float32(9), 12*z)
		msg := "Image Node"
		if asset != nil && asset.Name != "" {
			msg = asset.Name
		}
		tw, _ := p.MeasureText(box.W-12*z, ui.Span{Text: msg, Size: fs})
		p.Text(box.X+(box.W-tw)/2, box.Y+(mediaBox.H-fs)/2, msg, fs, pal.NodeText.Alpha(0.65))
	}
	if footerH > 0 {
		bar := ui.Rect{X: box.X, Y: box.Y + box.H - footerH, W: box.W, H: footerH}
		p.Fill(bar, pal.EdgeLabelBG.Alpha(0.92), 0)
		p.Line(bar.X, bar.Y, bar.X+bar.W, bar.Y, 1, stroke.Alpha(0.5))
		fs := max(float32(8), 11*z)
		lbl := strings.TrimSpace(n.Label)
		if asset != nil && asset.NaturalW > 0 {
			lbl = fmt.Sprintf("%s (%dx%d)", lbl, asset.NaturalW, asset.NaturalH)
		}
		p.Text(bar.X+8*z, bar.Y+(footerH-fs)/2, lbl, fs, pal.NodeText)
	}
	strokeRect(p, box, stroke, rad, swpx, n.Style.Dash)
}

func paintVideoCard(p *ui.Painter, box ui.Rect, st SceneState, pal Palette, n *graph.Node, stroke ui.Color, rad, swpx, z float32) {
	if rad == 0 {
		rad = 10 * z
	}
	var asset *graph.MediaAsset
	if n.Media != nil && n.Media.AssetID != "" && st.Doc != nil {
		asset = st.Doc.Assets[n.Media.AssetID]
	}
	live := st.ActiveVideos != nil && st.ActiveVideos[n.ID] && !st.Export
	if !live {
		p.Fill(box, ui.Hex("#0F172A"), rad)
		// Subtle cinema top/bottom letterbox framing & play emblem.
		cx, cy := box.X+box.W/2, box.Y+box.H/2
		pr := min(min(box.W, box.H)*0.18, 24*z)
		if pr >= 8 {
			circle := &ui.Path{}
			circle.Circle(cx, cy, pr)
			p.FillPath(circle, ui.RGBA(37, 99, 235, 0.90))
			tri := &ui.Path{}
			tri.MoveTo(cx-pr*0.30, cy-pr*0.42).
				LineTo(cx+pr*0.48, cy).
				LineTo(cx-pr*0.30, cy+pr*0.42).Close()
			p.FillPath(tri, ui.Hex("#FFFFFF"))
		}
		// Title & resolution pill
		lbl := strings.TrimSpace(n.Label)
		if lbl == "" && asset != nil {
			lbl = asset.Name
		}
		if lbl != "" {
			fs := max(float32(8), 11*z)
			p.Text(box.X+10*z, box.Y+box.H-fs-8*z, lbl, fs, ui.Hex("#F8FAFC"))
		}
		if asset != nil && asset.NaturalW > 0 {
			badge := fmt.Sprintf("VIDEO %dx%d", asset.NaturalW, asset.NaturalH)
			if asset.DurationSec > 0 {
				badge += fmt.Sprintf(" · %.0fs", asset.DurationSec)
			}
			bfs := max(float32(7), 9.5*z)
			bw, bh := p.MeasureText(box.W-16*z, ui.Span{Text: badge, Size: bfs})
			pill := ui.Rect{X: box.X + 8*z, Y: box.Y + 8*z, W: bw + 10*z, H: bh + 4*z}
			p.Fill(pill, ui.RGBA(15, 23, 42, 0.78), 4*z)
			p.Text(pill.X+5*z, pill.Y+2*z, badge, bfs, ui.Hex("#93C5FD"))
		}
	}
	strokeRect(p, box, stroke, rad, swpx, n.Style.Dash)
}

func init() {
	_ = fmt.Sprint
}
