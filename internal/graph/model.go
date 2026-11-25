// Package graph defines the DrawGraph diagram document model, node/edge primitives,
// validation, atomic persistence, and snapshot-based undo/redo history.
package graph

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/neko233-com/DrawGraph/internal/geo"
)

const FormatVersion = 1

// NodeKind identifies a draw.io-compatible diagram node geometry.
type NodeKind string

const (
	KindRect          NodeKind = "rect"
	KindRounded       NodeKind = "rounded"
	KindEllipse       NodeKind = "ellipse"
	KindDiamond       NodeKind = "diamond"
	KindHexagon       NodeKind = "hexagon"
	KindParallelogram NodeKind = "parallelogram"
	KindCylinder      NodeKind = "cylinder"
	KindCloud         NodeKind = "cloud"
	KindDocument      NodeKind = "document"
	KindActor         NodeKind = "actor"
	KindNote          NodeKind = "note"
	KindText          NodeKind = "text"
	KindContainer     NodeKind = "container"
	KindSwimlane      NodeKind = "swimlane"
	KindIcon          NodeKind = "icon"
	KindImage         NodeKind = "image"
	KindVideo         NodeKind = "video"
)

// AllNodeKinds lists every built-in node shape in display order.
var AllNodeKinds = []NodeKind{
	KindRounded,
	KindRect,
	KindEllipse,
	KindDiamond,
	KindHexagon,
	KindParallelogram,
	KindCylinder,
	KindCloud,
	KindDocument,
	KindActor,
	KindNote,
	KindText,
	KindContainer,
	KindSwimlane,
	KindIcon,
	KindImage,
	KindVideo,
}

// DashStyle specifies stroke dash pattern.
type DashStyle string

const (
	DashSolid  DashStyle = "solid"
	DashDashed DashStyle = "dashed"
	DashDotted DashStyle = "dotted"
)

// RoutingStyle specifies how an edge routes between endpoints.
type RoutingStyle string

const (
	RouteOrthogonal RoutingStyle = "orthogonal"
	RouteStraight   RoutingStyle = "straight"
	RouteCurved     RoutingStyle = "curved"
)

// ArrowStyle specifies connector endpoint markers.
type ArrowStyle string

const (
	ArrowNone    ArrowStyle = "none"
	ArrowClassic ArrowStyle = "arrow"
	ArrowOpen    ArrowStyle = "open"
	ArrowDiamond ArrowStyle = "diamond"
	ArrowCircle  ArrowStyle = "circle"
)

// IconRef binds a node to an icon in a built-in or user-extended icon pack.
type IconRef struct {
	PackID   string  `json:"packId,omitempty"`
	Name     string  `json:"name,omitempty"`
	Position string  `json:"position,omitempty"` // "top", "left", "center", "badge"
	Size     float64 `json:"size,omitempty"`
	Tint     string  `json:"tint,omitempty"`
}

// Valid reports whether IconRef identifies a non-empty icon.
func (r *IconRef) Valid() bool {
	return r != nil && strings.TrimSpace(r.PackID) != "" && strings.TrimSpace(r.Name) != ""
}

// String returns "packId/name" or "" when empty.
func (r *IconRef) String() string {
	if !r.Valid() {
		return ""
	}
	return r.PackID + "/" + r.Name
}

// ParseIconRef parses "packId/iconName" (defaulting packId to "builtin" when omitted).
func ParseIconRef(raw string) *IconRef {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if i := strings.IndexByte(raw, '/'); i > 0 {
		return &IconRef{PackID: raw[:i], Name: raw[i+1:]}
	}
	return &IconRef{PackID: "builtin", Name: raw}
}

// NodeStyle configures visual rendering of a node.
type NodeStyle struct {
	Fill        string    `json:"fill,omitempty"`
	Stroke      string    `json:"stroke,omitempty"`
	StrokeWidth float64   `json:"strokeWidth,omitempty"`
	Dash        DashStyle `json:"dash,omitempty"`
	Radius      float64   `json:"radius,omitempty"`
	Shadow      bool      `json:"shadow,omitempty"`
	Opacity     float64   `json:"opacity,omitempty"`
	FontSize    float64   `json:"fontSize,omitempty"`
	FontFamily  string    `json:"fontFamily,omitempty"`
	TextColor   string    `json:"textColor,omitempty"`
	Align       string    `json:"align,omitempty"`  // "left", "center", "right"
	VAlign      string    `json:"valign,omitempty"` // "top", "middle", "bottom"
}

// EdgeStyle configures visual rendering of a connector.
type EdgeStyle struct {
	Stroke      string    `json:"stroke,omitempty"`
	StrokeWidth float64   `json:"strokeWidth,omitempty"`
	Dash        DashStyle `json:"dash,omitempty"`
	Opacity     float64   `json:"opacity,omitempty"`
	FontSize    float64   `json:"fontSize,omitempty"`
	TextColor   string    `json:"textColor,omitempty"`
}

// NodeMedia binds an image or video node to a MediaAsset in Document.Assets.
type NodeMedia struct {
	AssetID  string `json:"assetId,omitempty"`
	Fit      string `json:"fit,omitempty"` // "contain" (default), "cover", "fill"
	Autoplay bool   `json:"autoplay,omitempty"`
	Loop     bool   `json:"loop,omitempty"`
	Muted    bool   `json:"muted,omitempty"`
	ShowBar  bool   `json:"showBar,omitempty"`
}

// MediaAsset holds full-resolution image or video metadata/payload shared by nodes.
type MediaAsset struct {
	ID          string  `json:"id"`
	Kind        string  `json:"kind"` // "image" or "video"
	Name        string  `json:"name,omitempty"`
	MimeType    string  `json:"mimeType,omitempty"`
	FilePath    string  `json:"filePath,omitempty"`
	DataURL     string  `json:"dataUrl,omitempty"`
	NaturalW    int     `json:"naturalW,omitempty"`
	NaturalH    int     `json:"naturalH,omitempty"`
	DurationSec float64 `json:"durationSec,omitempty"`
}

// Node represents a shape, container, text block, icon, or media element on a page.
type Node struct {
	ID        string            `json:"id"`
	Kind      NodeKind          `json:"kind"`
	ParentID  string            `json:"parentId,omitempty"`
	X         float64           `json:"x"`
	Y         float64           `json:"y"`
	W         float64           `json:"w"`
	H         float64           `json:"h"`
	Rotation  float64           `json:"rotation,omitempty"`
	ZIndex    int               `json:"zIndex,omitempty"`
	Label     string            `json:"label,omitempty"`
	Sublabel  string            `json:"sublabel,omitempty"`
	Icon      *IconRef          `json:"icon,omitempty"`
	Media     *NodeMedia        `json:"media,omitempty"`
	Style     NodeStyle         `json:"style,omitempty"`
	Locked    bool              `json:"locked,omitempty"`
	Collapsed bool              `json:"collapsed,omitempty"`
	Tags      []string          `json:"tags,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

// Box returns the node's axis-aligned bounding box.
func (n *Node) Box() geo.Box {
	return geo.Box{X: n.X, Y: n.Y, W: n.W, H: n.H}
}

// Edge represents a connector between two nodes (or free points via Waypoints).
type Edge struct {
	ID           string            `json:"id"`
	SourceID     string            `json:"sourceId"`
	TargetID     string            `json:"targetId"`
	SourceAnchor geo.AnchorSide    `json:"sourceAnchor,omitempty"`
	TargetAnchor geo.AnchorSide    `json:"targetAnchor,omitempty"`
	Routing      RoutingStyle      `json:"routing,omitempty"`
	Waypoints    []geo.Vec         `json:"waypoints,omitempty"`
	Label        string            `json:"label,omitempty"`
	StartArrow   ArrowStyle        `json:"startArrow,omitempty"`
	EndArrow     ArrowStyle        `json:"endArrow,omitempty"`
	Style        EdgeStyle         `json:"style,omitempty"`
	ZIndex       int               `json:"zIndex,omitempty"`
	Meta         map[string]string `json:"meta,omitempty"`
}

// Page represents one tab/diagram canvas inside a Document.
type Page struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Background string            `json:"background,omitempty"`
	Nodes      []*Node           `json:"nodes"`
	Edges      []*Edge           `json:"edges"`
	Meta       map[string]string `json:"meta,omitempty"`
}

// Document is the root persisted diagram file (.drawgraph / .drawgraph.json).
type Document struct {
	FormatVersion  int                    `json:"formatVersion"`
	Title          string                 `json:"title"`
	ActivePageID   string                 `json:"activePageId"`
	GridSize       float64                `json:"gridSize"`
	SnapToGrid     bool                   `json:"snapToGrid"`
	ThemeMode      string                 `json:"themeMode,omitempty"` // "auto", "light", "dark"
	DefaultRouting RoutingStyle           `json:"defaultRouting,omitempty"`
	StylePreset    string                 `json:"stylePreset,omitempty"`
	Pages          []*Page                `json:"pages"`
	Assets         map[string]*MediaAsset `json:"assets,omitempty"`
	Meta           map[string]string      `json:"meta,omitempty"`
	UpdatedAt      time.Time              `json:"updatedAt"`
}

// NewDocument creates a ready-to-edit document with one blank page.
func NewDocument(title string) *Document {
	if strings.TrimSpace(title) == "" {
		title = "Untitled Diagram"
	}
	return &Document{
		FormatVersion:  FormatVersion,
		Title:          title,
		ActivePageID:   "page-1",
		GridSize:       20,
		SnapToGrid:     true,
		ThemeMode:      "auto",
		DefaultRouting: RouteOrthogonal,
		StylePreset:    "lobe-clean",
		Assets:         map[string]*MediaAsset{},
		UpdatedAt:      time.Now().UTC(),
		Pages: []*Page{
			{
				ID:    "page-1",
				Name:  "Page-1",
				Nodes: []*Node{},
				Edges: []*Edge{},
			},
		},
	}
}

// Clone returns a deep copy of the document suitable for immutable undo snapshots.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	raw, _ := json.Marshal(d)
	var copyDoc Document
	_ = json.Unmarshal(raw, &copyDoc)
	copyDoc.Normalize()
	return &copyDoc
}

// Normalize fills sensible defaults on a loaded or newly constructed Document.
func (d *Document) Normalize() {
	if d.FormatVersion <= 0 {
		d.FormatVersion = FormatVersion
	}
	if strings.TrimSpace(d.Title) == "" {
		d.Title = "Untitled Diagram"
	}
	if d.GridSize <= 0 {
		d.GridSize = 20
	}
	if d.DefaultRouting == "" {
		d.DefaultRouting = RouteOrthogonal
	}
	if len(d.Pages) == 0 {
		d.Pages = []*Page{{ID: "page-1", Name: "Page-1"}}
	}
	if d.Assets == nil {
		d.Assets = map[string]*MediaAsset{}
	}
	hasActive := false
	for i, p := range d.Pages {
		if strings.TrimSpace(p.ID) == "" {
			p.ID = fmt.Sprintf("page-%d", i+1)
		}
		if strings.TrimSpace(p.Name) == "" {
			p.Name = fmt.Sprintf("Page-%d", i+1)
		}
		if p.Nodes == nil {
			p.Nodes = []*Node{}
		}
		if p.Edges == nil {
			p.Edges = []*Edge{}
		}
		for _, n := range p.Nodes {
			NormalizeNode(n)
		}
		for _, e := range p.Edges {
			NormalizeEdge(e, d.DefaultRouting)
		}
		if p.ID == d.ActivePageID {
			hasActive = true
		}
	}
	if !hasActive {
		d.ActivePageID = d.Pages[0].ID
	}
}

// NormalizeNode applies missing defaults to n without overriding user choices.
func NormalizeNode(n *Node) {
	if n == nil {
		return
	}
	if n.Kind == "" {
		n.Kind = KindRounded
	}
	if n.W <= 0 {
		if n.Kind == KindImage || n.Kind == KindVideo {
			n.W = 320
		} else {
			n.W = 160
		}
	}
	if n.H <= 0 {
		if n.Kind == KindImage || n.Kind == KindVideo {
			n.H = 180
		} else {
			n.H = 72
		}
	}
	if n.Media != nil && n.Media.Fit == "" {
		n.Media.Fit = "contain"
	}
	if n.Style.Opacity <= 0 || n.Style.Opacity > 1 {
		n.Style.Opacity = 1
	}
	if n.Style.StrokeWidth <= 0 {
		n.Style.StrokeWidth = 1.5
	}
	if n.Style.FontSize <= 0 {
		n.Style.FontSize = 13
	}
	if n.Style.Dash == "" {
		n.Style.Dash = DashSolid
	}
	if n.Style.Radius == 0 && (n.Kind == KindRounded || n.Kind == KindContainer || n.Kind == KindSwimlane) {
		n.Style.Radius = 10
	}
	if n.Style.Align == "" {
		n.Style.Align = "center"
	}
	if n.Style.VAlign == "" {
		if n.Kind == KindContainer || n.Kind == KindSwimlane {
			n.Style.VAlign = "top"
		} else {
			n.Style.VAlign = "middle"
		}
	}
}

// NormalizeEdge applies missing defaults to e.
func NormalizeEdge(e *Edge, fallbackRouting RoutingStyle) {
	if e == nil {
		return
	}
	if e.Routing == "" {
		if fallbackRouting != "" {
			e.Routing = fallbackRouting
		} else {
			e.Routing = RouteOrthogonal
		}
	}
	if e.SourceAnchor == "" {
		e.SourceAnchor = geo.AnchorAuto
	}
	if e.TargetAnchor == "" {
		e.TargetAnchor = geo.AnchorAuto
	}
	if e.StartArrow == "" {
		e.StartArrow = ArrowNone
	}
	if e.EndArrow == "" {
		e.EndArrow = ArrowClassic
	}
	if e.Style.Opacity <= 0 || e.Style.Opacity > 1 {
		e.Style.Opacity = 1
	}
	if e.Style.StrokeWidth <= 0 {
		e.Style.StrokeWidth = 1.75
	}
	if e.Style.FontSize <= 0 {
		e.Style.FontSize = 12
	}
	if e.Style.Dash == "" {
		e.Style.Dash = DashSolid
	}
}

// ActivePage returns the currently active page (guaranteed non-nil after Normalize).
func (d *Document) ActivePage() *Page {
	d.Normalize()
	for _, p := range d.Pages {
		if p.ID == d.ActivePageID {
			return p
		}
	}
	return d.Pages[0]
}

// PageByID returns the page with the given id, or nil.
func (d *Document) PageByID(id string) *Page {
	for _, p := range d.Pages {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// AddPage appends a new page and optionally switches ActivePageID to it.
func (d *Document) AddPage(id, name string, activate bool) (*Page, error) {
	if strings.TrimSpace(id) == "" {
		id = d.NextPageID()
	}
	if d.PageByID(id) != nil {
		return nil, fmt.Errorf("page %q already exists", id)
	}
	if strings.TrimSpace(name) == "" {
		name = id
	}
	p := &Page{ID: id, Name: name, Nodes: []*Node{}, Edges: []*Edge{}}
	d.Pages = append(d.Pages, p)
	if activate {
		d.ActivePageID = p.ID
	}
	d.Touch()
	return p, nil
}

// NextPageID generates a unique page ID.
func (d *Document) NextPageID() string {
	for i := len(d.Pages) + 1; ; i++ {
		id := fmt.Sprintf("page-%d", i)
		if d.PageByID(id) == nil {
			return id
		}
	}
}

// Touch updates the document modification timestamp.
func (d *Document) Touch() {
	d.UpdatedAt = time.Now().UTC()
}

// NodeByID finds a node on page p by ID.
func (p *Page) NodeByID(id string) *Node {
	for _, n := range p.Nodes {
		if n.ID == id {
			return n
		}
	}
	return nil
}

// EdgeByID finds an edge on page p by ID.
func (p *Page) EdgeByID(id string) *Edge {
	for _, e := range p.Edges {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// NextNodeID generates a unique node ID on page p.
func (p *Page) NextNodeID(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "node"
	}
	for i := len(p.Nodes) + 1; ; i++ {
		id := fmt.Sprintf("%s-%d", prefix, i)
		if p.NodeByID(id) == nil {
			return id
		}
	}
}

// NextEdgeID generates a unique edge ID on page p.
func (p *Page) NextEdgeID(prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "edge"
	}
	for i := len(p.Edges) + 1; ; i++ {
		id := fmt.Sprintf("%s-%d", prefix, i)
		if p.EdgeByID(id) == nil {
			return id
		}
	}
}

// AddNode validates uniqueness, normalizes defaults, and adds n to page p.
func (p *Page) AddNode(n *Node) error {
	if n == nil {
		return fmt.Errorf("nil node")
	}
	if strings.TrimSpace(n.ID) == "" {
		n.ID = p.NextNodeID("node")
	}
	if p.NodeByID(n.ID) != nil {
		return fmt.Errorf("node %q already exists", n.ID)
	}
	if n.ParentID != "" && p.NodeByID(n.ParentID) == nil {
		return fmt.Errorf("parent node %q not found", n.ParentID)
	}
	if n.ZIndex == 0 && len(p.Nodes) > 0 {
		maxZ := 0
		for _, ex := range p.Nodes {
			if ex.ZIndex > maxZ {
				maxZ = ex.ZIndex
			}
		}
		if n.Kind == KindContainer || n.Kind == KindSwimlane {
			n.ZIndex = 0
		} else {
			n.ZIndex = maxZ + 1
		}
	}
	NormalizeNode(n)
	p.Nodes = append(p.Nodes, n)
	return nil
}

// RemoveNode removes node id and any edges attached to it. Children are unparented.
func (p *Page) RemoveNode(id string) bool {
	idx := -1
	for i, n := range p.Nodes {
		if n.ID == id {
			idx = i
			break
		}
	}
	if idx < 0 {
		return false
	}
	p.Nodes = append(p.Nodes[:idx], p.Nodes[idx+1:]...)
	for _, n := range p.Nodes {
		if n.ParentID == id {
			n.ParentID = ""
		}
	}
	kept := p.Edges[:0]
	for _, e := range p.Edges {
		if e.SourceID != id && e.TargetID != id {
			kept = append(kept, e)
		}
	}
	p.Edges = kept
	return true
}

// AddEdge validates source/target existence and adds e to page p.
func (p *Page) AddEdge(e *Edge) error {
	if e == nil {
		return fmt.Errorf("nil edge")
	}
	if strings.TrimSpace(e.ID) == "" {
		e.ID = p.NextEdgeID("edge")
	}
	if p.EdgeByID(e.ID) != nil {
		return fmt.Errorf("edge %q already exists", e.ID)
	}
	if p.NodeByID(e.SourceID) == nil {
		return fmt.Errorf("source node %q not found", e.SourceID)
	}
	if p.NodeByID(e.TargetID) == nil {
		return fmt.Errorf("target node %q not found", e.TargetID)
	}
	NormalizeEdge(e, RouteOrthogonal)
	p.Edges = append(p.Edges, e)
	return nil
}

// RemoveEdge removes edge id from page p.
func (p *Page) RemoveEdge(id string) bool {
	for i, e := range p.Edges {
		if e.ID == id {
			p.Edges = append(p.Edges[:i], p.Edges[i+1:]...)
			return true
		}
	}
	return false
}

// SortedNodes returns a slice of nodes ordered stably by ZIndex (containers first).
func (p *Page) SortedNodes() []*Node {
	out := slices.Clone(p.Nodes)
	sort.SliceStable(out, func(i, j int) bool {
		ci := out[i].Kind == KindContainer || out[i].Kind == KindSwimlane
		cj := out[j].Kind == KindContainer || out[j].Kind == KindSwimlane
		if ci != cj {
			return ci
		}
		if out[i].ZIndex != out[j].ZIndex {
			return out[i].ZIndex < out[j].ZIndex
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// EdgePoints computes the screen/world polyline for e using either explicit waypoints or auto routing.
func (p *Page) EdgePoints(e *Edge) []geo.Vec {
	src := p.NodeByID(e.SourceID)
	dst := p.NodeByID(e.TargetID)
	if src == nil || dst == nil {
		return e.Waypoints
	}
	if len(e.Waypoints) > 0 {
		p0, _ := geo.AnchorPoint(src.Box(), e.SourceAnchor, e.Waypoints[0])
		p1, _ := geo.AnchorPoint(dst.Box(), e.TargetAnchor, e.Waypoints[len(e.Waypoints)-1])
		pts := make([]geo.Vec, 0, len(e.Waypoints)+2)
		pts = append(pts, p0)
		pts = append(pts, e.Waypoints...)
		pts = append(pts, p1)
		return pts
	}
	if e.Routing == RouteStraight || e.Routing == RouteCurved {
		p0, _ := geo.AnchorPoint(src.Box(), e.SourceAnchor, dst.Box().Center())
		p1, _ := geo.AnchorPoint(dst.Box(), e.TargetAnchor, src.Box().Center())
		return []geo.Vec{p0, p1}
	}
	return geo.OrthogonalRoute(src.Box(), dst.Box(), e.SourceAnchor, e.TargetAnchor, 24)
}

// Bounds returns the bounding box covering all nodes and edge waypoints on page p.
func (p *Page) Bounds() (geo.Box, bool) {
	if len(p.Nodes) == 0 {
		return geo.Box{}, false
	}
	b := p.Nodes[0].Box()
	for _, n := range p.Nodes[1:] {
		b = b.Union(n.Box())
	}
	for _, e := range p.Edges {
		for _, pt := range p.EdgePoints(e) {
			b = b.Union(geo.Box{X: pt.X, Y: pt.Y, W: 1, H: 1})
		}
	}
	return b, true
}

// ValidationIssue describes a structural or visual warning in a Document.
type ValidationIssue struct {
	Severity string `json:"severity"` // "error" or "warning"
	PageID   string `json:"pageId"`
	Code     string `json:"code"`
	TargetID string `json:"targetId,omitempty"`
	Message  string `json:"message"`
}

// Validate checks referential integrity, overlapping sibling nodes, and basic geometry.
func (d *Document) Validate() []ValidationIssue {
	var issues []ValidationIssue
	pageIDs := map[string]bool{}
	for _, p := range d.Pages {
		if pageIDs[p.ID] {
			issues = append(issues, ValidationIssue{
				Severity: "error", PageID: p.ID, Code: "duplicate_page_id",
				Message: fmt.Sprintf("duplicate page id %q", p.ID),
			})
		}
		pageIDs[p.ID] = true
		nodeIDs := map[string]bool{}
		for _, n := range p.Nodes {
			if nodeIDs[n.ID] {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "duplicate_node_id", TargetID: n.ID,
					Message: fmt.Sprintf("duplicate node id %q", n.ID),
				})
			}
			nodeIDs[n.ID] = true
			if n.W <= 0 || n.H <= 0 {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "invalid_size", TargetID: n.ID,
					Message: fmt.Sprintf("node %q has non-positive size %gx%g", n.ID, n.W, n.H),
				})
			}
		}
		for _, n := range p.Nodes {
			if n.ParentID != "" && !nodeIDs[n.ParentID] {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "missing_parent", TargetID: n.ID,
					Message: fmt.Sprintf("node %q references missing parent %q", n.ID, n.ParentID),
				})
			}
		}
		// Check non-container overlaps.
		for i := 0; i < len(p.Nodes); i++ {
			a := p.Nodes[i]
			if a.Kind == KindContainer || a.Kind == KindSwimlane {
				continue
			}
			for j := i + 1; j < len(p.Nodes); j++ {
				b := p.Nodes[j]
				if b.Kind == KindContainer || b.Kind == KindSwimlane || a.ParentID != b.ParentID {
					continue
				}
				if a.Box().Overlaps(b.Box()) {
					issues = append(issues, ValidationIssue{
						Severity: "warning", PageID: p.ID, Code: "overlapping_nodes", TargetID: a.ID,
						Message: fmt.Sprintf("node %q overlaps with %q", a.ID, b.ID),
					})
				}
			}
		}
		edgeIDs := map[string]bool{}
		for _, e := range p.Edges {
			if edgeIDs[e.ID] {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "duplicate_edge_id", TargetID: e.ID,
					Message: fmt.Sprintf("duplicate edge id %q", e.ID),
				})
			}
			edgeIDs[e.ID] = true
			if !nodeIDs[e.SourceID] {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "dangling_source", TargetID: e.ID,
					Message: fmt.Sprintf("edge %q source %q does not exist", e.ID, e.SourceID),
				})
			}
			if !nodeIDs[e.TargetID] {
				issues = append(issues, ValidationIssue{
					Severity: "error", PageID: p.ID, Code: "dangling_target", TargetID: e.ID,
					Message: fmt.Sprintf("edge %q target %q does not exist", e.ID, e.TargetID),
				})
			}
		}
	}
	return issues
}

// LoadFile reads a .drawgraph / .drawgraph.json file from disk.
func LoadFile(path string) (*Document, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := MigrateUp(&doc); err != nil {
		return nil, err
	}
	doc.Normalize()
	return &doc, nil
}

// SaveAtomic writes doc as formatted JSON atomically using a temporary file and rename.
func SaveAtomic(path string, doc *Document) error {
	if doc == nil {
		return fmt.Errorf("nil document")
	}
	doc.Normalize()
	doc.Touch()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
