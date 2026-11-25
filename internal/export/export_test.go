package export_test

import (
	"bytes"
	"image/png"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
)

func TestExportPNGAndSVGWithIcons(t *testing.T) {
	reg, err := icons.OpenRegistry(filepath.Join(t.TempDir(), "icons"))
	if err != nil {
		t.Fatal(err)
	}
	doc := graph.NewDocument("Export Test")
	p := doc.ActivePage()
	_ = p.AddNode(&graph.Node{
		ID: "gw", Kind: graph.KindRounded, X: 60, Y: 80, W: 180, H: 76,
		Label: "Gateway", Icon: graph.ParseIconRef("builtin/gateway"),
		Style: graph.NodeStyle{Fill: "#EFF6FF", Stroke: "#3B82F6", Radius: 10, Shadow: true},
	})
	_ = p.AddNode(&graph.Node{
		ID: "db", Kind: graph.KindCylinder, X: 320, Y: 80, W: 160, H: 76,
		Label: "Database", Icon: graph.ParseIconRef("builtin/database"),
	})
	_ = p.AddEdge(&graph.Edge{ID: "e1", SourceID: "gw", TargetID: "db", Label: "gRPC", EndArrow: graph.ArrowClassic})

	for _, dark := range []bool{false, true} {
		pngBytes, err := export.RenderPNG(doc, "", reg, dark, 2)
		if err != nil {
			t.Fatalf("RenderPNG(dark=%v): %v", dark, err)
		}
		img, err := png.Decode(bytes.NewReader(pngBytes))
		if err != nil {
			t.Fatalf("decode PNG: %v", err)
		}
		b := img.Bounds()
		if b.Dx() < 200 || b.Dy() < 150 {
			t.Fatalf("unexpected PNG size: %v", b)
		}
		// Ensure non-uniform pixel content (diagram actually painted).
		c0 := img.At(b.Min.X+2, b.Min.Y+2)
		diff := false
		for y := b.Min.Y; y < b.Max.Y; y += 8 {
			for x := b.Min.X; x < b.Max.X; x += 8 {
				if img.At(x, y) != c0 {
					diff = true
					break
				}
			}
			if diff {
				break
			}
		}
		if !diff {
			t.Fatalf("exported PNG(dark=%v) is completely uniform", dark)
		}
	}

	svgBytes, err := export.RenderSVG(doc, "", reg, false)
	if err != nil {
		t.Fatal(err)
	}
	svgStr := string(svgBytes)
	if !strings.Contains(svgStr, "<svg") || !strings.Contains(svgStr, "Gateway") || !strings.Contains(svgStr, "gRPC") {
		t.Fatalf("SVG missing expected elements:\n%s", svgStr)
	}
}
