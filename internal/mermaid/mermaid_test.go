package mermaid_test

import (
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/mermaid"
)

func TestParseMermaidFlowchartAndSubgraphs(t *testing.T) {
	src := `
flowchart LR
  subgraph cloud[Production Cloud]
    gw([API Gateway]) --> auth{Auth?}
    auth -->|Yes| svc[Order Service] -. async .-> db[(Postgres DB)]
  end
  user((User)) -->|HTTPS| gw
`
	hb, _ := habits.OpenStore(t.TempDir() + "/habits.json")
	doc, err := mermaid.Parse(src, hb)
	if err != nil {
		t.Fatal(err)
	}
	p := doc.ActivePage()
	if p.NodeByID("cloud") == nil || p.NodeByID("cloud").Kind != graph.KindContainer {
		t.Fatalf("expected cloud container node")
	}
	if p.NodeByID("auth") == nil || p.NodeByID("auth").Kind != graph.KindDiamond {
		t.Fatalf("expected diamond auth node, got %+v", p.NodeByID("auth"))
	}
	if p.NodeByID("db") == nil || p.NodeByID("db").Kind != graph.KindCylinder {
		t.Fatalf("expected cylinder db node, got %+v", p.NodeByID("db"))
	}
	if len(p.Edges) != 4 {
		t.Fatalf("expected 4 edges, got %d", len(p.Edges))
	}
	// Verify dashed edge parsing
	var foundAsync bool
	for _, e := range p.Edges {
		if e.Label == "async" && e.Style.Dash == graph.DashDashed {
			foundAsync = true
		}
	}
	if !foundAsync {
		t.Fatalf("expected dashed async edge, got %+v", p.Edges)
	}
}
