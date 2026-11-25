// Package export renders DrawGraph documents to standalone PNG and SVG files
// without requiring a desktop window, including tiled high-DPI infinite-canvas export.
package export

import (
	"bytes"
	"fmt"
	"html"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/media"
	"github.com/neko233-com/DrawGraph/internal/render"
)

// Driver renders a document page to a custom export byte format (extensible for PDF, Mermaid, etc.).
type Driver func(doc *graph.Document, pageID string, reg *icons.Registry, dark bool, scale float64) ([]byte, error)

var customDrivers = map[string]Driver{}

// RegisterDriver registers an extensible export format by lowercase file extension (e.g. ".mermaid").
func RegisterDriver(ext string, fn Driver) {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" || fn == nil {
		return
	}
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	customDrivers[ext] = fn
}

// SupportedExtensions returns built-in and registered custom export extensions.
func SupportedExtensions() []string {
	out := []string{".png", ".svg"}
	for ext := range customDrivers {
		out = append(out, ext)
	}
	return out
}

// RenderPNG renders the specified page (or ActivePage when pageID == "") to PNG bytes.
// Uses tiled ui.Render (up to 8192px) so huge infinite canvases and 4K embedded images export cleanly.
func RenderPNG(doc *graph.Document, pageID string, reg *icons.Registry, dark bool, scale float64) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}
	if scale <= 0 {
		scale = 2
	}
	page := doc.ActivePage()
	if pageID != "" {
		if found := doc.PageByID(pageID); found != nil {
			page = found
		} else {
			return nil, fmt.Errorf("page %q not found", pageID)
		}
	}
	box, ok := page.Bounds()
	if !ok {
		box = geo.B(0, 0, 640, 400)
	} else {
		box = box.Expand(48)
	}
	box.W = max(box.W, 160)
	box.H = max(box.H, 120)

	const maxSide = 8192.0
	if m := max(box.W, box.H) * scale; m > maxSide {
		scale = maxSide / max(box.W, box.H)
	}

	var mu sync.Mutex
	mediaCache := map[string]ui.ImageSource{}
	resolveMedia := func(assetID string) ui.ImageSource {
		mu.Lock()
		defer mu.Unlock()
		if s, ok := mediaCache[assetID]; ok {
			return s
		}
		asset := doc.Assets[assetID]
		if asset == nil || asset.Kind != "image" {
			return nil
		}
		src, err := media.DecodeAssetImage(asset)
		if err != nil {
			mediaCache[assetID] = nil
			return nil
		}
		mediaCache[assetID] = src
		return src
	}

	const tile = 1024
	w, h := max(int(math.Ceil(box.W*scale)), 1), max(int(math.Ceil(box.H*scale)), 1)
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	side := max(int(math.Ceil(tile/scale)), 1)

	for ty := 0; ty < h; ty += tile {
		for tx := 0; tx < w; tx += tile {
			atX := box.X + float64(tx)/scale
			atY := box.Y + float64(ty)/scale
			tileImg := ui.Render(func(c *ui.Context) {
				ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
					render.PaintScene(p, r, render.SceneState{
						Doc:          doc,
						Page:         page,
						Icons:        reg,
						ResolveMedia: resolveMedia,
						Cam:          render.Camera{X: -atX, Y: -atY, Zoom: 1},
						Dark:         dark,
						Export:       true,
					})
				})
			}, side, side, float32(scale))
			drawRect := image.Rect(tx, ty, min(tx+tile, w), min(ty+tile, h))
			draw.Draw(out, drawRect, tileImg, image.Point{}, draw.Src)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, out); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// RenderSVG renders the specified page (or ActivePage when pageID == "") as clean standalone SVG,
// embedding any custom/Git-installed icon SVGs and HD image DataURLs cleanly.
func RenderSVG(doc *graph.Document, pageID string, reg *icons.Registry, dark bool) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}
	page := doc.ActivePage()
	if pageID != "" {
		if found := doc.PageByID(pageID); found != nil {
			page = found
		} else {
			return nil, fmt.Errorf("page %q not found", pageID)
		}
	}
	box, ok := page.Bounds()
	if !ok {
		box = geo.B(0, 0, 640, 400)
	} else {
		box = box.Expand(48)
	}

	bg := "#F8FAFC"
	defaultFill := "#FFFFFF"
	defaultStroke := "#CBD5E1"
	defaultText := "#0F172A"
	defaultEdge := "#64748B"
	if dark {
		bg = "#0B1120"
		defaultFill = "#1E293B"
		defaultStroke = "#475569"
		defaultText = "#F8FAFC"
		defaultEdge = "#94A3B8"
	}
	if page.Background != "" {
		bg = page.Background
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="%.2f %.2f %.2f %.2f">`+"\n",
		box.W, box.H, box.X, box.Y, box.W, box.H)
	b.WriteString(`<defs>` + "\n")
	fmt.Fprintf(&b, `  <marker id="dg-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 1 L 10 5 L 0 9 z" fill="%s"/></marker>`+"\n", defaultEdge)
	b.WriteString(`  <style>text{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI","Microsoft YaHei","PingFang SC","Noto Sans CJK SC",Roboto,sans-serif;}</style>` + "\n")
	b.WriteString(`</defs>` + "\n")
	fmt.Fprintf(&b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" fill="%s"/>`+"\n", box.X, box.Y, box.W, box.H, html.EscapeString(bg))

	idx := page.EnsureIndex()
	sorted := idx.Sorted()
	for _, n := range sorted {
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
			writeSVGNode(&b, doc, n, reg, defaultFill, defaultStroke, defaultText, dark)
		}
	}
	for _, ve := range idx.QueryEdges(page, box) {
		writeSVGVisibleEdge(&b, ve, defaultEdge, bg, defaultText)
	}
	for _, n := range sorted {
		if n.Kind != graph.KindContainer && n.Kind != graph.KindSwimlane {
			writeSVGNode(&b, doc, n, reg, defaultFill, defaultStroke, defaultText, dark)
		}
	}

	b.WriteString("</svg>\n")
	return []byte(b.String()), nil
}

// ExportFile writes PNG or SVG inferred from the target path extension.
func ExportFile(outPath string, doc *graph.Document, pageID string, reg *icons.Registry, dark bool, scale float64) error {
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	ext := strings.ToLower(filepath.Ext(outPath))
	switch ext {
	case ".png":
		data, err := RenderPNG(doc, pageID, reg, dark, scale)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, data, 0o644)
	case ".svg":
		data, err := RenderSVG(doc, pageID, reg, dark)
		if err != nil {
			return err
		}
		return os.WriteFile(outPath, data, 0o644)
	default:
		if drv, ok := customDrivers[ext]; ok {
			data, err := drv(doc, pageID, reg, dark, scale)
			if err != nil {
				return err
			}
			return os.WriteFile(outPath, data, 0o644)
		}
		return fmt.Errorf("unsupported export extension %q (expected .png, .svg, or registered driver)", ext)
	}
}

func writeSVGNode(b *strings.Builder, doc *graph.Document, n *graph.Node, reg *icons.Registry, defFill, defStroke, defText string, dark bool) {
	fill := firstStr(n.Style.Fill, defFill)
	if dark && strings.EqualFold(fill, "#FFFFFF") {
		fill = defFill
	}
	stroke := firstStr(n.Style.Stroke, defStroke)
	textCol := firstStr(n.Style.TextColor, defText)
	if dark && strings.EqualFold(textCol, "#0F172A") {
		textCol = defText
	}
	sw := n.Style.StrokeWidth
	if sw <= 0 {
		sw = 1.5
	}
	dashAttr := ""
	if n.Style.Dash == graph.DashDashed {
		dashAttr = ` stroke-dasharray="6,4"`
	} else if n.Style.Dash == graph.DashDotted {
		dashAttr = ` stroke-dasharray="2,4"`
	}

	switch n.Kind {
	case graph.KindText:
	case graph.KindImage:
		rad := n.Style.Radius
		if rad == 0 {
			rad = 8
		}
		fmt.Fprintf(b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			n.X, n.Y, n.W, n.H, rad, esc(fill), esc(stroke), sw, dashAttr)
		if n.Media != nil && doc != nil {
			if a := doc.Assets[n.Media.AssetID]; a != nil && a.DataURL != "" {
				par := "xMidYMid meet"
				if n.Media.Fit == "cover" {
					par = "xMidYMid slice"
				} else if n.Media.Fit == "fill" {
					par = "none"
				}
				fmt.Fprintf(b, `<image href="%s" x="%.2f" y="%.2f" width="%.2f" height="%.2f" preserveAspectRatio="%s"/>`+"\n",
					esc(a.DataURL), n.X, n.Y, n.W, n.H, par)
			}
		}
		return
	case graph.KindVideo:
		rad := n.Style.Radius
		if rad == 0 {
			rad = 10
		}
		fmt.Fprintf(b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" fill="#0F172A" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			n.X, n.Y, n.W, n.H, rad, esc(stroke), sw, dashAttr)
		cx, cy := n.X+n.W/2, n.Y+n.H/2
		fmt.Fprintf(b, `<circle cx="%.2f" cy="%.2f" r="22" fill="#2563EB" fill-opacity="0.9"/>`+"\n", cx, cy)
		fmt.Fprintf(b, `<polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="#FFFFFF"/>`+"\n", cx-6, cy-9, cx+10, cy, cx-6, cy+9)
		if strings.TrimSpace(n.Label) != "" {
			fmt.Fprintf(b, `<text x="%.2f" y="%.2f" font-size="12" fill="#F8FAFC">%s</text>`+"\n", n.X+10, n.Y+n.H-10, esc(n.Label))
		}
		return
	case graph.KindEllipse:
		fmt.Fprintf(b, `<ellipse cx="%.2f" cy="%.2f" rx="%.2f" ry="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			n.X+n.W/2, n.Y+n.H/2, n.W/2, n.H/2, esc(fill), esc(stroke), sw, dashAttr)
	case graph.KindDiamond:
		cx, cy := n.X+n.W/2, n.Y+n.H/2
		fmt.Fprintf(b, `<polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			cx, n.Y, n.X+n.W, cy, cx, n.Y+n.H, n.X, cy, esc(fill), esc(stroke), sw, dashAttr)
	case graph.KindHexagon:
		inset := min(n.W*0.2, 22)
		cy := n.Y + n.H/2
		fmt.Fprintf(b, `<polygon points="%.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f %.2f,%.2f" fill="%s" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			n.X+inset, n.Y, n.X+n.W-inset, n.Y, n.X+n.W, cy, n.X+n.W-inset, n.Y+n.H, n.X+inset, n.Y+n.H, n.X, cy,
			esc(fill), esc(stroke), sw, dashAttr)
	default:
		rad := n.Style.Radius
		if n.Kind == graph.KindRect {
			rad = 2
		}
		fmt.Fprintf(b, `<rect x="%.2f" y="%.2f" width="%.2f" height="%.2f" rx="%.2f" fill="%s" stroke="%s" stroke-width="%.2f"%s/>`+"\n",
			n.X, n.Y, n.W, n.H, rad, esc(fill), esc(stroke), sw, dashAttr)
	}

	if n.Icon.Valid() && reg != nil {
		if raw, ok := reg.SVGBytes(n.Icon.String()); ok {
			sz := n.Icon.Size
			if sz <= 0 {
				if n.Kind == graph.KindIcon {
					sz = min(n.W, n.H) * 0.5
				} else {
					sz = 22
				}
			}
			ix := n.X + 12
			iy := n.Y + (n.H-sz)/2
			if n.Kind == graph.KindIcon || n.Icon.Position == "top" || n.Icon.Position == "center" {
				ix = n.X + (n.W-sz)/2
				iy = n.Y + 10
			}
			tint := firstStr(n.Icon.Tint, textCol)
			inner := extractSVGInner(string(raw), tint)
			vb := extractSVGViewBox(string(raw))
			fmt.Fprintf(b, `<svg x="%.2f" y="%.2f" width="%.2f" height="%.2f" viewBox="%s" color="%s">%s</svg>`+"\n",
				ix, iy, sz, sz, esc(vb), esc(tint), inner)
		}
	}

	if strings.TrimSpace(n.Label) != "" {
		fs := n.Style.FontSize
		if fs <= 0 {
			fs = 13
		}
		tx := n.X + n.W/2
		ty := n.Y + n.H/2 + fs*0.35
		anchor := "middle"
		if n.Kind == graph.KindContainer || n.Kind == graph.KindSwimlane {
			tx = n.X + 12
			ty = n.Y + 20
			anchor = "start"
		} else if n.Icon.Valid() && n.Kind != graph.KindIcon && n.Icon.Position != "top" && n.Icon.Position != "center" {
			tx = n.X + (n.W+24)/2
		} else if n.Icon.Valid() && (n.Kind == graph.KindIcon || n.Icon.Position == "top") {
			ty = n.Y + n.H - 12
		}
		fmt.Fprintf(b, `<text x="%.2f" y="%.2f" text-anchor="%s" font-size="%.1f" fill="%s">%s</text>`+"\n",
			tx, ty, anchor, fs, esc(textCol), esc(n.Label))
	}
}

func writeSVGVisibleEdge(b *strings.Builder, ve graph.VisibleEdge, defEdge, labelBG, labelText string) {
	e := ve.Edge
	pts := ve.Points
	if e == nil || len(pts) < 2 {
		return
	}
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
	col := firstStr(e.Style.Stroke, defEdge)
	sw := e.Style.StrokeWidth
	if sw <= 0 {
		sw = 1.75
	}
	marker := ""
	if e.EndArrow != "" && e.EndArrow != graph.ArrowNone {
		marker += ` marker-end="url(#dg-arrow)"`
	}
	if e.StartArrow != "" && e.StartArrow != graph.ArrowNone {
		marker += ` marker-start="url(#dg-arrow)"`
	}
	dashAttr := ""
	if e.Style.Dash == graph.DashDashed {
		dashAttr = ` stroke-dasharray="6,4"`
	} else if e.Style.Dash == graph.DashDotted {
		dashAttr = ` stroke-dasharray="2,4"`
	}
	if e.Routing == graph.RouteCurved && len(pts) == 2 {
		a, c := pts[0], pts[1]
		dx := (c.X - a.X) * 0.5
		dy := (c.Y - a.Y) * 0.5
		var d string
		if math.Abs(dx) >= math.Abs(dy) {
			d = fmt.Sprintf("M %.2f %.2f C %.2f %.2f, %.2f %.2f, %.2f %.2f", a.X, a.Y, a.X+dx, a.Y, c.X-dx, c.Y, c.X, c.Y)
		} else {
			d = fmt.Sprintf("M %.2f %.2f C %.2f %.2f, %.2f %.2f, %.2f %.2f", a.X, a.Y, a.X, a.Y+dy, c.X, c.Y-dy, c.X, c.Y)
		}
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%.2f"%s%s/>`+"\n",
			d, esc(col), sw, dashAttr, marker)
	} else if e.Routing == graph.RouteCurved && len(pts) == 3 {
		d := fmt.Sprintf("M %.2f %.2f Q %.2f %.2f, %.2f %.2f", pts[0].X, pts[0].Y, pts[1].X, pts[1].Y, pts[2].X, pts[2].Y)
		fmt.Fprintf(b, `<path d="%s" fill="none" stroke="%s" stroke-width="%.2f"%s%s/>`+"\n",
			d, esc(col), sw, dashAttr, marker)
	} else {
		var sb strings.Builder
		for i, pt := range pts {
			if i > 0 {
				sb.WriteByte(' ')
			}
			fmt.Fprintf(&sb, "%.2f,%.2f", pt.X, pt.Y)
		}
		fmt.Fprintf(b, `<polyline points="%s" fill="none" stroke="%s" stroke-width="%.2f" stroke-linejoin="round"%s%s/>`+"\n",
			sb.String(), esc(col), sw, dashAttr, marker)
	}
	if strings.TrimSpace(e.Label) != "" {
		mid := geo.PolylineMidpoint(pts)
		tw := float64(len([]rune(e.Label)))*7.5 + 14
		fmt.Fprintf(b, `<rect x="%.2f" y="%.2f" width="%.2f" height="20" rx="4" fill="%s" stroke="%s" stroke-opacity="0.35"/>`+"\n",
			mid.X-tw/2, mid.Y-10, tw, esc(labelBG), esc(col))
		fmt.Fprintf(b, `<text x="%.2f" y="%.2f" text-anchor="middle" font-size="12" fill="%s">%s</text>`+"\n",
			mid.X, mid.Y+4, esc(labelText), esc(e.Label))
	}
}

func extractSVGViewBox(raw string) string {
	lower := strings.ToLower(raw)
	if idx := strings.Index(lower, "viewbox="); idx >= 0 {
		rest := raw[idx+len("viewbox="):]
		if len(rest) > 0 && (rest[0] == '"' || rest[0] == '\'') {
			q := rest[0]
			if end := strings.IndexByte(rest[1:], q); end > 0 {
				return rest[1 : 1+end]
			}
		}
	}
	return "0 0 24 24"
}

func extractSVGInner(raw, tint string) string {
	start := strings.Index(raw, ">")
	end := strings.LastIndex(raw, "</svg>")
	if start < 0 || end <= start {
		return ""
	}
	inner := raw[start+1 : end]
	if tint != "" {
		inner = strings.ReplaceAll(inner, "currentColor", tint)
	}
	return inner
}

func firstStr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func esc(s string) string { return html.EscapeString(s) }
