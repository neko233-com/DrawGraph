package media_test

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/media"
)

func TestFitDisplaySizePreservesAspectRatio(t *testing.T) {
	w, h := media.FitDisplaySize(3840, 2160, 480, 96)
	if w != 480 || h != 270 {
		t.Fatalf("4K 16:9 fit = (%v,%v), want (480,270)", w, h)
	}
	w, h = media.FitDisplaySize(1080, 1920, 480, 96)
	if w != 270 || h != 480 {
		t.Fatalf("vertical 9:16 fit = (%v,%v), want (270,480)", w, h)
	}
	w, h = media.FitDisplaySize(240, 180, 480, 96)
	if w != 240 || h != 180 {
		t.Fatalf("medium 1:1 = (%v,%v), want (240,180)", w, h)
	}
	w, h = media.FitDisplaySize(32, 32, 480, 96)
	if w != 96 || h != 96 {
		t.Fatalf("tiny min clamp = (%v,%v), want (96,96)", w, h)
	}
}

func TestImportHDImagePreservesFullResolutionAndDeduplicates(t *testing.T) {
	tmp := t.TempDir()
	imgPath := filepath.Join(tmp, "hero-1080p.png")
	src := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	for y := 0; y < 1080; y += 20 {
		for x := 0; x < 1920; x += 20 {
			src.Set(x, y, color.RGBA{R: 37, G: 99, B: 235, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(imgPath, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}

	doc := graph.NewDocument("Media")
	a1, w, h, err := media.ImportImageFile(doc, imgPath, 480)
	if err != nil {
		t.Fatal(err)
	}
	if a1.NaturalW != 1920 || a1.NaturalH != 1080 {
		t.Fatalf("expected natural 1920x1080, got %dx%d", a1.NaturalW, a1.NaturalH)
	}
	if w != 480 || h != 270 {
		t.Fatalf("expected fitted canvas 480x270, got %gx%g", w, h)
	}

	// Backing decoded image retains full 1920x1080 HD pixels.
	decoded, err := media.DecodeAssetImage(a1)
	if err != nil {
		t.Fatal(err)
	}
	bmp, ok := decoded.(*ui.Bitmap)
	if !ok {
		t.Fatalf("expected *ui.Bitmap, got %T", decoded)
	}
	bw, bh := bmp.Size()
	if bw != 1920 || bh != 1080 {
		t.Fatalf("decoded asset size = %dx%d, want full 1920x1080", bw, bh)
	}

	// Re-importing identical file reuses same asset ID cleanly.
	a2, _, _, err := media.ImportImageFile(doc, imgPath, 480)
	if err != nil || a2.ID != a1.ID || len(doc.Assets) != 1 {
		t.Fatalf("expected SHA-256 deduplication, got assets=%d (%s vs %s)", len(doc.Assets), a1.ID, a2.ID)
	}
}

func TestProbeMP4MetaAndImportVideo(t *testing.T) {
	// Construct a minimal valid ISO BMFF moov -> mvhd + trak -> tkhd (2560x1440, 15s).
	mvhdPayload := make([]byte, 24)
	mvhdPayload[0] = 0 // version 0
	binary.BigEndian.PutUint32(mvhdPayload[12:16], 1000)
	binary.BigEndian.PutUint32(mvhdPayload[16:20], 15000) // 15.0s
	mvhd := makeBox("mvhd", mvhdPayload)

	tkhdPayload := make([]byte, 84)
	tkhdPayload[0] = 0 // version 0
	binary.BigEndian.PutUint32(tkhdPayload[76:80], 2560<<16)
	binary.BigEndian.PutUint32(tkhdPayload[80:84], 1440<<16)
	tkhd := makeBox("tkhd", tkhdPayload)
	trak := makeBox("trak", tkhd)
	moov := makeBox("moov", append(mvhd, trak...))

	vidPath := filepath.Join(t.TempDir(), "demo.mp4")
	if err := os.WriteFile(vidPath, moov, 0o644); err != nil {
		t.Fatal(err)
	}

	doc := graph.NewDocument("Video Test")
	asset, w, h, err := media.ImportVideoFile(doc, vidPath, 480)
	if err != nil {
		t.Fatal(err)
	}
	if asset.NaturalW != 2560 || asset.NaturalH != 1440 || asset.DurationSec != 15 {
		t.Fatalf("unexpected MP4 probe result: %+v", asset)
	}
	if w != 480 || h != 270 {
		t.Fatalf("expected fitted 480x270, got %gx%g", w, h)
	}
}

func makeBox(typ string, payload []byte) []byte {
	out := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint32(out[0:4], uint32(len(out)))
	copy(out[4:8], typ)
	copy(out[8:], payload)
	return out
}
