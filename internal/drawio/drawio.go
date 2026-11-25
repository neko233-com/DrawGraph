// Package drawio implements bidirectional import and export for draw.io (.drawio / mxGraphModel XML) files.
package drawio

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

type mxFile struct {
	XMLName  xml.Name    `xml:"mxfile"`
	Host     string      `xml:"host,attr,omitempty"`
	Agent    string      `xml:"agent,attr,omitempty"`
	Type     string      `xml:"type,attr,omitempty"`
	Diagrams []mxDiagram `xml:"diagram"`
}

type mxDiagram struct {
	ID    string        `xml:"id,attr,omitempty"`
	Name  string        `xml:"name,attr,omitempty"`
	Model *mxGraphModel `xml:"mxGraphModel,omitempty"`
	Raw   string        `xml:",chardata"`
}

type mxGraphModel struct {
	XMLName xml.Name `xml:"mxGraphModel"`
	Grid    string   `xml:"grid,attr,omitempty"`
	GridSz  string   `xml:"gridSize,attr,omitempty"`
	Root    mxRoot   `xml:"root"`
}

type mxRoot struct {
	Cells []mxCell `xml:"mxCell"`
}

type mxCell struct {
	ID       string      `xml:"id,attr"`
	Value    string      `xml:"value,attr,omitempty"`
	Style    string      `xml:"style,attr,omitempty"`
	Vertex   string      `xml:"vertex,attr,omitempty"`
	Edge     string      `xml:"edge,attr,omitempty"`
	Parent   string      `xml:"parent,attr,omitempty"`
	Source   string      `xml:"source,attr,omitempty"`
	Target   string      `xml:"target,attr,omitempty"`
	Geometry *mxGeometry `xml:"mxGeometry,omitempty"`
}

type mxGeometry struct {
	X      string    `xml:"x,attr,omitempty"`
	Y      string    `xml:"y,attr,omitempty"`
	Width  string    `xml:"width,attr,omitempty"`
	Height string    `xml:"height,attr,omitempty"`
	As     string    `xml:"as,attr,omitempty"`
	Rel    string    `xml:"relative,attr,omitempty"`
	Arrays []mxArray `xml:"Array,omitempty"`
}

type mxArray struct {
	As     string    `xml:"as,attr,omitempty"`
	Points []mxPoint `xml:"mxPoint"`
}

type mxPoint struct {
	X string `xml:"x,attr"`
	Y string `xml:"y,attr"`
}

// ParseXML parses either an <mxfile> or standalone <mxGraphModel> XML payload into a graph.Document.
func ParseXML(data []byte) (*graph.Document, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty drawio XML")
	}
	doc := graph.NewDocument("Imported draw.io")
	doc.Pages = nil

	var file mxFile
	if bytes.Contains(trimmed, []byte("<mxfile")) {
		if err := xml.Unmarshal(trimmed, &file); err != nil {
			return nil, fmt.Errorf("parse mxfile: %w", err)
		}
		for i, diag := range file.Diagrams {
			model := diag.Model
			if model == nil && strings.TrimSpace(diag.Raw) != "" {
				var err error
				model, err = decodeCompressedModel(diag.Raw)
				if err != nil {
					return nil, fmt.Errorf("diagram %q: %w", diag.Name, err)
				}
			}
			if model == nil {
				continue
			}
			pageID := diag.ID
			if pageID == "" {
				pageID = fmt.Sprintf("page-%d", i+1)
			}
			pageName := diag.Name
			if pageName == "" {
				pageName = fmt.Sprintf("Page-%d", i+1)
			}
			page := convertModel(pageID, pageName, model, i == 0, doc)
			doc.Pages = append(doc.Pages, page)
		}
	} else {
		var model mxGraphModel
		if err := xml.Unmarshal(trimmed, &model); err != nil {
			return nil, fmt.Errorf("parse mxGraphModel: %w", err)
		}
		doc.Pages = append(doc.Pages, convertModel("page-1", "Page-1", &model, true, doc))
	}

	if len(doc.Pages) == 0 {
		return nil, fmt.Errorf("drawio file contained no diagrams")
	}
	doc.ActivePageID = doc.Pages[0].ID
	doc.Normalize()
	return doc, nil
}

// ImportFile reads a .drawio / .xml file from disk.
func ImportFile(path string) (*graph.Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	doc, err := ParseXML(data)
	if err != nil {
		return nil, err
	}
	base := strings.TrimSuffix(strings.TrimSuffix(filepathBase(path), ".drawio"), ".xml")
	if base != "" {
		doc.Title = base
	}
	return doc, nil
}

func filepathBase(p string) string {
	p = strings.ReplaceAll(p, "\\", "/")
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[i+1:]
	}
	return p
}

// ExportXML serializes a graph.Document into an uncompressed <mxfile> compatible with draw.io.
func ExportXML(doc *graph.Document) ([]byte, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}
	doc.Normalize()
	file := mxFile{
		Host:  "DrawGraph",
		Agent: "DrawGraph/1.0",
		Type:  "device",
	}
	for _, p := range doc.Pages {
		cells := []mxCell{
			{ID: "0"},
			{ID: "1", Parent: "0"},
		}
		nodeCoords := map[string]geo.Vec{}
		for _, n := range p.Nodes {
			nodeCoords[n.ID] = geo.V(n.X, n.Y)
		}
		for _, n := range p.SortedNodes() {
			parent := "1"
			x, y := n.X, n.Y
			if n.ParentID != "" && p.NodeByID(n.ParentID) != nil {
				parent = n.ParentID
				par := p.NodeByID(n.ParentID)
				x = n.X - par.X
				y = n.Y - par.Y
			}
			cells = append(cells, mxCell{
				ID:     n.ID,
				Value:  n.Label,
				Style:  nodeStyleToDrawio(n),
				Vertex: "1",
				Parent: parent,
				Geometry: &mxGeometry{
					X:      fmtNum(x),
					Y:      fmtNum(y),
					Width:  fmtNum(n.W),
					Height: fmtNum(n.H),
					As:     "geometry",
				},
			})
		}
		for _, e := range p.Edges {
			geom := &mxGeometry{Rel: "1", As: "geometry"}
			if len(e.Waypoints) > 0 {
				pts := make([]mxPoint, 0, len(e.Waypoints))
				for _, pt := range e.Waypoints {
					pts = append(pts, mxPoint{X: fmtNum(pt.X), Y: fmtNum(pt.Y)})
				}
				geom.Arrays = []mxArray{{As: "points", Points: pts}}
			}
			cells = append(cells, mxCell{
				ID:       e.ID,
				Value:    e.Label,
				Style:    edgeStyleToDrawio(e),
				Edge:     "1",
				Parent:   "1",
				Source:   e.SourceID,
				Target:   e.TargetID,
				Geometry: geom,
			})
		}
		file.Diagrams = append(file.Diagrams, mxDiagram{
			ID:   p.ID,
			Name: p.Name,
			Model: &mxGraphModel{
				Grid:   bool01(doc.SnapToGrid),
				GridSz: fmtNum(doc.GridSize),
				Root:   mxRoot{Cells: cells},
			},
		})
	}
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(file); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// ExportFile writes doc as a .drawio XML file.
func ExportFile(path string, doc *graph.Document) error {
	data, err := ExportXML(doc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func decodeCompressedModel(raw string) (*mxGraphModel, error) {
	raw = strings.TrimSpace(raw)
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("base64 decode diagram: %w", err)
	}
	r := flate.NewReader(bytes.NewReader(decoded))
	defer r.Close()
	inflated, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("inflate diagram: %w", err)
	}
	unescaped, err := url.QueryUnescape(string(inflated))
	if err != nil {
		unescaped = string(inflated)
	}
	var model mxGraphModel
	if err := xml.Unmarshal([]byte(unescaped), &model); err != nil {
		return nil, fmt.Errorf("unmarshal inflated mxGraphModel: %w", err)
	}
	return &model, nil
}

func convertModel(pageID, pageName string, model *mxGraphModel, applyGrid bool, doc *graph.Document) *graph.Page {
	if applyGrid && model.GridSz != "" {
		if gs, err := strconv.ParseFloat(model.GridSz, 64); err == nil && gs > 0 {
			doc.GridSize = gs
		}
	}
	p := &graph.Page{ID: pageID, Name: pageName, Nodes: []*graph.Node{}, Edges: []*graph.Edge{}}

	// Map cell coordinates first so relative child coordinates resolve to absolute world coords.
	cellMap := map[string]mxCell{}
	for _, c := range model.Root.Cells {
		cellMap[c.ID] = c
	}

	absOrigin := func(c mxCell) (float64, float64) {
		var ox, oy float64
		cur := c
		seen := map[string]bool{}
		for cur.Parent != "" && cur.Parent != "0" && cur.Parent != "1" && !seen[cur.Parent] {
			seen[cur.Parent] = true
			par, ok := cellMap[cur.Parent]
			if !ok || par.Geometry == nil {
				break
			}
			ox += parseFloat(par.Geometry.X, 0)
			oy += parseFloat(par.Geometry.Y, 0)
			cur = par
		}
		return ox, oy
	}

	for _, c := range model.Root.Cells {
		if c.ID == "0" || c.ID == "1" {
			continue
		}
		st := parseDrawioStyle(c.Style)
		if c.Vertex == "1" || (c.Edge != "1" && c.Source == "" && c.Target == "" && c.Geometry != nil && c.Geometry.Width != "") {
			ox, oy := absOrigin(c)
			x := ox + parseFloat(geomVal(c.Geometry, "x"), 0)
			y := oy + parseFloat(geomVal(c.Geometry, "y"), 0)
			w := parseFloat(geomVal(c.Geometry, "width"), 140)
			h := parseFloat(geomVal(c.Geometry, "height"), 64)
			parentID := ""
			if c.Parent != "" && c.Parent != "0" && c.Parent != "1" {
				parentID = c.Parent
			}
			node := &graph.Node{
				ID:       c.ID,
				Kind:     drawioStyleToKind(st),
				ParentID: parentID,
				X:        x,
				Y:        y,
				W:        w,
				H:        h,
				Label:    stripHTML(html.UnescapeString(c.Value)),
				Style: graph.NodeStyle{
					Fill:        st["fillColor"],
					Stroke:      st["strokeColor"],
					StrokeWidth: parseFloat(st["strokeWidth"], 1.5),
					Shadow:      st["shadow"] == "1",
					FontSize:    parseFloat(st["fontSize"], 13),
					TextColor:   st["fontColor"],
					Align:       st["align"],
					VAlign:      st["verticalAlign"],
				},
			}
			if st["dashed"] == "1" {
				node.Style.Dash = graph.DashDashed
			}
			if op := parseFloat(st["opacity"], 100); op > 0 && op < 100 {
				node.Style.Opacity = op / 100.0
			}
			if icon := st["drawgraphIcon"]; icon != "" {
				node.Icon = graph.ParseIconRef(icon)
			}
			graph.NormalizeNode(node)
			p.Nodes = append(p.Nodes, node)
			continue
		}
		if c.Edge == "1" && c.Source != "" && c.Target != "" {
			edge := &graph.Edge{
				ID:         c.ID,
				SourceID:   c.Source,
				TargetID:   c.Target,
				Label:      stripHTML(html.UnescapeString(c.Value)),
				Routing:    drawioStyleToRouting(st),
				StartArrow: drawioArrow(st["startArrow"], graph.ArrowNone),
				EndArrow:   drawioArrow(st["endArrow"], graph.ArrowClassic),
				Style: graph.EdgeStyle{
					Stroke:      st["strokeColor"],
					StrokeWidth: parseFloat(st["strokeWidth"], 1.75),
					FontSize:    parseFloat(st["fontSize"], 12),
					TextColor:   st["fontColor"],
				},
			}
			if st["dashed"] == "1" {
				edge.Style.Dash = graph.DashDashed
			}
			if c.Geometry != nil {
				for _, arr := range c.Geometry.Arrays {
					if arr.As == "points" {
						for _, pt := range arr.Points {
							edge.Waypoints = append(edge.Waypoints, geo.V(parseFloat(pt.X, 0), parseFloat(pt.Y, 0)))
						}
					}
				}
			}
			graph.NormalizeEdge(edge, doc.DefaultRouting)
			p.Edges = append(p.Edges, edge)
		}
	}
	return p
}

func parseDrawioStyle(raw string) map[string]string {
	m := map[string]string{}
	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if k, v, ok := strings.Cut(part, "="); ok {
			m[k] = v
		} else {
			m[part] = "1"
		}
	}
	return m
}

func drawioStyleToKind(st map[string]string) graph.NodeKind {
	if st["drawgraphKind"] != "" {
		return graph.NodeKind(st["drawgraphKind"])
	}
	if st["swimlane"] == "1" {
		return graph.KindSwimlane
	}
	if st["text"] == "1" {
		return graph.KindText
	}
	if st["ellipse"] == "1" || st["shape"] == "ellipse" {
		return graph.KindEllipse
	}
	if st["rhombus"] == "1" || st["shape"] == "rhombus" {
		return graph.KindDiamond
	}
	switch st["shape"] {
	case "hexagon":
		return graph.KindHexagon
	case "parallelogram":
		return graph.KindParallelogram
	case "cylinder", "cylinder3":
		return graph.KindCylinder
	case "cloud":
		return graph.KindCloud
	case "document":
		return graph.KindDocument
	case "umlActor":
		return graph.KindActor
	case "note":
		return graph.KindNote
	case "image":
		return graph.KindIcon
	}
	if st["rounded"] == "1" {
		return graph.KindRounded
	}
	return graph.KindRect
}

func drawioStyleToRouting(st map[string]string) graph.RoutingStyle {
	if st["curved"] == "1" {
		return graph.RouteCurved
	}
	if st["edgeStyle"] == "orthogonalEdgeStyle" {
		return graph.RouteOrthogonal
	}
	if st["edgeStyle"] == "none" {
		return graph.RouteStraight
	}
	return graph.RouteOrthogonal
}

func drawioArrow(raw string, fallback graph.ArrowStyle) graph.ArrowStyle {
	switch raw {
	case "":
		return fallback
	case "none":
		return graph.ArrowNone
	case "classic", "block":
		return graph.ArrowClassic
	case "open":
		return graph.ArrowOpen
	case "diamond":
		return graph.ArrowDiamond
	case "oval":
		return graph.ArrowCircle
	default:
		return fallback
	}
}

func nodeStyleToDrawio(n *graph.Node) string {
	var parts []string
	switch n.Kind {
	case graph.KindRounded:
		parts = append(parts, "rounded=1", "whiteSpace=wrap", "html=1")
	case graph.KindEllipse:
		parts = append(parts, "ellipse", "whiteSpace=wrap", "html=1")
	case graph.KindDiamond:
		parts = append(parts, "rhombus", "whiteSpace=wrap", "html=1")
	case graph.KindHexagon:
		parts = append(parts, "shape=hexagon", "perimeter=hexagonPerimeter2", "whiteSpace=wrap", "html=1")
	case graph.KindParallelogram:
		parts = append(parts, "shape=parallelogram", "perimeter=parallelogramPerimeter", "whiteSpace=wrap", "html=1")
	case graph.KindCylinder:
		parts = append(parts, "shape=cylinder3", "whiteSpace=wrap", "html=1", "boundedLbl=1", "backgroundOutline=1", "size=15")
	case graph.KindCloud:
		parts = append(parts, "ellipse", "shape=cloud", "whiteSpace=wrap", "html=1")
	case graph.KindDocument:
		parts = append(parts, "shape=document", "whiteSpace=wrap", "html=1", "boundedLbl=1")
	case graph.KindActor:
		parts = append(parts, "shape=umlActor", "verticalLabelPosition=bottom", "verticalAlign=top", "html=1")
	case graph.KindNote:
		parts = append(parts, "shape=note", "whiteSpace=wrap", "html=1", "backgroundOutline=1")
	case graph.KindText:
		parts = append(parts, "text", "html=1", "strokeColor=none", "fillColor=none")
	case graph.KindSwimlane:
		parts = append(parts, "swimlane", "whiteSpace=wrap", "html=1", "startSize=28")
	case graph.KindContainer:
		parts = append(parts, "rounded=1", "whiteSpace=wrap", "html=1", "container=1")
	case graph.KindIcon:
		parts = append(parts, "rounded=1", "whiteSpace=wrap", "html=1")
	default:
		parts = append(parts, "rounded=0", "whiteSpace=wrap", "html=1")
	}
	parts = append(parts, "drawgraphKind="+string(n.Kind))
	if n.Style.Fill != "" {
		parts = append(parts, "fillColor="+n.Style.Fill)
	}
	if n.Style.Stroke != "" {
		parts = append(parts, "strokeColor="+n.Style.Stroke)
	}
	if n.Style.StrokeWidth > 0 {
		parts = append(parts, "strokeWidth="+fmtNum(n.Style.StrokeWidth))
	}
	if n.Style.Dash != "" && n.Style.Dash != graph.DashSolid {
		parts = append(parts, "dashed=1")
	}
	if n.Style.Shadow {
		parts = append(parts, "shadow=1")
	}
	if n.Style.FontSize > 0 {
		parts = append(parts, "fontSize="+fmtNum(n.Style.FontSize))
	}
	if n.Style.TextColor != "" {
		parts = append(parts, "fontColor="+n.Style.TextColor)
	}
	if n.Style.Opacity > 0 && n.Style.Opacity < 1 {
		parts = append(parts, "opacity="+fmtNum(n.Style.Opacity*100))
	}
	if n.Icon.Valid() {
		parts = append(parts, "drawgraphIcon="+n.Icon.String())
	}
	return strings.Join(parts, ";") + ";"
}

func edgeStyleToDrawio(e *graph.Edge) string {
	parts := []string{"html=1"}
	switch e.Routing {
	case graph.RouteStraight:
		parts = append(parts, "edgeStyle=none")
	case graph.RouteCurved:
		parts = append(parts, "edgeStyle=orthogonalEdgeStyle", "curved=1")
	default:
		parts = append(parts, "edgeStyle=orthogonalEdgeStyle", "rounded=1")
	}
	if e.StartArrow != "" && e.StartArrow != graph.ArrowNone {
		parts = append(parts, "startArrow="+string(e.StartArrow))
	}
	if e.EndArrow != "" {
		parts = append(parts, "endArrow="+drawioArrowName(e.EndArrow))
	}
	if e.Style.Stroke != "" {
		parts = append(parts, "strokeColor="+e.Style.Stroke)
	}
	if e.Style.StrokeWidth > 0 {
		parts = append(parts, "strokeWidth="+fmtNum(e.Style.StrokeWidth))
	}
	if e.Style.Dash != "" && e.Style.Dash != graph.DashSolid {
		parts = append(parts, "dashed=1")
	}
	if e.Style.FontSize > 0 {
		parts = append(parts, "fontSize="+fmtNum(e.Style.FontSize))
	}
	return strings.Join(parts, ";") + ";"
}

func drawioArrowName(a graph.ArrowStyle) string {
	switch a {
	case graph.ArrowNone:
		return "none"
	case graph.ArrowClassic:
		return "classic"
	case graph.ArrowOpen:
		return "open"
	case graph.ArrowDiamond:
		return "diamond"
	case graph.ArrowCircle:
		return "oval"
	default:
		return "classic"
	}
}

func geomVal(g *mxGeometry, field string) string {
	if g == nil {
		return ""
	}
	switch field {
	case "x":
		return g.X
	case "y":
		return g.Y
	case "width":
		return g.Width
	case "height":
		return g.Height
	default:
		return ""
	}
}

func parseFloat(s string, def float64) float64 {
	if strings.TrimSpace(s) == "" {
		return def
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return def
	}
	return v
}

func fmtNum(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func bool01(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func stripHTML(s string) string {
	if !strings.ContainsAny(s, "<>") {
		return s
	}
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	var b strings.Builder
	inTag := false
	for _, r := range s {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		default:
			if !inTag {
				b.WriteRune(r)
			}
		}
	}
	return strings.TrimSpace(b.String())
}
