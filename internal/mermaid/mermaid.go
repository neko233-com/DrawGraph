// Package mermaid imports Mermaid flowchart / graph definitions into DrawGraph Documents.
package mermaid

import (
	"fmt"
	"strings"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/layout"
)

// Parse converts Mermaid flowchart / graph source into a formatted DrawGraph Document.
func Parse(src string, hb *habits.Store) (*graph.Document, error) {
	if hb == nil {
		hb, _ = habits.OpenStore("")
	}
	doc := graph.NewDocument("Mermaid Diagram")
	hb.ApplyToDocument(doc)
	page := doc.ActivePage()

	dir, addedN, addedE, err := Apply(doc, page, src, hb)
	if err != nil {
		return nil, err
	}
	if addedN == 0 && addedE == 0 {
		return nil, fmt.Errorf("no Mermaid nodes or edges found")
	}
	opts := layout.OptionsFromHabits(hb.Snapshot(), false)
	if dir != "" {
		opts.Direction = dir
	}
	layout.FormatPage(doc, page, opts)
	return doc, nil
}

// Apply parses Mermaid flowchart statements onto an existing doc/page, returning detected direction and counts.
func Apply(doc *graph.Document, page *graph.Page, src string, hb *habits.Store) (direction string, addedNodes int, addedEdges int, err error) {
	if doc == nil {
		return "", 0, 0, fmt.Errorf("nil document")
	}
	if page == nil {
		page = doc.ActivePage()
	}
	if hb == nil {
		hb, _ = habits.OpenStore("")
	}

	var subgraphStack []string
	rawLines := strings.Split(src, "\n")

	for _, raw := range rawLines {
		line := strings.TrimSpace(raw)
		if i := strings.Index(line, "%%"); i >= 0 {
			line = strings.TrimSpace(line[:i])
		}
		if line == "" || strings.HasPrefix(line, "classDef ") || strings.HasPrefix(line, "class ") ||
			strings.HasPrefix(line, "style ") || strings.HasPrefix(line, "linkStyle ") || strings.HasPrefix(line, "click ") {
			continue
		}

		// Multi-statement semicolon split (outside quotes/brackets not usually needed cleanly here)
		for _, stmt := range strings.Split(line, ";") {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			low := strings.ToLower(stmt)
			if strings.HasPrefix(low, "flowchart ") || strings.HasPrefix(low, "graph ") {
				fields := strings.Fields(stmt)
				if len(fields) >= 2 {
					switch strings.ToUpper(fields[1]) {
					case "TD", "TB":
						direction = "TB"
					case "BT":
						direction = "BT"
					case "LR":
						direction = "LR"
					case "RL":
						direction = "RL"
					}
				}
				continue
			}
			if strings.HasPrefix(low, "subgraph ") {
				rest := strings.TrimSpace(stmt[len("subgraph "):])
				subID, subLabel := parseSubgraphHeader(rest, page)
				parentID := ""
				if len(subgraphStack) > 0 {
					parentID = subgraphStack[len(subgraphStack)-1]
				}
				if page.NodeByID(subID) == nil {
					c := hb.NewNodeFromHabits(graph.KindContainer, subLabel, 80, 80)
					c.ID = subID
					c.ParentID = parentID
					_ = page.AddNode(c)
					addedNodes++
				}
				subgraphStack = append(subgraphStack, subID)
				continue
			}
			if low == "end" {
				if len(subgraphStack) > 0 {
					subgraphStack = subgraphStack[:len(subgraphStack)-1]
				}
				continue
			}
			if strings.HasPrefix(low, "direction ") {
				fields := strings.Fields(stmt)
				if len(fields) >= 2 {
					switch strings.ToUpper(fields[1]) {
					case "TD", "TB":
						direction = "TB"
					case "LR":
						direction = "LR"
					case "RL":
						direction = "RL"
					case "BT":
						direction = "BT"
					}
				}
				continue
			}

			curParent := ""
			if len(subgraphStack) > 0 {
				curParent = subgraphStack[len(subgraphStack)-1]
			}
			nCount, eCount := parseEdgeOrNodeStatement(stmt, curParent, page, hb)
			addedNodes += nCount
			addedEdges += eCount
		}
	}
	page.MarkDirty()
	return direction, addedNodes, addedEdges, nil
}

func parseSubgraphHeader(rest string, p *graph.Page) (id, label string) {
	// Forms: `id[Title]`, `id ["Title"]`, `Title`
	if idx := strings.IndexByte(rest, '['); idx > 0 && strings.HasSuffix(rest, "]") {
		id = sanitizeID(strings.TrimSpace(rest[:idx]))
		label = strings.Trim(strings.TrimSpace(rest[idx+1:len(rest)-1]), `"'`)
		return id, label
	}
	fields := strings.Fields(rest)
	if len(fields) == 1 {
		clean := strings.Trim(fields[0], `"'`)
		return sanitizeID(clean), clean
	}
	clean := strings.Trim(rest, `"'`)
	return p.NextNodeID("group"), clean
}

type edgeTok struct {
	op    string
	label string
	dash  graph.DashStyle
	biDir bool
	arrow graph.ArrowStyle
}

func parseEdgeOrNodeStatement(stmt, parentID string, p *graph.Page, hb *habits.Store) (addedN, addedE int) {
	parts, edges := splitChain(stmt)
	if len(edges) == 0 {
		for _, tok := range strings.Split(stmt, "&") {
			if _, created := upsertNodeExpr(strings.TrimSpace(tok), parentID, p, hb); created {
				addedN++
			}
		}
		return addedN, addedE
	}

	var prevGroup []*graph.Node
	for _, tok := range strings.Split(parts[0], "&") {
		n, created := upsertNodeExpr(strings.TrimSpace(tok), parentID, p, hb)
		if created {
			addedN++
		}
		if n != nil {
			prevGroup = append(prevGroup, n)
		}
	}

	for ei, et := range edges {
		var nextGroup []*graph.Node
		for _, tok := range strings.Split(parts[ei+1], "&") {
			n, created := upsertNodeExpr(strings.TrimSpace(tok), parentID, p, hb)
			if created {
				addedN++
			}
			if n != nil {
				nextGroup = append(nextGroup, n)
			}
		}
		for _, src := range prevGroup {
			for _, dst := range nextGroup {
				e := hb.NewEdgeFromHabits(src.ID, dst.ID, et.label)
				if et.dash != "" {
					e.Style.Dash = et.dash
				}
				if et.arrow != "" {
					e.EndArrow = et.arrow
				}
				if et.biDir {
					e.StartArrow = e.EndArrow
				}
				if err := p.AddEdge(e); err == nil {
					addedE++
				}
			}
		}
		prevGroup = nextGroup
	}
	return addedN, addedE
}

type opSpec struct {
	prefix string
	dash   graph.DashStyle
	biDir  bool
	arrow  graph.ArrowStyle
}

var mermaidOps = []opSpec{
	{prefix: "<-->", arrow: graph.ArrowClassic, biDir: true},
	{prefix: "-.->", dash: graph.DashDashed, arrow: graph.ArrowClassic},
	{prefix: "==>", arrow: graph.ArrowClassic},
	{prefix: "-->", arrow: graph.ArrowClassic},
	{prefix: "---", arrow: graph.ArrowNone},
	{prefix: "->-", arrow: graph.ArrowClassic},
	{prefix: "->", arrow: graph.ArrowClassic},
}

func splitChain(s string) ([]string, []edgeTok) {
	var nodes []string
	var edges []edgeTok
	start := 0
	inParen, inBracket, inBrace := 0, 0, 0
	inQuote := rune(0)

	i := 0
	for i < len(s) {
		ch := rune(s[i])
		if inQuote != 0 {
			if ch == inQuote {
				inQuote = 0
			}
			i++
			continue
		}
		switch ch {
		case '"', '\'':
			inQuote = ch
			i++
			continue
		case '(':
			inParen++
		case ')':
			if inParen > 0 {
				inParen--
			}
		case '[':
			inBracket++
		case ']':
			if inBracket > 0 {
				inBracket--
			}
		case '{':
			inBrace++
		case '}':
			if inBrace > 0 {
				inBrace--
			}
		}
		if inParen > 0 || inBracket > 0 || inBrace > 0 {
			i++
			continue
		}

		matched := false
		// Check inline label form: `-- text -->` or `-. text .->` or `== text ==>`
		if strings.HasPrefix(s[i:], "-- ") || strings.HasPrefix(s[i:], "-. ") {
			isDash := strings.HasPrefix(s[i:], "-. ")
			endTok := "-->"
			if isDash {
				endTok = ".->"
			}
			if end := strings.Index(s[i+3:], endTok); end >= 0 {
				lbl := strings.TrimSpace(s[i+3 : i+3+end])
				nodes = append(nodes, strings.TrimSpace(s[start:i]))
				d := graph.DashSolid
				if isDash {
					d = graph.DashDashed
				}
				edges = append(edges, edgeTok{op: endTok, label: lbl, dash: d, arrow: graph.ArrowClassic})
				i += 3 + end + len(endTok)
				start = i
				matched = true
			}
		}
		if matched {
			continue
		}

		for _, sp := range mermaidOps {
			if strings.HasPrefix(s[i:], sp.prefix) {
				nodes = append(nodes, strings.TrimSpace(s[start:i]))
				j := i + len(sp.prefix)
				lbl := ""
				// Optional trailing `|label|`
				rest := strings.TrimLeft(s[j:], " ")
				if strings.HasPrefix(rest, "|") {
					if endPipe := strings.IndexByte(rest[1:], '|'); endPipe >= 0 {
						lbl = strings.TrimSpace(rest[1 : 1+endPipe])
						advance := (len(s[j:]) - len(rest)) + 1 + endPipe + 1
						j += advance
					}
				}
				edges = append(edges, edgeTok{op: sp.prefix, label: lbl, dash: sp.dash, biDir: sp.biDir, arrow: sp.arrow})
				i = j
				start = i
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	nodes = append(nodes, strings.TrimSpace(s[start:]))
	return nodes, edges
}

func upsertNodeExpr(expr, parentID string, p *graph.Page, hb *habits.Store) (*graph.Node, bool) {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil, false
	}
	// Strip Mermaid class suffix `:::cls`
	if idx := strings.Index(expr, ":::"); idx > 0 {
		expr = strings.TrimSpace(expr[:idx])
	}
	id, label, kind, hasShape := parseNodeToken(expr)
	if id == "" {
		return nil, false
	}
	if existing := p.NodeByID(id); existing != nil {
		if hasShape {
			existing.Kind = kind
			if label != "" {
				existing.Label = label
			}
		}
		if existing.ParentID == "" && parentID != "" {
			existing.ParentID = parentID
		}
		return existing, false
	}
	n := hb.NewNodeFromHabits(kind, label, float64(80+len(p.Nodes)*40), 80)
	n.ID = id
	n.ParentID = parentID
	if err := p.AddNode(n); err != nil {
		return nil, false
	}
	return n, true
}

func parseNodeToken(expr string) (id, label string, kind graph.NodeKind, hasShape bool) {
	kind = graph.KindRounded
	delims := []struct {
		open  string
		close string
		kind  graph.NodeKind
	}{
		{"[(", ")]", graph.KindCylinder},
		{"((", "))", graph.KindEllipse},
		{"([", "])", graph.KindRounded},
		{"{{", "}}", graph.KindHexagon},
		{"[/", "/]", graph.KindParallelogram},
		{"[", "]", graph.KindRect},
		{"(", ")", graph.KindRounded},
		{"{", "}", graph.KindDiamond},
	}

	bestIdx := len(expr)
	var chosenClose string
	var chosenOpen string
	for _, d := range delims {
		idx := strings.Index(expr, d.open)
		if idx >= 0 && idx < bestIdx && strings.HasSuffix(expr, d.close) {
			bestIdx = idx
			chosenOpen = d.open
			chosenClose = d.close
			kind = d.kind
			hasShape = true
		}
	}
	if !hasShape {
		clean := sanitizeID(expr)
		return clean, clean, graph.KindRounded, false
	}
	id = sanitizeID(expr[:bestIdx])
	rawLbl := expr[bestIdx+len(chosenOpen) : len(expr)-len(chosenClose)]
	rawLbl = strings.ReplaceAll(rawLbl, "<br/>", "\n")
	rawLbl = strings.ReplaceAll(rawLbl, "<br>", "\n")
	label = strings.Trim(strings.TrimSpace(rawLbl), `"'`)
	if label == "" {
		label = id
	}
	return id, label, kind, true
}

func sanitizeID(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.Trim(s, `"'`)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}
