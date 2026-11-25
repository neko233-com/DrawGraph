package graph_test

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

func TestDocumentHistoryAndPersistence(t *testing.T) {
	doc := graph.NewDocument("Service Mesh")
	h := graph.NewHistory(doc)
	p := h.Doc.ActivePage()

	h.Snapshot()
	if err := p.AddNode(&graph.Node{ID: "a", Kind: graph.KindRounded, X: 40, Y: 40, W: 160, H: 72, Label: "Gateway"}); err != nil {
		t.Fatal(err)
	}
	if err := p.AddNode(&graph.Node{ID: "b", Kind: graph.KindCylinder, X: 300, Y: 40, W: 160, H: 72, Label: "Postgres"}); err != nil {
		t.Fatal(err)
	}
	if err := p.AddEdge(&graph.Edge{ID: "e1", SourceID: "a", TargetID: "b", Label: "SQL"}); err != nil {
		t.Fatal(err)
	}

	if !h.Undo() {
		t.Fatal("expected Undo to succeed")
	}
	if len(h.Doc.ActivePage().Nodes) != 0 {
		t.Fatalf("expected 0 nodes after Undo, got %d", len(h.Doc.ActivePage().Nodes))
	}
	if !h.Redo() {
		t.Fatal("expected Redo to succeed")
	}
	if len(h.Doc.ActivePage().Nodes) != 2 || len(h.Doc.ActivePage().Edges) != 1 {
		t.Fatalf("unexpected counts after Redo")
	}

	out := filepath.Join(t.TempDir(), "mesh.drawgraph")
	if err := graph.SaveAtomic(out, h.Doc); err != nil {
		t.Fatal(err)
	}
	loaded, err := graph.LoadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Title != "Service Mesh" || len(loaded.ActivePage().Edges) != 1 {
		t.Fatalf("loaded mismatch: %+v", loaded)
	}
}

func TestAlignDistributeAndValidate(t *testing.T) {
	doc := graph.NewDocument("Align")
	p := doc.ActivePage()
	_ = p.AddNode(&graph.Node{ID: "n1", X: 100, Y: 50, W: 100, H: 60})
	_ = p.AddNode(&graph.Node{ID: "n2", X: 250, Y: 90, W: 100, H: 60})
	_ = p.AddNode(&graph.Node{ID: "n3", X: 500, Y: 120, W: 100, H: 60})

	if !graph.AlignNodes(p, []string{"n1", "n2", "n3"}, graph.AlignTop) {
		t.Fatal("AlignTop did not report changes")
	}
	for _, n := range p.Nodes {
		if n.Y != 50 {
			t.Fatalf("node %s Y=%v, want 50", n.ID, n.Y)
		}
	}
	if !graph.AlignNodes(p, []string{"n1", "n2", "n3"}, graph.DistributeHoriz) {
		t.Fatal("DistributeHoriz did not report changes")
	}
	if p.NodeByID("n2").X != 300 {
		t.Fatalf("n2.X = %v, want 300", p.NodeByID("n2").X)
	}

	// Overlapping & dangling validation
	_ = p.AddNode(&graph.Node{ID: "n4", X: 110, Y: 55, W: 100, H: 60})
	p.Edges = append(p.Edges, &graph.Edge{ID: "bad", SourceID: "n1", TargetID: "missing"})
	issues := doc.Validate()
	var sawOverlap, sawDangling bool
	for _, is := range issues {
		if is.Code == "overlapping_nodes" {
			sawOverlap = true
		}
		if is.Code == "dangling_target" {
			sawDangling = true
		}
	}
	if !sawOverlap || !sawDangling {
		t.Fatalf("expected overlap and dangling issues, got %+v", issues)
	}

	// RemoveNode cascades attached edges cleanly
	_ = p.AddEdge(&graph.Edge{ID: "good", SourceID: "n1", TargetID: "n3"})
	if !p.RemoveNode("n1") {
		t.Fatal("RemoveNode failed")
	}
	if p.EdgeByID("good") != nil || p.EdgeByID("bad") != nil {
		t.Fatal("expected attached edges to be removed")
	}
	_ = geo.V
}
