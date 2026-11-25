package graph

import (
	"math"
	"slices"

	"github.com/neko233-com/DrawGraph/internal/geo"
)

// History manages immutable document snapshots for unlimited undo/redo.
type History struct {
	Doc   *Document
	undo  []*Document
	redo  []*Document
	limit int
}

// NewHistory initializes an editor history around doc.
func NewHistory(doc *Document) *History {
	if doc == nil {
		doc = NewDocument("Untitled Diagram")
	}
	doc.Normalize()
	return &History{
		Doc:   doc.Clone(),
		limit: 200,
	}
}

// Snapshot saves the current state before applying a mutation.
func (h *History) Snapshot() {
	h.undo = append(h.undo, h.Doc.Clone())
	if len(h.undo) > h.limit {
		h.undo = h.undo[len(h.undo)-h.limit:]
	}
	h.redo = nil
}

// CanUndo reports whether Undo has a previous state.
func (h *History) CanUndo() bool { return len(h.undo) > 0 }

// CanRedo reports whether Redo has a forward state.
func (h *History) CanRedo() bool { return len(h.redo) > 0 }

// Undo reverts Doc to the latest snapshot.
func (h *History) Undo() bool {
	if !h.CanUndo() {
		return false
	}
	h.redo = append(h.redo, h.Doc.Clone())
	h.Doc = h.undo[len(h.undo)-1]
	h.undo = h.undo[:len(h.undo)-1]
	return true
}

// Redo re-applies the latest undone snapshot.
func (h *History) Redo() bool {
	if !h.CanRedo() {
		return false
	}
	h.undo = append(h.undo, h.Doc.Clone())
	h.Doc = h.redo[len(h.redo)-1]
	h.redo = h.redo[:len(h.redo)-1]
	return true
}

// AlignMode specifies an alignment operation across selected nodes.
type AlignMode string

const (
	AlignLeft       AlignMode = "left"
	AlignCenterX    AlignMode = "center-x"
	AlignRight      AlignMode = "right"
	AlignTop        AlignMode = "top"
	AlignCenterY    AlignMode = "center-y"
	AlignBottom     AlignMode = "bottom"
	DistributeHoriz AlignMode = "distribute-h"
	DistributeVert  AlignMode = "distribute-v"
)

// AlignNodes aligns or distributes the nodes matching ids on page p.
// Returns true if at least one node moved.
func AlignNodes(p *Page, ids []string, mode AlignMode) bool {
	var nodes []*Node
	idSet := map[string]bool{}
	for _, id := range ids {
		idSet[id] = true
	}
	for _, n := range p.Nodes {
		if idSet[n.ID] && !n.Locked {
			nodes = append(nodes, n)
		}
	}
	if len(nodes) < 2 {
		return false
	}
	minX, minY := nodes[0].X, nodes[0].Y
	maxX, maxY := nodes[0].X+nodes[0].W, nodes[0].Y+nodes[0].H
	for _, n := range nodes[1:] {
		minX = math.Min(minX, n.X)
		minY = math.Min(minY, n.Y)
		maxX = math.Max(maxX, n.X+n.W)
		maxY = math.Max(maxY, n.Y+n.H)
	}
	changed := false
	switch mode {
	case AlignLeft:
		for _, n := range nodes {
			if n.X != minX {
				n.X = minX
				changed = true
			}
		}
	case AlignRight:
		for _, n := range nodes {
			if target := maxX - n.W; n.X != target {
				n.X = target
				changed = true
			}
		}
	case AlignCenterX:
		mid := (minX + maxX) / 2
		for _, n := range nodes {
			if target := mid - n.W/2; n.X != target {
				n.X = target
				changed = true
			}
		}
	case AlignTop:
		for _, n := range nodes {
			if n.Y != minY {
				n.Y = minY
				changed = true
			}
		}
	case AlignBottom:
		for _, n := range nodes {
			if target := maxY - n.H; n.Y != target {
				n.Y = target
				changed = true
			}
		}
	case AlignCenterY:
		mid := (minY + maxY) / 2
		for _, n := range nodes {
			if target := mid - n.H/2; n.Y != target {
				n.Y = target
				changed = true
			}
		}
	case DistributeHoriz:
		if len(nodes) < 3 {
			return false
		}
		slices.SortFunc(nodes, func(a, b *Node) int {
			if a.X < b.X {
				return -1
			}
			if a.X > b.X {
				return 1
			}
			return 0
		})
		sumW := 0.0
		for _, n := range nodes {
			sumW += n.W
		}
		gap := ((maxX - minX) - sumW) / float64(len(nodes)-1)
		curX := nodes[0].X + nodes[0].W + gap
		for _, n := range nodes[1 : len(nodes)-1] {
			if n.X != curX {
				n.X = curX
				changed = true
			}
			curX += n.W + gap
		}
	case DistributeVert:
		if len(nodes) < 3 {
			return false
		}
		slices.SortFunc(nodes, func(a, b *Node) int {
			if a.Y < b.Y {
				return -1
			}
			if a.Y > b.Y {
				return 1
			}
			return 0
		})
		sumH := 0.0
		for _, n := range nodes {
			sumH += n.H
		}
		gap := ((maxY - minY) - sumH) / float64(len(nodes)-1)
		curY := nodes[0].Y + nodes[0].H + gap
		for _, n := range nodes[1 : len(nodes)-1] {
			if n.Y != curY {
				n.Y = curY
				changed = true
			}
			curY += n.H + gap
		}
	}
	return changed
}

// HitNode returns the topmost node at world coordinate pt, or nil.
func (p *Page) HitNode(pt geo.Vec) *Node {
	sorted := p.SortedNodes()
	for i := len(sorted) - 1; i >= 0; i-- {
		n := sorted[i]
		if n.Box().Contains(pt) {
			return n
		}
	}
	return nil
}

// HitEdge returns the topmost edge within tolerance pixels of pt, or nil.
func (p *Page) HitEdge(pt geo.Vec, tolerance float64) *Edge {
	if tolerance <= 0 {
		tolerance = 8
	}
	for i := len(p.Edges) - 1; i >= 0; i-- {
		e := p.Edges[i]
		pts := p.EdgePoints(e)
		if geo.DistToPolyline(pt, pts) <= tolerance {
			return e
		}
	}
	return nil
}
