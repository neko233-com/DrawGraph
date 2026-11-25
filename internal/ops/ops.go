// Package ops provides shared high-level diagram operations, DSL parsing, transactional batching,
// templates, capability schemas, and persistence helpers shared cleanly by both the Agent CLI and MCP server.
package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/neko233-com/DrawGraph/internal/drawio"
	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/i18n"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/ipc"
	"github.com/neko233-com/DrawGraph/internal/layout"
	"github.com/neko233-com/DrawGraph/internal/media"
	"github.com/neko233-com/DrawGraph/internal/render"
	"github.com/neko233-com/DrawGraph/internal/version"
)

// BatchOp describes one transactional diagram mutation for CLI `batch` and MCP `diagram_batch`.
type BatchOp struct {
	Op       string          `json:"op"`
	ID       string          `json:"id,omitempty"`
	IDs      []string        `json:"ids,omitempty"`
	Name     string          `json:"name,omitempty"`
	Kind     graph.NodeKind  `json:"kind,omitempty"`
	Label    string          `json:"label,omitempty"`
	Sublabel string          `json:"sublabel,omitempty"`
	Icon     string          `json:"icon,omitempty"`
	ParentID string          `json:"parentId,omitempty"`
	From     string          `json:"from,omitempty"`
	To       string          `json:"to,omitempty"`
	Routing  string          `json:"routing,omitempty"`
	Arrow    string          `json:"arrow,omitempty"`
	Fill     string          `json:"fill,omitempty"`
	Stroke   string          `json:"stroke,omitempty"`
	Mode     string          `json:"mode,omitempty"`
	Strategy string          `json:"strategy,omitempty"`
	Dir      string          `json:"direction,omitempty"`
	X        *float64        `json:"x,omitempty"`
	Y        *float64        `json:"y,omitempty"`
	W        *float64        `json:"w,omitempty"`
	H        *float64        `json:"h,omitempty"`
	Dash     graph.DashStyle `json:"dash,omitempty"`
}

// LoadAnyDoc loads a .drawgraph or .drawio/.xml document from disk.
func LoadAnyDoc(path string) (*graph.Document, error) {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".drawio" || ext == ".xml" {
		return drawio.ImportFile(path)
	}
	return graph.LoadFile(path)
}

// LoadOrCreateDoc opens an existing diagram or initializes a fresh document seeded by user habits.
func LoadOrCreateDoc(path string, hb *habits.Store) (*graph.Document, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		doc := graph.NewDocument(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
		if hb != nil {
			hb.ApplyToDocument(doc)
		}
		return doc, nil
	}
	return LoadAnyDoc(path)
}

// SaveDoc saves doc atomically to path (.drawgraph or .drawio) and optionally pushes to the running GUI.
func SaveDoc(path string, doc *graph.Document, live bool, reason string) error {
	ext := strings.ToLower(filepath.Ext(path))
	var err error
	if ext == ".drawio" || ext == ".xml" {
		err = drawio.ExportFile(path, doc)
	} else {
		err = graph.SaveAtomic(path, doc)
	}
	if err != nil {
		return err
	}
	if live {
		if ep, derr := ipc.Discover(""); derr == nil {
			_ = ipc.PushDocument(ep, doc, reason)
		}
	}
	return nil
}

// SelectPage resolves pageID on doc or returns ActivePage when pageID is empty.
func SelectPage(doc *graph.Document, pageID string) (*graph.Page, error) {
	if doc == nil {
		return nil, fmt.Errorf("nil document")
	}
	if pageID == "" {
		return doc.ActivePage(), nil
	}
	p := doc.PageByID(pageID)
	if p == nil {
		return nil, fmt.Errorf("page %q not found", pageID)
	}
	return p, nil
}

// ExecuteBatchOps applies atomic batch operations to doc/page.
func ExecuteBatchOps(doc *graph.Document, page *graph.Page, opsList []BatchOp, hb *habits.Store) error {
	if hb == nil {
		hb, _ = habits.OpenStore("")
	}
	for i, op := range opsList {
		switch op.Op {
		case "add_node":
			x, y := 80.0, 80.0
			if op.X != nil {
				x = *op.X
			}
			if op.Y != nil {
				y = *op.Y
			}
			n := hb.NewNodeFromHabits(op.Kind, op.Label, x, y)
			if op.ID != "" {
				n.ID = op.ID
			}
			n.Sublabel = op.Sublabel
			n.ParentID = op.ParentID
			if op.Icon != "" {
				n.Icon = graph.ParseIconRef(op.Icon)
			}
			if op.Fill != "" {
				n.Style.Fill = op.Fill
			}
			if op.Stroke != "" {
				n.Style.Stroke = op.Stroke
			}
			if op.Dash != "" {
				n.Style.Dash = op.Dash
			}
			if op.W != nil {
				n.W = *op.W
			}
			if op.H != nil {
				n.H = *op.H
			}
			if err := page.AddNode(n); err != nil {
				return fmt.Errorf("op[%d] add_node: %w", i, err)
			}
		case "update_node":
			n := page.NodeByID(op.ID)
			if n == nil {
				return fmt.Errorf("op[%d]: node %q not found", i, op.ID)
			}
			if op.Kind != "" {
				n.Kind = op.Kind
			}
			if op.Label != "" {
				n.Label = op.Label
			}
			if op.Sublabel != "" {
				n.Sublabel = op.Sublabel
			}
			if op.Icon != "" {
				n.Icon = graph.ParseIconRef(op.Icon)
			}
			if op.ParentID != "" {
				n.ParentID = op.ParentID
			}
			if op.Fill != "" {
				n.Style.Fill = op.Fill
			}
			if op.Stroke != "" {
				n.Style.Stroke = op.Stroke
			}
			if op.Dash != "" {
				n.Style.Dash = op.Dash
			}
			if op.X != nil {
				n.X = *op.X
			}
			if op.Y != nil {
				n.Y = *op.Y
			}
			if op.W != nil {
				n.W = *op.W
			}
			if op.H != nil {
				n.H = *op.H
			}
			page.MarkDirty()
		case "remove_node":
			if !page.RemoveNode(op.ID) {
				return fmt.Errorf("op[%d]: node %q not found", i, op.ID)
			}
		case "connect":
			e := hb.NewEdgeFromHabits(op.From, op.To, op.Label)
			if op.ID != "" {
				e.ID = op.ID
			}
			if op.Routing != "" {
				e.Routing = graph.RoutingStyle(op.Routing)
			}
			if op.Arrow != "" {
				e.EndArrow = graph.ArrowStyle(op.Arrow)
			}
			if op.Stroke != "" {
				e.Style.Stroke = op.Stroke
			}
			if op.Dash != "" {
				e.Style.Dash = op.Dash
			}
			if err := page.AddEdge(e); err != nil {
				return fmt.Errorf("op[%d] connect: %w", i, err)
			}
		case "disconnect":
			if !page.RemoveEdge(op.ID) {
				return fmt.Errorf("op[%d]: edge %q not found", i, op.ID)
			}
		case "add_page":
			np, err := doc.AddPage(op.ID, op.Name, true)
			if err != nil {
				return fmt.Errorf("op[%d] add_page: %w", i, err)
			}
			page = np
		case "align":
			graph.AlignNodes(page, op.IDs, graph.AlignMode(op.Mode))
		case "zorder":
			graph.ReorderNodes(page, op.IDs, graph.ZOrderMode(op.Mode))
		case "group":
			if _, err := graph.GroupNodes(page, op.IDs, op.Label); err != nil {
				return fmt.Errorf("op[%d] group: %w", i, err)
			}
		case "ungroup":
			graph.UngroupNodes(page, op.IDs)
		case "format":
			opts := layout.OptionsFromHabits(hb.Snapshot(), false)
			if op.Strategy != "" {
				opts.Strategy = op.Strategy
			}
			if op.Dir != "" {
				opts.Direction = op.Dir
			}
			layout.FormatPage(doc, page, opts)
		default:
			return fmt.Errorf("op[%d]: unknown op %q", i, op.Op)
		}
	}
	return nil
}

// CapabilitiesSchema builds the JSON schema describing all CLI & MCP capabilities dynamically.
func CapabilitiesSchema(reg *icons.Registry) map[string]any {
	var packIDs []string
	if reg != nil {
		for _, p := range reg.Packs() {
			packIDs = append(packIDs, fmt.Sprintf("%s (%d icons)", p.ID, len(p.Icons)))
		}
	}
	presets := make([]string, 0)
	for _, pr := range habits.BuiltinPresets() {
		presets = append(presets, pr.ID)
	}
	return map[string]any{
		"schemaVersion":    1,
		"tool":             "drawgraph",
		"version":          version.Version,
		"nodeKinds":        graph.AllNodeKinds,
		"customShapes":     render.RegisteredShapes(),
		"layoutStrategies": layout.RegisteredStrategies(),
		"exportFormats":    export.SupportedExtensions(),
		"languages":        i18n.SupportedLocales(),
		"routings":         []graph.RoutingStyle{graph.RouteOrthogonal, graph.RouteStraight, graph.RouteCurved},
		"arrows":           []graph.ArrowStyle{graph.ArrowClassic, graph.ArrowOpen, graph.ArrowDiamond, graph.ArrowCircle, graph.ArrowNone},
		"presets":          presets,
		"iconPacks":        packIDs,
		"templates":        []string{"blank", "arch", "flow", "microservices", "org"},
		"commands": []string{
			"new", "inspect", "validate", "add-node", "add-image", "add-video", "update-node", "remove-node",
			"connect", "disconnect", "align", "page", "batch", "dsl", "mermaid", "format",
			"import", "export", "icon", "habits", "update", "live", "mcp", "schema", "skill",
		},
	}
}

// ApplyTemplate seeds doc/page with a named curated template ("arch", "flow", "microservices", "org").
func ApplyTemplate(doc *graph.Document, p *graph.Page, name string, hb *habits.Store) error {
	var dsl, strat, dir string
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "blank":
		return nil
	case "arch":
		dsl = `
container cluster "Cloud Cluster"
node client "Web / Mobile" kind:rounded icon:builtin/browser
node gw "API Gateway" kind:rounded icon:builtin/gateway parent:cluster
node auth "Auth Service" kind:rounded icon:builtin/lock parent:cluster
node core "Core Service" kind:rounded icon:builtin/service parent:cluster
node cache "Redis Cache" kind:rounded icon:builtin/cache parent:cluster
node db "Primary DB" kind:cylinder icon:builtin/database parent:cluster
client -> gw : "HTTPS"
gw -> auth : "JWT"
gw -> core : "gRPC"
core -> cache : "GET/SET"
core -> db : "SQL"
`
		strat = "layered"
		dir = "LR"
	case "flow":
		dsl = `
node start "Request In" kind:ellipse
node check "Authorized?" kind:diamond
node work "Process Task" kind:rounded icon:builtin/bot
node reject "Return 401" kind:rounded
node done "Done" kind:ellipse
start -> check
check -> work : "Yes"
check -> reject : "No"
work -> done
`
		strat = "tree"
		dir = "TB"
	case "microservices":
		dsl = `
node mesh "Event Bus / Gateway" kind:hexagon icon:builtin/gateway
node orders "Order Service" icon:builtin/service
node billing "Billing Service" icon:builtin/service
node notify "Notify Worker" icon:builtin/queue
node analytics "AI Insights" icon:builtin/brain
node db "Orders DB" kind:cylinder icon:builtin/database
node store "Object Storage" kind:cloud icon:builtin/storage
mesh -> orders : "REST"
mesh -> billing : "gRPC"
mesh -> notify : "Pub/Sub"
mesh -> analytics : "Stream"
orders -> db : "SQL"
analytics -> store : "Parquet"
`
		strat = "radial"
		dir = "LR"
	case "org":
		dsl = `
node ceo "Engineering Lead" icon:builtin/user
node fe "Frontend Team" icon:builtin/monitor
node be "Platform Team" icon:builtin/server
node ai "AI Agents Team" icon:builtin/bot
node sre "SRE & Security" icon:builtin/firewall
ceo -> fe
ceo -> be
ceo -> ai
be -> sre
`
		strat = "tree"
		dir = "TB"
	default:
		return fmt.Errorf("unknown template %q (supported: blank, arch, flow, microservices, org)", name)
	}
	if _, _, err := ApplyDSL(doc, p, dsl, hb); err != nil {
		return err
	}
	opts := layout.OptionsFromHabits(hb.Snapshot(), false)
	if strat != "" {
		opts.Strategy = strat
	}
	if dir != "" {
		opts.Direction = dir
	}
	layout.FormatPage(doc, p, opts)
	return nil
}

// ApplyDSL parses concise line-oriented DrawGraph DSL statements onto page.
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
		case "layout":
			opts := layout.OptionsFromHabits(hb.Snapshot(), false)
			for _, tok := range tokens[1:] {
				if k, v, ok := strings.Cut(tok, ":"); ok {
					switch strings.ToLower(k) {
					case "strategy", "algo":
						opts.Strategy = v
					case "dir", "direction":
						opts.Direction = v
					case "preset":
						opts.Preset = v
					case "routing":
						opts.Routing = graph.RoutingStyle(v)
					}
				} else {
					opts.Strategy = tok
				}
			}
			layout.FormatPage(doc, page, opts)
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
	page.MarkDirty()
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
	if escaped || inQuote != 0 {
		return nil, fmt.Errorf("unterminated quote or escape")
	}
	flush()
	return out, nil
}
