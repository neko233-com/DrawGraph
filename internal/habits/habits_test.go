package habits_test

import (
	"path/filepath"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
)

func TestPersistentCrossProjectHabitsAndLearning(t *testing.T) {
	path := filepath.Join(t.TempDir(), "habits.json")
	s, err := habits.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}

	s.RecordFormatHabit("TB", graph.RouteCurved, "blueprint", 96, 72, 16)
	s.RecordNodeHabit(&graph.Node{
		Kind: graph.KindHexagon,
		Icon: graph.ParseIconRef("builtin/gateway"),
		Style: graph.NodeStyle{
			Fill: "#EFF6FF", Stroke: "#3B82F6", StrokeWidth: 2, Radius: 14, FontSize: 14, Shadow: false,
		},
	})
	s.RecordIconUse("builtin/gateway")
	s.RecordRecentFile("/proj-a/arch.drawgraph")

	// Simulate launching a brand-new project in another session.
	s2, err := habits.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p := s2.Snapshot()
	if p.PreferredDirection != "TB" || p.StylePreset != "blueprint" || p.DefaultRouting != graph.RouteCurved {
		t.Fatalf("persisted format habits lost: %+v", p)
	}
	if p.DefaultNodeKind != graph.KindHexagon || p.DefaultFill != "#EFF6FF" || p.DefaultRadius != 14 {
		t.Fatalf("persisted node habits lost: %+v", p)
	}
	if len(p.FavoriteIcons) == 0 || p.FavoriteIcons[0] != "builtin/gateway" {
		t.Fatalf("expected favorite icon builtin/gateway first, got %v", p.FavoriteIcons)
	}

	newDoc := graph.NewDocument("Project B")
	s2.ApplyToDocument(newDoc)
	if newDoc.GridSize != 16 || newDoc.StylePreset != "blueprint" || newDoc.DefaultRouting != graph.RouteCurved {
		t.Fatalf("new project did not inherit persisted habits: %+v", newDoc)
	}

	// Disable AutoRemember and ensure style tweaks no longer overwrite user defaults.
	if err := s2.Update(func(pr *habits.Profile) { pr.AutoRemember = false }); err != nil {
		t.Fatal(err)
	}
	s2.RecordFormatHabit("LR", graph.RouteStraight, "slate-mono", 40, 40, 10)
	if s2.Snapshot().PreferredDirection != "TB" {
		t.Fatal("AutoRemember=false should not overwrite PreferredDirection")
	}

	if err := s2.Reset(); err != nil {
		t.Fatal(err)
	}
	if s2.Snapshot().PreferredDirection != "LR" {
		t.Fatal("Reset did not restore default LR")
	}

	// Verify language, layout strategy, and wheel mode persist cleanly across instances.
	if err := s2.Update(func(pr *habits.Profile) {
		pr.Language = "zh-CN"
		pr.DefaultStrategy = "tree"
		pr.WheelMode = "zoom"
	}); err != nil {
		t.Fatal(err)
	}
	s3, err := habits.OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	p3 := s3.Snapshot()
	if p3.Language != "zh-CN" || p3.DefaultStrategy != "tree" || p3.WheelMode != "zoom" {
		t.Fatalf("expected persisted language/strategy/wheel, got %+v", p3)
	}
}
