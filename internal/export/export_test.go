package export_test

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/media"
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

func TestTiledInfiniteCanvasExportAndEmbeddedHDMedia(t *testing.T) {
	tmp := t.TempDir()
	reg, err := icons.OpenRegistry(filepath.Join(tmp, "icons"))
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 800, 450))
	for y := 0; y < 450; y++ {
		for x := 0; x < 800; x++ {
			img.Set(x, y, color.RGBA{R: 16, G: 185, B: 129, A: 255})
		}
	}
	var pngBuf bytes.Buffer
	_ = png.Encode(&pngBuf, img)
	imgPath := filepath.Join(tmp, "shot.png")
	_ = os.WriteFile(imgPath, pngBuf.Bytes(), 0o644)
	vidPath := filepath.Join(tmp, "clip.mp4")
	_ = os.WriteFile(vidPath, []byte("ftypisom"), 0o644)

	doc := graph.NewDocument("Infinite Board")
	imgAsset, iw, ih, err := media.ImportImageFile(doc, imgPath, 480)
	if err != nil {
		t.Fatal(err)
	}
	vidAsset, vw, vh, err := media.ImportVideoFile(doc, vidPath, 480)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.ActivePage()
	// Place media across a huge infinite canvas spanning negative & positive quadrants (>2400px wide).
	_ = p.AddNode(&graph.Node{
		ID: "pic", Kind: graph.KindImage, X: -900, Y: -400, W: iw, H: ih, Label: "Hero Shot",
		Media: &graph.NodeMedia{AssetID: imgAsset.ID, Fit: "contain", ShowBar: true},
	})
	_ = p.AddNode(&graph.Node{
		ID: "vid", Kind: graph.KindVideo, X: 1400, Y: 900, W: vw, H: vh, Label: "Demo Clip",
		Media: &graph.NodeMedia{AssetID: vidAsset.ID, Fit: "contain", Loop: true, Muted: true},
	})
	_ = p.AddEdge(&graph.Edge{ID: "e1", SourceID: "pic", TargetID: "vid", Label: "Links"})

	outPNG, err := export.RenderPNG(doc, "", reg, false, 1)
	if err != nil {
		t.Fatalf("tiled infinite RenderPNG failed: %v", err)
	}
	decoded, err := png.Decode(bytes.NewReader(outPNG))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds().Dx() < 2000 || decoded.Bounds().Dy() < 1200 {
		t.Fatalf("expected multi-tile infinite canvas PNG >=2000x1200, got %v", decoded.Bounds())
	}

	outSVG, err := export.RenderSVG(doc, "", reg, false)
	if err != nil {
		t.Fatal(err)
	}
	s := string(outSVG)
	if !strings.Contains(s, "<image href=\"data:image/png;base64,") || !strings.Contains(s, "Demo Clip") {
		t.Fatalf("SVG missing embedded HD image or video node:\n%s", s)
	}
}
