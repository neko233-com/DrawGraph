// Package media handles importing high-resolution images and videos into DrawGraph
// documents, computing aspect-preserving canvas footprints while retaining full original
// pixel fidelity, and probing MP4/MOV natural video dimensions in pure Go.
package media

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/neko233-com/DrawGraph/internal/graph"
)

const (
	DefaultMaxDisplaySide = 480.0
	DefaultMinDisplaySide = 96.0
)

// Kind constants for imported media files.
const (
	KindImage = "image"
	KindVideo = "video"
)

// ClassifyPath returns "image", "video", or "" based on file extension.
func ClassifyPath(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return KindImage
	case ".mp4", ".mov", ".m4v", ".webm", ".mkv", ".avi":
		return KindVideo
	default:
		return ""
	}
}

// FitDisplaySize computes a reasonable initial canvas node footprint (w, h in DIPs)
// preserving exact natural aspect ratio:
//   - Images/videos larger than maxSide (e.g. 3840x2160) scale proportionally into maxSide (e.g. 480x270)
//     while the backing asset keeps 100% of its native resolution for sharp zoom and HD export.
//   - Medium images (e.g. 320x240) remain 1:1 with their natural dimensions.
//   - Tiny images (< minSide on both axes) scale gently up to minSide so they stay selectable.
func FitDisplaySize(natW, natH int, maxSide, minSide float64) (w, h float64) {
	if maxSide <= 0 {
		maxSide = DefaultMaxDisplaySide
	}
	if minSide <= 0 {
		minSide = DefaultMinDisplaySide
	}
	if natW <= 0 || natH <= 0 {
		return 320, 180
	}
	fw, fh := float64(natW), float64(natH)
	longest := max(fw, fh)
	if longest > maxSide {
		scale := maxSide / longest
		return math.Round(fw * scale), math.Round(fh * scale)
	}
	if longest < minSide {
		scale := minSide / longest
		return math.Round(fw * scale), math.Round(fh * scale)
	}
	return fw, fh
}

// ImportImageFile reads an image file from disk at full native resolution, stores/deduplicates it
// in doc.Assets, and returns the asset plus its reasonable initial canvas display size.
func ImportImageFile(doc *graph.Document, path string, maxSide float64) (*graph.MediaAsset, float64, float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, 0, err
	}
	abs, _ := filepath.Abs(path)
	asset, w, h, err := ImportImageBytes(doc, filepath.Base(path), mimeForExt(filepath.Ext(path)), data, maxSide)
	if err != nil {
		return nil, 0, 0, err
	}
	if abs != "" && asset.FilePath == "" {
		asset.FilePath = abs
	}
	return asset, w, h, nil
}

// ImportImageBytes decodes image bytes (PNG/JPEG/GIF/WebP/BMP/SVG), extracts natural dimensions,
// embeds the full-resolution DataURL in doc.Assets (deduplicated by SHA-256), and computes display size.
func ImportImageBytes(doc *graph.Document, name, mimeType string, data []byte, maxSide float64) (*graph.MediaAsset, float64, float64, error) {
	if doc == nil {
		return nil, 0, 0, fmt.Errorf("nil document")
	}
	doc.Normalize()
	if len(data) == 0 {
		return nil, 0, 0, fmt.Errorf("empty image data")
	}
	src, err := ui.DecodeImage(data)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("decode image %s: %w", name, err)
	}
	natW, natH := imageSourceSize(src)
	if mimeType == "" {
		mimeType = detectMime(name, data)
	}
	sum := sha256.Sum256(data)
	id := fmt.Sprintf("img-%x", sum[:8])
	if existing, ok := doc.Assets[id]; ok {
		w, h := FitDisplaySize(existing.NaturalW, existing.NaturalH, maxSide, DefaultMinDisplaySide)
		return existing, w, h, nil
	}
	dataURL := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
	asset := &graph.MediaAsset{
		ID:       id,
		Kind:     KindImage,
		Name:     name,
		MimeType: mimeType,
		DataURL:  dataURL,
		NaturalW: natW,
		NaturalH: natH,
	}
	doc.Assets[id] = asset
	w, h := FitDisplaySize(natW, natH, maxSide, DefaultMinDisplaySide)
	return asset, w, h, nil
}

// ImportVideoFile registers a video file in doc.Assets, probing MP4/MOV track dimensions
// in pure Go when available and computing an aspect-matched initial node footprint.
func ImportVideoFile(doc *graph.Document, path string, maxSide float64) (*graph.MediaAsset, float64, float64, error) {
	if doc == nil {
		return nil, 0, 0, fmt.Errorf("nil document")
	}
	doc.Normalize()
	st, err := os.Stat(path)
	if err != nil {
		return nil, 0, 0, err
	}
	if st.IsDir() {
		return nil, 0, 0, fmt.Errorf("%s is a directory", path)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	sum := sha256.Sum256([]byte(abs))
	id := fmt.Sprintf("vid-%x", sum[:8])
	if existing, ok := doc.Assets[id]; ok {
		w, h := FitDisplaySize(existing.NaturalW, existing.NaturalH, maxSide, DefaultMinDisplaySide)
		return existing, w, h, nil
	}
	natW, natH, dur := ProbeMP4Meta(abs)
	if natW <= 0 || natH <= 0 {
		natW, natH = 1920, 1080
	}
	asset := &graph.MediaAsset{
		ID:          id,
		Kind:        KindVideo,
		Name:        filepath.Base(abs),
		MimeType:    mimeForExt(filepath.Ext(abs)),
		FilePath:    abs,
		NaturalW:    natW,
		NaturalH:    natH,
		DurationSec: dur,
	}
	doc.Assets[id] = asset
	w, h := FitDisplaySize(natW, natH, maxSide, DefaultMinDisplaySide)
	return asset, w, h, nil
}

// DecodeAssetImage decodes a stored MediaAsset's DataURL (or fallback FilePath) into a ui.ImageSource.
func DecodeAssetImage(asset *graph.MediaAsset) (ui.ImageSource, error) {
	if asset == nil {
		return nil, fmt.Errorf("nil asset")
	}
	if strings.HasPrefix(asset.DataURL, "data:") {
		if idx := strings.IndexByte(asset.DataURL, ','); idx > 0 {
			raw, err := base64.StdEncoding.DecodeString(asset.DataURL[idx+1:])
			if err == nil {
				return ui.DecodeImage(raw)
			}
		}
	}
	if asset.FilePath != "" {
		raw, err := os.ReadFile(asset.FilePath)
		if err != nil {
			return nil, err
		}
		return ui.DecodeImage(raw)
	}
	return nil, fmt.Errorf("asset %s has no image data or path", asset.ID)
}

func imageSourceSize(src ui.ImageSource) (int, int) {
	switch v := src.(type) {
	case *ui.Bitmap:
		return v.Size()
	default:
		w, h := ui.ImageSize(src)
		return int(math.Round(float64(w))), int(math.Round(float64(h)))
	}
}

func mimeForExt(ext string) string {
	switch strings.ToLower(ext) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".bmp":
		return "image/bmp"
	case ".svg":
		return "image/svg+xml"
	case ".mp4", ".m4v":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "application/octet-stream"
	}
}

func detectMime(name string, data []byte) string {
	if ext := filepath.Ext(name); ext != "" {
		return mimeForExt(ext)
	}
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "image/png"
	}
	if len(data) >= 3 && data[0] == 0xff && data[1] == 0xd8 && data[2] == 0xff {
		return "image/jpeg"
	}
	if bytes.Contains(bytes.TrimSpace(data[:min(len(data), 128)]), []byte("<svg")) {
		return "image/svg+xml"
	}
	return "image/png"
}

// ProbeMP4Meta reads ISO BMFF (MP4/MOV/M4V) mvhd duration and tkhd track dimensions
// in pure Go without spawning external decoders. Returns (0,0,0) for unsupported containers.
func ProbeMP4Meta(path string) (w, h int, durationSec float64) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, 0
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		return 0, 0, 0
	}
	scanISOBoxes(f, 0, st.Size(), &w, &h, &durationSec)
	return w, h, durationSec
}

func scanISOBoxes(r io.ReaderAt, start, end int64, w, h *int, dur *float64) {
	var hdr [16]byte
	pos := start
	for pos+8 <= end {
		if _, err := r.ReadAt(hdr[:8], pos); err != nil {
			return
		}
		sz := int64(binary.BigEndian.Uint32(hdr[0:4]))
		typ := string(hdr[4:8])
		hdrLen := int64(8)
		if sz == 1 {
			if pos+16 > end {
				return
			}
			if _, err := r.ReadAt(hdr[8:16], pos+8); err != nil {
				return
			}
			sz = int64(binary.BigEndian.Uint64(hdr[8:16]))
			hdrLen = 16
		} else if sz == 0 {
			sz = end - pos
		}
		if sz < hdrLen || pos+sz > end {
			return
		}
		payloadStart := pos + hdrLen
		payloadEnd := pos + sz
		switch typ {
		case "moov", "trak", "mdia", "minf", "stbl":
			scanISOBoxes(r, payloadStart, payloadEnd, w, h, dur)
		case "mvhd":
			parseMVHD(r, payloadStart, payloadEnd, dur)
		case "tkhd":
			parseTKHD(r, payloadStart, payloadEnd, w, h)
		}
		pos += sz
	}
}

func parseMVHD(r io.ReaderAt, start, end int64, dur *float64) {
	if end-start < 24 {
		return
	}
	var buf [32]byte
	n := min(int64(len(buf)), end-start)
	if _, err := r.ReadAt(buf[:n], start); err != nil {
		return
	}
	version := buf[0]
	var timescale uint32
	var rawDur uint64
	if version == 0 && n >= 20 {
		timescale = binary.BigEndian.Uint32(buf[12:16])
		rawDur = uint64(binary.BigEndian.Uint32(buf[16:20]))
	} else if version == 1 && n >= 32 {
		timescale = binary.BigEndian.Uint32(buf[20:24])
		rawDur = binary.BigEndian.Uint64(buf[24:32])
	}
	if timescale > 0 && rawDur > 0 {
		*dur = float64(rawDur) / float64(timescale)
	}
}

func parseTKHD(r io.ReaderAt, start, end int64, w, h *int) {
	if end-start < 84 {
		return
	}
	buf := make([]byte, min(int64(96), end-start))
	if _, err := r.ReadAt(buf, start); err != nil {
		return
	}
	version := buf[0]
	off := 76
	if version == 1 {
		off = 88
	}
	if off+8 > len(buf) {
		return
	}
	// Fixed-point 16.16 width & height.
	tw := int(binary.BigEndian.Uint32(buf[off:off+4]) >> 16)
	th := int(binary.BigEndian.Uint32(buf[off+4:off+8]) >> 16)
	if tw > 0 && th > 0 && (*w == 0 || tw*th > (*w)*(*h)) {
		*w = tw
		*h = th
	}
}
