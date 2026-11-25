package drawio_test

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"net/url"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/drawio"
	"github.com/neko233-com/DrawGraph/internal/geo"
	"github.com/neko233-com/DrawGraph/internal/graph"
)

func TestRoundTripDrawioXML(t *testing.T) {
	doc := graph.NewDocument("Architecture")
	p := doc.ActivePage()
	_ = p.AddNode(&graph.Node{
		ID: "vpc", Kind: graph.KindContainer, X: 40, Y: 40, W: 500, H: 240, Label: "VPC",
	})
	_ = p.AddNode(&graph.Node{
		ID: "api", Kind: graph.KindRounded, ParentID: "vpc", X: 80, Y: 100, W: 160, H: 72,
		Label: "API Server", Icon: graph.ParseIconRef("builtin/server"),
		Style: graph.NodeStyle{Fill: "#EFF6FF", Stroke: "#3B82F6", Shadow: true},
	})
	_ = p.AddNode(&graph.Node{
		ID: "db", Kind: graph.KindCylinder, ParentID: "vpc", X: 340, Y: 100, W: 140, H: 80, Label: "Orders DB",
	})
	_ = p.AddEdge(&graph.Edge{
		ID: "e1", SourceID: "api", TargetID: "db", Label: "SQL",
		Routing: graph.RouteOrthogonal, EndArrow: graph.ArrowClassic,
		Waypoints: []geo.Vec{geo.V(280, 136)},
	})
	_, _ = doc.AddPage("page-2", "Flow", false)
	_ = doc.PageByID("page-2").AddNode(&graph.Node{ID: "d1", Kind: graph.KindDiamond, X: 100, Y: 100, W: 120, H: 80, Label: "OK?"})

	xmlBytes, err := drawio.ExportXML(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(xmlBytes), "<mxfile") || !strings.Contains(string(xmlBytes), "drawgraphIcon=builtin/server") {
		t.Fatalf("unexpected xml output:\n%s", string(xmlBytes))
	}

	imported, err := drawio.ParseXML(xmlBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(imported.Pages) != 2 {
		t.Fatalf("pages = %d, want 2", len(imported.Pages))
	}
	ip := imported.Pages[0]
	api := ip.NodeByID("api")
	if api == nil || api.X != 80 || api.Y != 100 || api.ParentID != "vpc" {
		t.Fatalf("imported api node mismatch: %+v", api)
	}
	if !api.Icon.Valid() || api.Icon.String() != "builtin/server" {
		t.Fatalf("lost custom icon ref on drawio roundtrip: %+v", api.Icon)
	}
	db := ip.NodeByID("db")
	if db == nil || db.Kind != graph.KindCylinder {
		t.Fatalf("expected cylinder node, got %+v", db)
	}
	e1 := ip.EdgeByID("e1")
	if e1 == nil || len(e1.Waypoints) != 1 || e1.Label != "SQL" {
		t.Fatalf("imported edge mismatch: %+v", e1)
	}
}

func TestParseCompressedDrawioDiagram(t *testing.T) {
	inner := `<mxGraphModel dx="800" dy="600" grid="1" gridSize="10"><root><mxCell id="0"/><mxCell id="1" parent="0"/><mxCell id="n1" value="Gateway" style="rounded=1;whiteSpace=wrap;html=1;" vertex="1" parent="1"><mxGeometry x="120" y="80" width="160" height="60" as="geometry"/></mxCell></root></mxGraphModel>`
	escaped := url.QueryEscape(inner)
	var compressed bytes.Buffer
	w, _ := flate.NewWriter(&compressed, flate.DefaultCompression)
	_, _ = w.Write([]byte(escaped))
	_ = w.Close()
	payload := base64.StdEncoding.EncodeToString(compressed.Bytes())

	mx := `<mxfile><diagram id="d1" name="Compressed">` + payload + `</diagram></mxfile>`
	doc, err := drawio.ParseXML([]byte(mx))
	if err != nil {
		t.Fatal(err)
	}
	n := doc.ActivePage().NodeByID("n1")
	if n == nil || n.Label != "Gateway" || n.X != 120 || n.Kind != graph.KindRounded {
		t.Fatalf("compressed diagram node mismatch: %+v", n)
	}
}
