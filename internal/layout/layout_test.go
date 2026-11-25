package layout_test

import (
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/layout"
)

func TestOneClickFormatArrangesDAGAndContainersCleanly(t *testing.T) {
	doc := graph.NewDocument("Pipeline")
	p := doc.ActivePage()

	// Intentionally stacked overlapping mess at (0,0)
	_ = p.AddNode(&graph.Node{ID: "vpc", Kind: graph.KindContainer, X: 0, Y: 0, W: 50, H: 50, Label: "Cloud"})
	_ = p.AddNode(&graph.Node{ID: "client", Kind: graph.KindRounded, X: 0, Y: 0, W: 120, H: 60, Label: "Browser"})
	_ = p.AddNode(&graph.Node{ID: "gw", Kind: graph.KindRounded, ParentID: "vpc", X: 0, Y: 0, W: 120, H: 60, Label: "API Gateway"})
	_ = p.AddNode(&graph.Node{ID: "auth", Kind: graph.KindRounded, ParentID: "vpc", X: 0, Y: 0, W: 120, H: 60, Label: "Auth Service"})
	_ = p.AddNode(&graph.Node{ID: "db", Kind: graph.KindCylinder, ParentID: "vpc", X: 0, Y: 0, W: 120, H: 60, Label: "Orders Database"})
	_ = p.AddEdge(&graph.Edge{ID: "e1", SourceID: "client", TargetID: "gw"})
	_ = p.AddEdge(&graph.Edge{ID: "e2", SourceID: "gw", TargetID: "auth"})
	_ = p.AddEdge(&graph.Edge{ID: "e3", SourceID: "gw", TargetID: "db"})
	// Cycle back-edge to ensure cycle resilience
	_ = p.AddEdge(&graph.Edge{ID: "e4", SourceID: "auth", TargetID: "gw"})

	opts := layout.OptionsFromHabits(habits.DefaultProfile(), false)
	opts.Direction = "LR"
	rep := layout.FormatPage(doc, p, opts)

	if rep.Layers < 3 || rep.ReroutedEdges != 4 || rep.ResizedContainers != 1 {
		t.Fatalf("unexpected format report: %+v", rep)
	}
	// Verify left-to-right monotonic layers: client.X < gw.X < db.X
	c := p.NodeByID("client")
	gw := p.NodeByID("gw")
	db := p.NodeByID("db")
	if !(c.X < gw.X && gw.X < db.X) {
		t.Fatalf("LR order broken: client=%v gw=%v db=%v", c.X, gw.X, db.X)
	}
	// Verify zero overlapping siblings
	for _, is := range doc.Validate() {
		if is.Code == "overlapping_nodes" {
			t.Fatalf("nodes still overlap after FormatPage: %+v", is)
		}
	}
	// Verify container wraps its children cleanly
	vpc := p.NodeByID("vpc")
	for _, cid := range []string{"gw", "auth", "db"} {
		cn := p.NodeByID(cid)
		if cn.X < vpc.X || cn.Y < vpc.Y || cn.X+cn.W > vpc.X+vpc.W || cn.Y+cn.H > vpc.Y+vpc.H {
			t.Fatalf("child %s (%+v) outside container vpc (%+v)", cid, cn.Box(), vpc.Box())
		}
	}
}

func TestAllRegisteredLayoutStrategiesWithoutOverlaps(t *testing.T) {
	strategies := layout.RegisteredStrategies()
	if len(strategies) < 6 {
		t.Fatalf("expected >=6 registered strategies, got %v", strategies)
	}
	for _, strat := range []string{"layered", "tree", "grid", "radial", "swimlane", "compact"} {
		doc := graph.NewDocument("Strategy " + strat)
		p := doc.ActivePage()
		_ = p.AddNode(&graph.Node{ID: "lane1", Kind: graph.KindSwimlane, Label: "Frontend"})
		_ = p.AddNode(&graph.Node{ID: "lane2", Kind: graph.KindSwimlane, Label: "Backend"})
		_ = p.AddNode(&graph.Node{ID: "a", Kind: graph.KindRounded, ParentID: "lane1", X: 100, Y: 100, Label: "UI"})
		_ = p.AddNode(&graph.Node{ID: "b", Kind: graph.KindRounded, ParentID: "lane2", X: 110, Y: 105, Label: "Gateway"})
		_ = p.AddNode(&graph.Node{ID: "c", Kind: graph.KindCylinder, ParentID: "lane2", X: 120, Y: 110, Label: "Database"})
		_ = p.AddNode(&graph.Node{ID: "d", Kind: graph.KindRounded, ParentID: "lane2", X: 125, Y: 115, Label: "Cache"})
		_ = p.AddEdge(&graph.Edge{ID: "e1", SourceID: "a", TargetID: "b"})
		_ = p.AddEdge(&graph.Edge{ID: "e2", SourceID: "b", TargetID: "c"})
		_ = p.AddEdge(&graph.Edge{ID: "e3", SourceID: "b", TargetID: "d"})

		opts := layout.OptionsFromHabits(habits.DefaultProfile(), false)
		opts.Strategy = strat
		rep := layout.FormatPage(doc, p, opts)
		if rep.Strategy != strat {
			t.Fatalf("strategy %s report mismatch: %+v", strat, rep)
		}
		for _, is := range doc.Validate() {
			if is.Code == "overlapping_nodes" {
				t.Fatalf("strategy %s still left overlapping siblings: %+v", strat, is)
			}
		}
	}
}
