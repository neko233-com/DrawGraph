package graph

import (
	"math"
	"slices"
	"sort"

	"github.com/neko233-com/DrawGraph/internal/geo"
)

const spatialCellSize = 512.0

// VisibleEdge bundles an Edge with precomputed world polyline and parallel lane slot.
type VisibleEdge struct {
	Edge      *Edge
	Points    []geo.Vec
	PairSlot  int
	PairTotal int
}

// SpatialIndex accelerates frustum queries, hit testing, and smart snap alignment
// over arbitrarily large infinite canvases without sorting or scanning all nodes per frame.
type SpatialIndex struct {
	rev       uint64
	nodeLen   int
	edgeLen   int
	bounds    geo.Box
	hasBounds bool

	nodeMap        map[string]*Node
	sortedNodes    []*Node
	nodeRank       map[string]int
	nodeCells      map[uint64][]int
	oversizedNodes []int

	edgeBoxes      []geo.Box
	edgePts        [][]geo.Vec
	edgePairSlot   []int
	edgePairTotal  []int
	edgeCells      map[uint64][]int
	oversizedEdges []int
}

func packCellKey(cx, cy int32) uint64 {
	return (uint64(uint32(cx)) << 32) | uint64(uint32(cy))
}

func cellRange(b geo.Box) (int32, int32, int32, int32) {
	norm := b.Normalized()
	minX := int32(math.Floor(norm.X / spatialCellSize))
	minY := int32(math.Floor(norm.Y / spatialCellSize))
	maxX := int32(math.Floor(norm.MaxX() / spatialCellSize))
	maxY := int32(math.Floor(norm.MaxY() / spatialCellSize))
	return minX, minY, maxX, maxY
}

// MarkDirty invalidates cached spatial index / polyline state after coordinate or graph edits.
func (p *Page) MarkDirty() {
	if p == nil {
		return
	}
	p.rev++
}

// Revision returns the page's monotonic mutation counter.
func (p *Page) Revision() uint64 {
	if p == nil {
		return 0
	}
	return p.rev
}

// EnsureIndex returns an up-to-date SpatialIndex for p, rebuilding only when mutated.
func (p *Page) EnsureIndex() *SpatialIndex {
	if p == nil {
		return nil
	}
	if p.idx != nil && p.idx.rev == p.rev && p.idx.nodeLen == len(p.Nodes) && p.idx.edgeLen == len(p.Edges) {
		return p.idx
	}
	idx := buildSpatialIndex(p)
	p.idx = idx
	return idx
}

func buildSpatialIndex(p *Page) *SpatialIndex {
	sorted := slices.Clone(p.Nodes)
	sort.SliceStable(sorted, func(i, j int) bool {
		ci := sorted[i].Kind == KindContainer || sorted[i].Kind == KindSwimlane
		cj := sorted[j].Kind == KindContainer || sorted[j].Kind == KindSwimlane
		if ci != cj {
			return ci
		}
		if sorted[i].ZIndex != sorted[j].ZIndex {
			return sorted[i].ZIndex < sorted[j].ZIndex
		}
		return sorted[i].ID < sorted[j].ID
	})

	idx := &SpatialIndex{
		rev:         p.rev,
		nodeLen:     len(p.Nodes),
		edgeLen:     len(p.Edges),
		nodeMap:     make(map[string]*Node, len(p.Nodes)),
		sortedNodes: sorted,
		nodeRank:    make(map[string]int, len(p.Nodes)),
		nodeCells:   make(map[uint64][]int, len(p.Nodes)),
		edgeCells:   make(map[uint64][]int, len(p.Edges)),
	}

	var wb geo.Box
	for i, n := range sorted {
		idx.nodeMap[n.ID] = n
		idx.nodeRank[n.ID] = i
		nb := n.Box()
		if !idx.hasBounds {
			wb = nb
			idx.hasBounds = true
		} else {
			wb = wb.Union(nb)
		}
		minX, minY, maxX, maxY := cellRange(nb)
		spanCells := int64(maxX-minX+1) * int64(maxY-minY+1)
		if spanCells > 64 {
			idx.oversizedNodes = append(idx.oversizedNodes, i)
			continue
		}
		for cy := minY; cy <= maxY; cy++ {
			for cx := minX; cx <= maxX; cx++ {
				key := packCellKey(cx, cy)
				idx.nodeCells[key] = append(idx.nodeCells[key], i)
			}
		}
	}

	idx.edgeBoxes = make([]geo.Box, len(p.Edges))
	idx.edgePts = make([][]geo.Vec, len(p.Edges))
	idx.edgePairSlot = make([]int, len(p.Edges))
	idx.edgePairTotal = make([]int, len(p.Edges))

	pairCounts := make(map[[2]string]int, len(p.Edges))
	edgeKeys := make([][2]string, len(p.Edges))
	for i, e := range p.Edges {
		a, b := e.SourceID, e.TargetID
		if a > b {
			a, b = b, a
		}
		k := [2]string{a, b}
		edgeKeys[i] = k
		idx.edgePairSlot[i] = pairCounts[k]
		pairCounts[k]++
	}

	for i, e := range p.Edges {
		idx.edgePairTotal[i] = pairCounts[edgeKeys[i]]
		pts := computeEdgePoints(e, idx.nodeMap)
		idx.edgePts[i] = pts
		if len(pts) == 0 {
			continue
		}
		eb := geo.Box{X: pts[0].X, Y: pts[0].Y, W: 1, H: 1}
		for _, pt := range pts[1:] {
			eb = eb.Union(geo.Box{X: pt.X, Y: pt.Y, W: 1, H: 1})
		}
		eb = eb.Expand(12)
		idx.edgeBoxes[i] = eb
		if idx.hasBounds {
			wb = wb.Union(eb)
		}
		minX, minY, maxX, maxY := cellRange(eb)
		spanCells := int64(maxX-minX+1) * int64(maxY-minY+1)
		if spanCells > 64 {
			idx.oversizedEdges = append(idx.oversizedEdges, i)
			continue
		}
		for cy := minY; cy <= maxY; cy++ {
			for cx := minX; cx <= maxX; cx++ {
				key := packCellKey(cx, cy)
				idx.edgeCells[key] = append(idx.edgeCells[key], i)
			}
		}
	}
	if idx.hasBounds {
		idx.bounds = wb
	}
	return idx
}

func computeEdgePoints(e *Edge, nodeMap map[string]*Node) []geo.Vec {
	src := nodeMap[e.SourceID]
	dst := nodeMap[e.TargetID]
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

// NodeByIDFast looks up node id via the spatial index map in O(1).
func (idx *SpatialIndex) NodeByIDFast(id string) *Node {
	if idx == nil {
		return nil
	}
	return idx.nodeMap[id]
}

// Sorted returns pre-sorted nodes (containers first, then ascending ZIndex).
func (idx *SpatialIndex) Sorted() []*Node {
	if idx == nil {
		return nil
	}
	return idx.sortedNodes
}

// EdgePointsByIndex returns precomputed world points for edge index i.
func (idx *SpatialIndex) EdgePointsByIndex(i int) []geo.Vec {
	if idx == nil || i < 0 || i >= len(idx.edgePts) {
		return nil
	}
	return idx.edgePts[i]
}

// QueryRect returns nodes intersecting view sorted in correct draw order (containers first, then ZIndex).
// Only O(visible + oversized) nodes are examined on large infinite canvases.
func (idx *SpatialIndex) QueryRect(view geo.Box) []*Node {
	if idx == nil || len(idx.sortedNodes) == 0 {
		return nil
	}
	norm := view.Normalized()
	minX, minY, maxX, maxY := cellRange(norm)
	spanCells := int64(maxX-minX+1) * int64(maxY-minY+1)
	if spanCells > 2048 || len(idx.sortedNodes) <= 64 {
		out := make([]*Node, 0, min(len(idx.sortedNodes), 128))
		for _, n := range idx.sortedNodes {
			if norm.Overlaps(n.Box()) {
				out = append(out, n)
			}
		}
		return out
	}

	seen := make([]bool, len(idx.sortedNodes))
	indices := make([]int, 0, 64)
	for _, ni := range idx.oversizedNodes {
		if norm.Overlaps(idx.sortedNodes[ni].Box()) {
			seen[ni] = true
			indices = append(indices, ni)
		}
	}
	for cy := minY; cy <= maxY; cy++ {
		for cx := minX; cx <= maxX; cx++ {
			for _, ni := range idx.nodeCells[packCellKey(cx, cy)] {
				if seen[ni] {
					continue
				}
				seen[ni] = true
				if norm.Overlaps(idx.sortedNodes[ni].Box()) {
					indices = append(indices, ni)
				}
			}
		}
	}
	slices.Sort(indices)
	out := make([]*Node, len(indices))
	for i, ni := range indices {
		out[i] = idx.sortedNodes[ni]
	}
	return out
}

// QueryEdges returns visible edges (with precomputed polylines and parallel slots) overlapping view.
func (idx *SpatialIndex) QueryEdges(p *Page, view geo.Box) []VisibleEdge {
	if idx == nil || len(p.Edges) == 0 {
		return nil
	}
	norm := view.Normalized()
	minX, minY, maxX, maxY := cellRange(norm)
	spanCells := int64(maxX-minX+1) * int64(maxY-minY+1)
	if spanCells > 2048 || len(p.Edges) <= 64 {
		out := make([]VisibleEdge, 0, min(len(p.Edges), 128))
		for i, e := range p.Edges {
			if len(idx.edgePts[i]) >= 2 && norm.Overlaps(idx.edgeBoxes[i]) {
				out = append(out, VisibleEdge{
					Edge:      e,
					Points:    idx.edgePts[i],
					PairSlot:  idx.edgePairSlot[i],
					PairTotal: idx.edgePairTotal[i],
				})
			}
		}
		return out
	}

	seen := make([]bool, len(p.Edges))
	indices := make([]int, 0, 64)
	for _, ei := range idx.oversizedEdges {
		if len(idx.edgePts[ei]) >= 2 && norm.Overlaps(idx.edgeBoxes[ei]) {
			seen[ei] = true
			indices = append(indices, ei)
		}
	}
	for cy := minY; cy <= maxY; cy++ {
		for cx := minX; cx <= maxX; cx++ {
			for _, ei := range idx.edgeCells[packCellKey(cx, cy)] {
				if seen[ei] {
					continue
				}
				seen[ei] = true
				if len(idx.edgePts[ei]) >= 2 && norm.Overlaps(idx.edgeBoxes[ei]) {
					indices = append(indices, ei)
				}
			}
		}
	}
	slices.Sort(indices)
	out := make([]VisibleEdge, len(indices))
	for i, ei := range indices {
		out[i] = VisibleEdge{
			Edge:      p.Edges[ei],
			Points:    idx.edgePts[ei],
			PairSlot:  idx.edgePairSlot[ei],
			PairTotal: idx.edgePairTotal[ei],
		}
	}
	return out
}

// HitPoint returns the topmost node containing pt in O(1) expected time.
func (idx *SpatialIndex) HitPoint(pt geo.Vec) *Node {
	if idx == nil || len(idx.sortedNodes) == 0 {
		return nil
	}
	if len(idx.sortedNodes) <= 48 {
		for i := len(idx.sortedNodes) - 1; i >= 0; i-- {
			n := idx.sortedNodes[i]
			if n.Box().Contains(pt) {
				return n
			}
		}
		return nil
	}
	cx := int32(math.Floor(pt.X / spatialCellSize))
	cy := int32(math.Floor(pt.Y / spatialCellSize))
	bestIdx := -1
	for _, ni := range idx.oversizedNodes {
		if ni > bestIdx && idx.sortedNodes[ni].Box().Contains(pt) {
			bestIdx = ni
		}
	}
	for _, ni := range idx.nodeCells[packCellKey(cx, cy)] {
		if ni > bestIdx && idx.sortedNodes[ni].Box().Contains(pt) {
			bestIdx = ni
		}
	}
	if bestIdx >= 0 {
		return idx.sortedNodes[bestIdx]
	}
	return nil
}

// HitEdgePoint returns the topmost edge within tol world units of pt using spatial cells.
func (idx *SpatialIndex) HitEdgePoint(p *Page, pt geo.Vec, tol float64) *Edge {
	if idx == nil || len(p.Edges) == 0 {
		return nil
	}
	if tol <= 0 {
		tol = 8
	}
	if len(p.Edges) <= 48 {
		for i := len(p.Edges) - 1; i >= 0; i-- {
			if geo.DistToPolyline(pt, idx.edgePts[i]) <= tol {
				return p.Edges[i]
			}
		}
		return nil
	}
	query := geo.B(pt.X-tol, pt.Y-tol, tol*2, tol*2)
	minX, minY, maxX, maxY := cellRange(query)
	bestIdx := -1
	for _, ei := range idx.oversizedEdges {
		if ei > bestIdx && idx.edgeBoxes[ei].Overlaps(query) && geo.DistToPolyline(pt, idx.edgePts[ei]) <= tol {
			bestIdx = ei
		}
	}
	for cy := minY; cy <= maxY; cy++ {
		for cx := minX; cx <= maxX; cx++ {
			for _, ei := range idx.edgeCells[packCellKey(cx, cy)] {
				if ei > bestIdx && idx.edgeBoxes[ei].Overlaps(query) && geo.DistToPolyline(pt, idx.edgePts[ei]) <= tol {
					bestIdx = ei
				}
			}
		}
	}
	if bestIdx >= 0 {
		return p.Edges[bestIdx]
	}
	return nil
}

// SnapGuide represents an active vertical or horizontal smart alignment guide line.
type SnapGuide struct {
	Axis  string  `json:"axis"` // "x" or "y"
	Pos   float64 `json:"pos"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// FindSnapGuides snaps candidate movingBox against nearby static nodes queried from SpatialIndex.
func (idx *SpatialIndex) FindSnapGuides(moving map[string]bool, movingBox geo.Box, tol float64) (dx, dy float64, guides []SnapGuide) {
	if idx == nil || tol <= 0 {
		return 0, 0, nil
	}
	nearby := idx.QueryRect(movingBox.Expand(640))
	bestDX, bestDY := tol+1, tol+1
	var gx, gy SnapGuide
	hasX, hasY := false, false

	mLeft, mMidX, mRight := movingBox.X, movingBox.X+movingBox.W/2, movingBox.MaxX()
	mTop, mMidY, mBot := movingBox.Y, movingBox.Y+movingBox.H/2, movingBox.MaxY()

	for _, other := range nearby {
		if moving[other.ID] || other.Kind == KindContainer || other.Kind == KindSwimlane {
			continue
		}
		ob := other.Box()
		oXs := [3]float64{ob.X, ob.X + ob.W/2, ob.MaxX()}
		mXs := [3]float64{mLeft, mMidX, mRight}
		for _, mx := range mXs {
			for _, ox := range oXs {
				diff := ox - mx
				if math.Abs(diff) < math.Abs(bestDX) && math.Abs(diff) <= tol {
					bestDX = diff
					gx = SnapGuide{
						Axis:  "x",
						Pos:   ox,
						Start: math.Min(movingBox.Y, ob.Y) - 24,
						End:   math.Max(movingBox.MaxY(), ob.MaxY()) + 24,
					}
					hasX = true
				}
			}
		}

		oYs := [3]float64{ob.Y, ob.Y + ob.H/2, ob.MaxY()}
		mYs := [3]float64{mTop, mMidY, mBot}
		for _, my := range mYs {
			for _, oy := range oYs {
				diff := oy - my
				if math.Abs(diff) < math.Abs(bestDY) && math.Abs(diff) <= tol {
					bestDY = diff
					gy = SnapGuide{
						Axis:  "y",
						Pos:   oy,
						Start: math.Min(movingBox.X, ob.X) - 24,
						End:   math.Max(movingBox.MaxX(), ob.MaxX()) + 24,
					}
					hasY = true
				}
			}
		}
	}
	if hasX {
		dx = bestDX
		guides = append(guides, gx)
	}
	if hasY {
		dy = bestDY
		guides = append(guides, gy)
	}
	return dx, dy, guides
}
