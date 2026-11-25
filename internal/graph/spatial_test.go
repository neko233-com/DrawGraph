package graph_test

import (
	"fmt"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

func buildGridDoc(n int) *graph.Document {
	doc := graph.NewDocument("Large Virtual Canvas")
	p := doc.ActivePage()
	cols := 100
	for i := 0; i < n; i++ {
		r, c := i/cols, i%cols
		_ = p.AddNode(&graph.Node{
			ID:    fmt.Sprintf("n-%d", i),
			Kind:  graph.KindRounded,
			X:     float64(c * 220),
			Y:     float64(r * 120),
			W:     160,
			H:     72,
			Label: fmt.Sprintf("Service %d", i),
		})
		if i > 0 && c > 0 {
			_ = p.AddEdge(&graph.Edge{
				ID:       fmt.Sprintf("e-%d", i),
				SourceID: fmt.Sprintf("n-%d", i-1),
				TargetID: fmt.Sprintf("n-%d", i),
			})
		}
	}
	p.MarkDirty()
	return doc
}

func TestSpatialIndexFrustumHitSnapGroupAndZOrder(t *testing.T) {
	doc := buildGridDoc(2000)
	p := doc.ActivePage()
	idx := p.EnsureIndex()

	// Viewport at top-left should cull nearly all 2000 nodes and return only nearby ones.
	vis := idx.QueryRect(geo.B(0, 0, 500, 300))
	if len(vis) == 0 || len(vis) > 15 {
		t.Fatalf("expected frustum cull to return 1..15 nodes, got %d", len(vis))
	}
	visEdges := idx.QueryEdges(p, geo.B(0, 0, 500, 300))
	if len(visEdges) == 0 || len(visEdges) > 15 {
		t.Fatalf("expected frustum cull for edges, got %d", len(visEdges))
	}

	// O(1) HitNode & HitEdge in deep negative/positive coordinates
	hit := p.HitNode(geo.V(220*50+40, 120*10+30))
	if hit == nil || hit.ID != "n-1050" {
		t.Fatalf("expected HitNode n-1050, got %+v", hit)
	}

	// Smart snap alignment guides near n-0 (X=0, Y=0, W=160, H=72)
	dx, dy, guides := idx.FindSnapGuides(map[string]bool{"moving": true}, geo.B(3, 500, 160, 72), 6)
	if dx != -3 || dy != 0 || len(guides) != 1 || guides[0].Axis != "x" {
		t.Fatalf("expected X left snap -3 with 1 guide, got dx=%v dy=%v guides=%+v", dx, dy, guides)
	}

	// Group, reorder ZIndex, ungroup cleanly
	grp, err := graph.GroupNodes(p, []string{"n-0", "n-1"}, "Edge Cluster")
	if err != nil || grp == nil {
		t.Fatalf("GroupNodes failed: %v", err)
	}
	if p.NodeByID("n-0").ParentID != grp.ID {
		t.Fatalf("expected n-0 grouped under %s", grp.ID)
	}
	if !graph.ReorderNodes(p, []string{"n-0"}, graph.ZOrderFront) {
		t.Fatal("expected ReorderNodes to update ZIndex")
	}
	if !graph.UngroupNodes(p, []string{grp.ID}) || p.NodeByID(grp.ID) != nil || p.NodeByID("n-0").ParentID != "" {
		t.Fatal("UngroupNodes did not dissolve container and unparent children")
	}
}

func BenchmarkSpatialQuery10kNodes(b *testing.B) {
	doc := buildGridDoc(10000)
	p := doc.ActivePage()
	idx := p.EnsureIndex()
	view := geo.B(4400, 2400, 1400, 900)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.QueryRect(view)
		_ = idx.QueryEdges(p, view)
	}
}
