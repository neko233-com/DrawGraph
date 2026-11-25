package agentcli

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/media"
)

// ApplyDSL parses concise line-oriented DrawGraph DSL statements and mutates page.
//
// Supported syntax:
//
//	# comment
//	page "<name>"
//	container <id> "<label>" [fill:#hex] [stroke:#hex]
//	swimlane <id> "<label>" [fill:#hex]
//	node <id> "<label>" [kind:rounded|cylinder|diamond|...] [icon:pack/name] [parent:id] [sub:"..."] [fill:#hex] [stroke:#hex] [x:N] [y:N] [w:N] [h:N]
//	<srcId> -> <dstId> [: "<label>"] [routing:orthogonal|curved|straight] [arrow:arrow|open|diamond|circle|none]
func ApplyDSL(doc *graph.Document, page *graph.Page, src string, hb *habits.Store) (addedNodes int, addedEdges int, err error) {
	if doc == nil {
		return 0, 0, fmt.Errorf("nil document")
	}
	if page == nil {
		page = doc.ActivePage()
	}
	if hb == nil {
		hb, _ = habits.OpenStore("")
	}
	lines := strings.Split(src, "\n")
	for lineNo, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			continue
		}
		tokens, perr := splitShellTokens(line)
		if perr != nil {
			return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, perr)
		}
		if len(tokens) == 0 {
			continue
		}

		// Edge form: <src> -> <dst> [: "label"] [attrs...]
		if len(tokens) >= 3 && (tokens[1] == "->" || tokens[1] == "-->" || tokens[1] == "<->") {
			srcID := tokens[0]
			dstID := tokens[2]
			edgeLabel := ""
			rest := tokens[3:]
			if len(rest) > 0 && rest[0] == ":" {
				if len(rest) < 2 {
					return addedNodes, addedEdges, fmt.Errorf("line %d: expected edge label after ':'", lineNo+1)
				}
				edgeLabel = rest[1]
				rest = rest[2:]
			} else if len(rest) > 0 && strings.HasPrefix(rest[0], ":") {
				edgeLabel = strings.TrimSpace(strings.TrimPrefix(rest[0], ":"))
				rest = rest[1:]
			}
			// Auto-create placeholder nodes if referenced before declaration.
			if page.NodeByID(srcID) == nil {
				n := hb.NewNodeFromHabits("", srcID, 80, 80)
				n.ID = srcID
				_ = page.AddNode(n)
				addedNodes++
			}
			if page.NodeByID(dstID) == nil {
				n := hb.NewNodeFromHabits("", dstID, 280, 80)
				n.ID = dstID
				_ = page.AddNode(n)
				addedNodes++
			}
			e := hb.NewEdgeFromHabits(srcID, dstID, edgeLabel)
			if tokens[1] == "<->" {
				e.StartArrow = e.EndArrow
			}
			if tokens[1] == "-->" {
				e.Style.Dash = graph.DashDashed
			}
			for _, attr := range rest {
				k, v, ok := strings.Cut(attr, ":")
				if !ok {
					k, v, ok = strings.Cut(attr, "=")
				}
				if !ok {
					continue
				}
				switch strings.ToLower(k) {
				case "id":
					e.ID = v
				case "label":
					e.Label = v
				case "routing", "route":
					e.Routing = graph.RoutingStyle(v)
				case "arrow", "end":
					e.EndArrow = graph.ArrowStyle(v)
				case "start":
					e.StartArrow = graph.ArrowStyle(v)
				case "stroke", "color":
					e.Style.Stroke = v
				case "dash":
					e.Style.Dash = graph.DashStyle(v)
				}
			}
			if err := page.AddEdge(e); err != nil {
				return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, err)
			}
			addedEdges++
			continue
		}

		kw := strings.ToLower(tokens[0])
		switch kw {
		case "page":
			if len(tokens) < 2 {
				return addedNodes, addedEdges, fmt.Errorf("line %d: page requires a name", lineNo+1)
			}
			if found := doc.PageByID(tokens[1]); found != nil {
				page = found
				doc.ActivePageID = found.ID
			} else {
				p, aerr := doc.AddPage("", tokens[1], true)
				if aerr != nil {
					return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, aerr)
				}
				page = p
			}
		case "node", "container", "swimlane", "icon":
			if len(tokens) < 2 {
				return addedNodes, addedEdges, fmt.Errorf("line %d: %s requires an id", lineNo+1, kw)
			}
			id := tokens[1]
			label := id
			restIdx := 2
			if len(tokens) > 2 && !strings.ContainsAny(tokens[2], ":=") {
				label = tokens[2]
				restIdx = 3
			}
			var kind graph.NodeKind
			switch kw {
			case "container":
				kind = graph.KindContainer
			case "swimlane":
				kind = graph.KindSwimlane
			case "icon":
				kind = graph.KindIcon
			}
			n := page.NodeByID(id)
			isNew := false
			if n == nil {
				n = hb.NewNodeFromHabits(kind, label, float64(80+len(page.Nodes)*40), 80)
				n.ID = id
				isNew = true
			} else {
				n.Label = label
				if kind != "" {
					n.Kind = kind
				}
			}
			for _, attr := range tokens[restIdx:] {
				k, v, ok := strings.Cut(attr, ":")
				if !ok {
					k, v, ok = strings.Cut(attr, "=")
				}
				if !ok {
					continue
				}
				switch strings.ToLower(k) {
				case "kind", "shape":
					n.Kind = graph.NodeKind(v)
				case "icon":
					n.Icon = graph.ParseIconRef(v)
				case "parent":
					n.ParentID = v
				case "sub", "sublabel":
					n.Sublabel = v
				case "fill":
					n.Style.Fill = v
				case "stroke":
					n.Style.Stroke = v
				case "text", "textcolor":
					n.Style.TextColor = v
				case "dash":
					n.Style.Dash = graph.DashStyle(v)
				case "x":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						n.X = f
					}
				case "y":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						n.Y = f
					}
				case "w":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						n.W = f
					}
				case "h":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						n.H = f
					}
				}
			}
			if isNew {
				if err := page.AddNode(n); err != nil {
					return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, err)
				}
				addedNodes++
			}
		case "image", "video":
			if len(tokens) < 3 {
				return addedNodes, addedEdges, fmt.Errorf("line %d: %s requires <id> <srcPath>", lineNo+1, kw)
			}
			id := tokens[1]
			srcPath := tokens[2]
			prof := hb.Snapshot()
			maxSide := prof.DefaultMediaMax
			nativeSz := false
			fit := "contain"
			label := filepath.Base(srcPath)
			parent := ""
			loop := true
			muted := true
			var xOpt, yOpt *float64

			for _, attr := range tokens[3:] {
				k, v, ok := strings.Cut(attr, ":")
				if !ok {
					k, v, ok = strings.Cut(attr, "=")
				}
				if !ok {
					continue
				}
				switch strings.ToLower(k) {
				case "label":
					label = v
				case "parent":
					parent = v
				case "fit":
					fit = v
				case "native":
					nativeSz = v == "true" || v == "1"
				case "loop":
					loop = v == "true" || v == "1"
				case "muted":
					muted = v == "true" || v == "1"
				case "max", "max-side":
					if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
						maxSide = f
					}
				case "x":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						xOpt = &f
					}
				case "y":
					if f, err := strconv.ParseFloat(v, 64); err == nil {
						yOpt = &f
					}
				}
			}

			var asset *graph.MediaAsset
			var dw, dh float64
			var merr error
			if kw == "image" {
				asset, dw, dh, merr = media.ImportImageFile(doc, srcPath, maxSide)
			} else {
				asset, dw, dh, merr = media.ImportVideoFile(doc, srcPath, maxSide)
			}
			if merr != nil {
				return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, merr)
			}
			if nativeSz && asset.NaturalW > 0 && asset.NaturalH > 0 {
				dw, dh = float64(asset.NaturalW), float64(asset.NaturalH)
			}
			x, y := float64(80+len(page.Nodes)*40), 80.0
			if xOpt != nil {
				x = *xOpt
			}
			if yOpt != nil {
				y = *yOpt
			}
			kind := graph.KindImage
			if kw == "video" {
				kind = graph.KindVideo
			}
			n := &graph.Node{
				ID:       id,
				Kind:     kind,
				ParentID: parent,
				X:        x,
				Y:        y,
				W:        dw,
				H:        dh,
				Label:    label,
				Media: &graph.NodeMedia{
					AssetID:  asset.ID,
					Fit:      fit,
					Autoplay: kw == "video",
					Loop:     loop,
					Muted:    muted,
				},
			}
			if err := page.AddNode(n); err != nil {
				return addedNodes, addedEdges, fmt.Errorf("line %d: %w", lineNo+1, err)
			}
			addedNodes++
		default:
			return addedNodes, addedEdges, fmt.Errorf("line %d: unknown statement %q", lineNo+1, tokens[0])
		}
	}
	return addedNodes, addedEdges, nil
}

func splitShellTokens(s string) ([]string, error) {
	var out []string
	var cur strings.Builder
	inQuote := rune(0)
	escaped := false
	flushed := false

	flush := func() {
		if cur.Len() > 0 || flushed {
			out = append(out, cur.String())
			cur.Reset()
			flushed = false
		}
	}

	for _, r := range s {
		if escaped {
			cur.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && inQuote != '\'' {
			escaped = true
			continue
		}
		if inQuote != 0 {
			if r == inQuote {
				inQuote = 0
				flushed = true
			} else {
				cur.WriteRune(r)
			}
			continue
		}
		switch r {
		case '"', '\'':
			inQuote = r
		case ' ', '\t', '\r':
			flush()
		default:
			cur.WriteRune(r)
		}
	}
	if inQuote != 0 {
		return nil, fmt.Errorf("unclosed quote")
	}
	flush()
	return out, nil
}
