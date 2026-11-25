package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
)

func TestFutureFormatVersionPreservedAndUpgradable(t *testing.T) {
	// A document saved with a future FormatVersion > current binary's FormatVersion
	// must load without losing unknown Meta fields.
	tmp := filepath.Join(t.TempDir(), "future.drawgraph")
	raw := `{
  "formatVersion": 99,
  "title": "Future Diagram",
  "activePageId": "page-1",
  "meta": {"futureFeature": "enabled"},
  "pages": [{
    "id": "page-1",
    "name": "Page-1",
    "nodes": [{"id": "n1", "kind": "future-widget", "x": 10, "y": 20, "w": 120, "h": 60, "label": "NextGen", "meta": {"ext": "v99"}}],
    "edges": []
  }]
}`
	if err := os.WriteFile(tmp, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err := graph.LoadFile(tmp)
	if err != nil {
		t.Fatal(err)
	}
	if doc.FormatVersion != 99 || doc.Meta["futureFeature"] != "enabled" {
		t.Fatalf("future metadata lost: %+v", doc)
	}
	n := doc.ActivePage().NodeByID("n1")
	if n == nil || n.Kind != "future-widget" || n.Meta["ext"] != "v99" {
		t.Fatalf("future node lost: %+v", n)
	}
}
