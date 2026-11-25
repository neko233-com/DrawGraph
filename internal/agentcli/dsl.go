package agentcli

import (
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/ops"
)

// ApplyDSL parses concise line-oriented DrawGraph DSL statements and mutates page.
func ApplyDSL(doc *graph.Document, page *graph.Page, src string, hb *habits.Store) (addedNodes int, addedEdges int, err error) {
	return ops.ApplyDSL(doc, page, src, hb)
}
