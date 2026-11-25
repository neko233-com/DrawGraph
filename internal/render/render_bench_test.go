package render_test

import (
	"fmt"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/render"
)

func buildLargeScene(n int) *graph.Document {
	doc := graph.NewDocument("Perf Scene")
	p := doc.ActivePage()
	cols := 100
	for i := 0; i < n; i++ {
		r, c := i/cols, i%cols
		_ = p.AddNode(&graph.Node{
			ID:    fmt.Sprintf("n-%d", i),
			Kind:  graph.KindRounded,
			X:     float64(c * 240),
			Y:     float64(r * 140),
			W:     160,
			H:     72,
			Label: fmt.Sprintf("Node %d", i),
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

func TestVirtualizedFrustumCullingAndLOD(t *testing.T) {
	doc := buildLargeScene(2000)
	reg, _ := icons.OpenRegistry(t.TempDir())
	var stats render.PaintStats
	img := ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			stats = render.PaintSceneWithStats(p, r, render.SceneState{
				Doc:      doc,
				Page:     doc.ActivePage(),
				Icons:    reg,
				Cam:      render.Camera{X: 0, Y: 0, Zoom: 1},
				ShowGrid: true,
			})
		})
	}, 1200, 800, 1)
	if img == nil || stats.TotalNodes != 2000 || stats.VisibleNodes >= 100 || stats.LODMode != "high" {
		t.Fatalf("expected aggressive viewport culling at 100%% zoom, got stats=%+v", stats)
	}

	// Zoom way out to verify Micro LOD kicks in without error
	ui.Render(func(c *ui.Context) {
		ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
			stats = render.PaintSceneWithStats(p, r, render.SceneState{
				Doc:      doc,
				Page:     doc.ActivePage(),
				Icons:    reg,
				Cam:      render.Camera{X: 0, Y: 0, Zoom: 0.05},
				ShowGrid: true,
			})
		})
	}, 1200, 800, 1)
	if stats.LODMode != "micro" {
		t.Fatalf("expected micro LOD at zoom 0.05, got %+v", stats)
	}
}

func BenchmarkPaint10kNodesVirtualized(b *testing.B) {
	doc := buildLargeScene(10000)
	reg, _ := icons.OpenRegistry(b.TempDir())
	st := render.SceneState{
		Doc:      doc,
		Page:     doc.ActivePage(),
		Icons:    reg,
		Cam:      render.Camera{X: -2000, Y: -1000, Zoom: 1},
		ShowGrid: true,
	}
	_ = doc.ActivePage().EnsureIndex()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ui.Render(func(c *ui.Context) {
			ui.Box(c).Fill().Draw(func(p *ui.Painter, r ui.Rect) {
				render.PaintScene(p, r, st)
			})
		}, 1200, 800, 1)
	}
}
