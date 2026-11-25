// Package agentcli implements the native agent-first DrawGraph CLI (`drawgraph`).
// Every subcommand supports deterministic `--json` output, transactional batching,
// concise DSL drawing, one-click layout formatting, Git icon plugins, and persistent habits.
package agentcli

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/neko233-com/DrawGraph/internal/drawio"
	"github.com/neko233-com/DrawGraph/internal/export"
	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/habits"
	"github.com/neko233-com/DrawGraph/internal/i18n"
	"github.com/neko233-com/DrawGraph/internal/icons"
	"github.com/neko233-com/DrawGraph/internal/ipc"
	"github.com/neko233-com/DrawGraph/internal/layout"
	"github.com/neko233-com/DrawGraph/internal/mcp"
	"github.com/neko233-com/DrawGraph/internal/media"
	"github.com/neko233-com/DrawGraph/internal/mermaid"
	"github.com/neko233-com/DrawGraph/internal/ops"
	"github.com/neko233-com/DrawGraph/internal/update"
	"github.com/neko233-com/DrawGraph/internal/version"
)

// Envelope is the uniform structured JSON output for AI agents.
type Envelope struct {
	OK      bool   `json:"ok"`
	Command string `json:"command"`
	Result  any    `json:"result,omitempty"`
	Error   string `json:"error,omitempty"`
}

// Run executes the CLI with args (excluding argv[0]), reading from stdin and writing to stdout/stderr.
// Returns process exit code.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}
	cmd := args[0]
	rest := args[1:]

	if cmd == "-h" || cmd == "--help" || cmd == "help" {
		printHelp(stdout)
		return 0
	}
	if cmd == "version" || cmd == "--version" || cmd == "-v" {
		return emitResult(stdout, hasJSONFlag(rest), "version", map[string]any{
			"version": version.Version,
			"commit":  version.Commit,
			"date":    version.Date,
		}, fmt.Sprintf("drawgraph %s (%s %s)\n", version.Version, version.Commit, version.Date))
	}

	hb, err := habits.OpenStore("")
	if err != nil {
		return emitErr(stdout, stderr, hasJSONFlag(rest), cmd, err, 1)
	}
	reg, err := icons.OpenRegistry("")
	if err != nil {
		return emitErr(stdout, stderr, hasJSONFlag(rest), cmd, err, 1)
	}

	switch cmd {
	case "schema":
		return cmdSchema(rest, reg, stdout)
	case "skill":
		return cmdSkill(rest, stdout, stderr)
	case "new":
		return cmdNew(rest, hb, stdout, stderr)
	case "inspect":
		return cmdInspect(rest, stdout, stderr)
	case "validate":
		return cmdValidate(rest, stdout, stderr)
	case "add-node":
		return cmdAddNode(rest, hb, stdout, stderr)
	case "add-image":
		return cmdAddMedia(rest, "image", hb, stdout, stderr)
	case "add-video":
		return cmdAddMedia(rest, "video", hb, stdout, stderr)
	case "update-node":
		return cmdUpdateNode(rest, hb, stdout, stderr)
	case "remove-node":
		return cmdRemoveNode(rest, stdout, stderr)
	case "connect":
		return cmdConnect(rest, hb, stdout, stderr)
	case "disconnect":
		return cmdDisconnect(rest, stdout, stderr)
	case "align":
		return cmdAlign(rest, stdout, stderr)
	case "page":
		return cmdPage(rest, stdout, stderr)
	case "batch":
		return cmdBatch(rest, hb, stdin, stdout, stderr)
	case "dsl":
		return cmdDSL(rest, hb, stdin, stdout, stderr)
	case "mermaid":
		return cmdMermaid(rest, hb, stdin, stdout, stderr)
	case "format":
		return cmdFormat(rest, hb, stdout, stderr)
	case "import":
		return cmdImport(rest, stdout, stderr)
	case "export":
		return cmdExport(rest, reg, stdout, stderr)
	case "icon":
		return cmdIcon(rest, reg, stdout, stderr)
	case "habits":
		return cmdHabits(rest, hb, stdout, stderr)
	case "update":
		return cmdUpdate(rest, hb, stdout, stderr)
	case "live":
		return cmdLive(rest, stdout, stderr)
	case "mcp":
		return cmdMCP(rest, reg, hb, stdin, stdout, stderr)
	default:
		return emitErr(stdout, stderr, hasJSONFlag(rest), cmd, fmt.Errorf("unknown command %q (run `drawgraph --help`)", cmd), 2)
	}
}

func cmdSchema(args []string, reg *icons.Registry, stdout io.Writer) int {
	return emitResult(stdout, true, "schema", ops.CapabilitiesSchema(reg), "")
}

func cmdSkill(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("skill", flag.ContinueOnError)
	fs.SetOutput(stderr)
	installDir := fs.String("install", "", "optional directory to write SKILL.md into")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *installDir != "" {
		dest := filepath.Join(*installDir, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "skill", err, 1)
		}
		if err := os.WriteFile(dest, []byte(agentSkillMarkdown), 0o644); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "skill", err, 1)
		}
		return emitResult(stdout, *jsonOut, "skill", map[string]any{"written": dest}, fmt.Sprintf("Installed DrawGraph SKILL.md to %s\n", dest))
	}
	if *jsonOut {
		return emitResult(stdout, true, "skill", map[string]any{"markdown": agentSkillMarkdown}, "")
	}
	_, _ = io.WriteString(stdout, agentSkillMarkdown)
	return 0
}

func cmdNew(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("new", flag.ContinueOnError)
	fs.SetOutput(stderr)
	title := fs.String("title", "", "diagram title")
	tpl := fs.String("template", "blank", "template: blank | arch | flow | microservices | org")
	jsonOut := fs.Bool("json", false, "output JSON")
	live := fs.Bool("live", false, "also push to running GUI window")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "new", fmt.Errorf("usage: drawgraph new <file.drawgraph> [--template blank|arch|flow|microservices|org]"), 2)
	}
	file := fs.Arg(0)
	t := *title
	if t == "" {
		t = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
	}
	doc := graph.NewDocument(t)
	hb.ApplyToDocument(doc)

	p := doc.ActivePage()
	if err := ops.ApplyTemplate(doc, p, *tpl, hb); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "new", err, 2)
	}

	if err := saveDoc(file, doc, *live, "new"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "new", err, 1)
	}
	hb.RecordRecentFile(file)
	return emitResult(stdout, *jsonOut, "new", map[string]any{
		"file":  file,
		"title": doc.Title,
		"nodes": len(p.Nodes),
		"edges": len(p.Edges),
	}, fmt.Sprintf("Created %s (%d nodes, %d edges)\n", file, len(p.Nodes), len(p.Edges)))
}

func cmdInspect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page ID to inspect")
	jsonOut := fs.Bool("json", false, "output JSON")
	live := fs.Bool("live", false, "inspect the currently open GUI document")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	var doc *graph.Document
	var err error
	file := fs.Arg(0)
	if *live && file == "" {
		ep, derr := ipc.Discover("")
		if derr != nil {
			return emitErr(stdout, stderr, *jsonOut, "inspect", derr, 1)
		}
		doc, err = ipc.PullDocument(ep)
	} else {
		if file == "" {
			return emitErr(stdout, stderr, *jsonOut, "inspect", fmt.Errorf("usage: drawgraph inspect <file> [--json]"), 2)
		}
		doc, err = loadAnyDoc(file)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "inspect", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "inspect", err, 2)
	}
	bounds, _ := p.Bounds()
	issues := doc.Validate()
	pages := make([]map[string]any, 0, len(doc.Pages))
	for _, pg := range doc.Pages {
		pages = append(pages, map[string]any{
			"id": pg.ID, "name": pg.Name, "nodes": len(pg.Nodes), "edges": len(pg.Edges),
		})
	}
	summary := map[string]any{
		"title":      doc.Title,
		"activePage": p.ID,
		"pages":      pages,
		"bounds":     bounds,
		"nodes":      p.Nodes,
		"edges":      p.Edges,
		"issues":     issues,
	}
	text := fmt.Sprintf("%s [page %s]: %d nodes, %d edges, %d validation issues\n",
		doc.Title, p.ID, len(p.Nodes), len(p.Edges), len(issues))
	return emitResult(stdout, *jsonOut, "inspect", summary, text)
}

func cmdValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "validate", fmt.Errorf("usage: drawgraph validate <file>"), 2)
	}
	doc, err := loadAnyDoc(fs.Arg(0))
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "validate", err, 1)
	}
	issues := doc.Validate()
	hasErr := false
	for _, is := range issues {
		if is.Severity == "error" {
			hasErr = true
			break
		}
	}
	code := 0
	if hasErr {
		code = 2
	}
	res := map[string]any{"valid": !hasErr, "issues": issues, "count": len(issues)}
	if *jsonOut {
		_ = writeJSON(stdout, Envelope{OK: !hasErr, Command: "validate", Result: res})
		return code
	}
	_, _ = fmt.Fprintf(stdout, "valid=%v issues=%d\n", !hasErr, len(issues))
	for _, is := range issues {
		_, _ = fmt.Fprintf(stdout, "  [%s] %s (%s): %s\n", is.Severity, is.Code, is.TargetID, is.Message)
	}
	return code
}

func cmdAddNode(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("add-node", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	id := fs.String("id", "", "node id")
	kind := fs.String("kind", "", "node shape kind")
	label := fs.String("label", "", "node label")
	sublabel := fs.String("sublabel", "", "secondary sublabel")
	icon := fs.String("icon", "", "icon ref (packId/name)")
	parent := fs.String("parent", "", "parent container node id")
	fill := fs.String("fill", "", "fill hex color")
	stroke := fs.String("stroke", "", "stroke hex color")
	x := fs.Float64("x", 80, "x coordinate")
	y := fs.Float64("y", 80, "y coordinate")
	w := fs.Float64("w", 0, "width override")
	h := fs.Float64("h", 0, "height override")
	remember := fs.Bool("remember", false, "remember style/icon in persistent habits")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "add-node", fmt.Errorf("usage: drawgraph add-node <file> --label ..."), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "add-node", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "add-node", err, 2)
	}
	n := hb.NewNodeFromHabits(graph.NodeKind(*kind), *label, *x, *y)
	if *id != "" {
		n.ID = *id
	}
	if *sublabel != "" {
		n.Sublabel = *sublabel
	}
	if *icon != "" {
		n.Icon = graph.ParseIconRef(*icon)
	}
	if *parent != "" {
		n.ParentID = *parent
	}
	if *fill != "" {
		n.Style.Fill = *fill
	}
	if *stroke != "" {
		n.Style.Stroke = *stroke
	}
	if *w > 0 {
		n.W = *w
	}
	if *h > 0 {
		n.H = *h
	}
	if err := p.AddNode(n); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "add-node", err, 2)
	}
	if *remember {
		hb.RecordNodeHabit(n)
	} else if n.Icon.Valid() {
		hb.RecordIconUse(n.Icon.String())
	}
	if err := saveDoc(file, doc, *live, "add-node"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "add-node", err, 1)
	}
	return emitResult(stdout, *jsonOut, "add-node", n, fmt.Sprintf("Added node %s (%s)\n", n.ID, n.Kind))
}

func cmdAddMedia(args []string, mediaKind string, hb *habits.Store, stdout, stderr io.Writer) int {
	cmdName := "add-" + mediaKind
	fs := flag.NewFlagSet(cmdName, flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	src := fs.String("src", "", "path to image or video file")
	id := fs.String("id", "", "node id")
	label := fs.String("label", "", "optional label/caption")
	parent := fs.String("parent", "", "parent container node id")
	fit := fs.String("fit", "contain", "contain | cover | fill")
	maxSide := fs.Float64("max-side", 0, "reasonable max display side in DIPs (defaults to user habit 480)")
	native := fs.Bool("native", false, "size node 1:1 to original pixel resolution")
	showBar := fs.Bool("show-bar", false, "show caption footer bar")
	loop := fs.Bool("loop", true, "loop video playback")
	muted := fs.Bool("muted", true, "mute video audio by default")
	x := fs.Float64("x", 80, "x coordinate")
	y := fs.Float64("y", 80, "y coordinate")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *src == "" {
		return emitErr(stdout, stderr, *jsonOut, cmdName, fmt.Errorf("usage: drawgraph %s <file> --src <media-path>", cmdName), 2)
	}
	file := fs.Arg(0)
	doc, err := loadOrCreateDoc(file, hb)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, cmdName, err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, cmdName, err, 2)
	}
	ms := *maxSide
	if ms <= 0 {
		ms = hb.Snapshot().DefaultMediaMax
	}
	var asset *graph.MediaAsset
	var dw, dh float64
	if mediaKind == "image" {
		asset, dw, dh, err = media.ImportImageFile(doc, *src, ms)
	} else {
		asset, dw, dh, err = media.ImportVideoFile(doc, *src, ms)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, cmdName, err, 1)
	}
	if *native && asset.NaturalW > 0 && asset.NaturalH > 0 {
		dw, dh = float64(asset.NaturalW), float64(asset.NaturalH)
	}
	lbl := *label
	if lbl == "" {
		lbl = asset.Name
	}
	kind := graph.KindImage
	if mediaKind == "video" {
		kind = graph.KindVideo
	}
	n := &graph.Node{
		ID:       *id,
		Kind:     kind,
		ParentID: *parent,
		X:        *x,
		Y:        *y,
		W:        dw,
		H:        dh,
		Label:    lbl,
		Media: &graph.NodeMedia{
			AssetID:  asset.ID,
			Fit:      *fit,
			Autoplay: mediaKind == "video",
			Loop:     *loop,
			Muted:    *muted,
			ShowBar:  *showBar,
		},
	}
	if err := p.AddNode(n); err != nil {
		return emitErr(stdout, stderr, *jsonOut, cmdName, err, 2)
	}
	if err := saveDoc(file, doc, *live, cmdName); err != nil {
		return emitErr(stdout, stderr, *jsonOut, cmdName, err, 1)
	}
	return emitResult(stdout, *jsonOut, cmdName, map[string]any{
		"node": n, "asset": asset,
	}, fmt.Sprintf("Added %s node %s (%dx%d -> %.0fx%.0f)\n", mediaKind, n.ID, asset.NaturalW, asset.NaturalH, n.W, n.H))
}

func cmdUpdateNode(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("update-node", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	id := fs.String("id", "", "target node id")
	label := fs.String("label", "", "new label")
	sublabel := fs.String("sublabel", "", "new sublabel")
	icon := fs.String("icon", "", "icon ref (packId/name or 'none')")
	fill := fs.String("fill", "", "fill color")
	stroke := fs.String("stroke", "", "stroke color")
	x := fs.String("x", "", "new x")
	y := fs.String("y", "", "new y")
	w := fs.String("w", "", "new w")
	h := fs.String("h", "", "new h")
	remember := fs.Bool("remember", false, "remember style in persistent habits")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *id == "" {
		return emitErr(stdout, stderr, *jsonOut, "update-node", fmt.Errorf("usage: drawgraph update-node <file> --id <nodeId> ..."), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "update-node", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "update-node", err, 2)
	}
	n := p.NodeByID(*id)
	if n == nil {
		return emitErr(stdout, stderr, *jsonOut, "update-node", fmt.Errorf("node %q not found", *id), 2)
	}
	if *label != "" {
		n.Label = *label
	}
	if *sublabel != "" {
		n.Sublabel = *sublabel
	}
	if *icon == "none" {
		n.Icon = nil
	} else if *icon != "" {
		n.Icon = graph.ParseIconRef(*icon)
	}
	if *fill != "" {
		n.Style.Fill = *fill
	}
	if *stroke != "" {
		n.Style.Stroke = *stroke
	}
	setOptFloat(*x, &n.X)
	setOptFloat(*y, &n.Y)
	setOptFloat(*w, &n.W)
	setOptFloat(*h, &n.H)
	graph.NormalizeNode(n)
	if *remember {
		hb.RecordNodeHabit(n)
	}
	if err := saveDoc(file, doc, *live, "update-node"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "update-node", err, 1)
	}
	return emitResult(stdout, *jsonOut, "update-node", n, fmt.Sprintf("Updated node %s\n", n.ID))
}

func cmdRemoveNode(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("remove-node", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	id := fs.String("id", "", "node id")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *id == "" {
		return emitErr(stdout, stderr, *jsonOut, "remove-node", fmt.Errorf("usage: drawgraph remove-node <file> --id <nodeId>"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "remove-node", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "remove-node", err, 2)
	}
	if !p.RemoveNode(*id) {
		return emitErr(stdout, stderr, *jsonOut, "remove-node", fmt.Errorf("node %q not found", *id), 2)
	}
	if err := saveDoc(file, doc, *live, "remove-node"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "remove-node", err, 1)
	}
	return emitResult(stdout, *jsonOut, "remove-node", map[string]any{"removed": *id}, fmt.Sprintf("Removed node %s\n", *id))
}

func cmdConnect(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("connect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	id := fs.String("id", "", "edge id")
	from := fs.String("from", "", "source node id")
	to := fs.String("to", "", "target node id")
	label := fs.String("label", "", "edge label")
	routing := fs.String("routing", "", "orthogonal | straight | curved")
	arrow := fs.String("arrow", "", "arrow | open | diamond | circle | none")
	remember := fs.Bool("remember", false, "remember routing/arrow in habits")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *from == "" || *to == "" {
		return emitErr(stdout, stderr, *jsonOut, "connect", fmt.Errorf("usage: drawgraph connect <file> --from <src> --to <dst>"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "connect", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "connect", err, 2)
	}
	e := hb.NewEdgeFromHabits(*from, *to, *label)
	if *id != "" {
		e.ID = *id
	}
	if *routing != "" {
		e.Routing = graph.RoutingStyle(*routing)
	}
	if *arrow != "" {
		e.EndArrow = graph.ArrowStyle(*arrow)
	}
	if err := p.AddEdge(e); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "connect", err, 2)
	}
	if *remember {
		hb.RecordEdgeHabit(e)
	}
	if err := saveDoc(file, doc, *live, "connect"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "connect", err, 1)
	}
	return emitResult(stdout, *jsonOut, "connect", e, fmt.Sprintf("Connected %s -> %s (%s)\n", e.SourceID, e.TargetID, e.ID))
}

func cmdDisconnect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("disconnect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	id := fs.String("id", "", "edge id")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *id == "" {
		return emitErr(stdout, stderr, *jsonOut, "disconnect", fmt.Errorf("usage: drawgraph disconnect <file> --id <edgeId>"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "disconnect", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "disconnect", err, 2)
	}
	if !p.RemoveEdge(*id) {
		return emitErr(stdout, stderr, *jsonOut, "disconnect", fmt.Errorf("edge %q not found", *id), 2)
	}
	if err := saveDoc(file, doc, *live, "disconnect"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "disconnect", err, 1)
	}
	return emitResult(stdout, *jsonOut, "disconnect", map[string]any{"removed": *id}, fmt.Sprintf("Removed edge %s\n", *id))
}

func cmdPage(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return emitErr(stdout, stderr, false, "page", fmt.Errorf("usage: drawgraph page <list|add|switch|remove> <file> ..."), 2)
	}
	sub := args[0]
	rest := args[1:]
	fs := flag.NewFlagSet("page", flag.ContinueOnError)
	fs.SetOutput(stderr)
	name := fs.String("name", "", "page name")
	id := fs.String("id", "", "page id")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(rest)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "page", fmt.Errorf("missing file path"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "page", err, 1)
	}
	switch sub {
	case "list":
		return emitResult(stdout, *jsonOut, "page.list", doc.Pages, fmt.Sprintf("%d pages\n", len(doc.Pages)))
	case "add":
		p, err := doc.AddPage(*id, *name, true)
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "page.add", err, 2)
		}
		if err := graph.SaveAtomic(file, doc); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "page.add", err, 1)
		}
		return emitResult(stdout, *jsonOut, "page.add", p, fmt.Sprintf("Added page %s (%s)\n", p.ID, p.Name))
	case "switch":
		if *id == "" {
			return emitErr(stdout, stderr, *jsonOut, "page.switch", fmt.Errorf("--id required"), 2)
		}
		if doc.PageByID(*id) == nil {
			return emitErr(stdout, stderr, *jsonOut, "page.switch", fmt.Errorf("page %q not found", *id), 2)
		}
		doc.ActivePageID = *id
		if err := graph.SaveAtomic(file, doc); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "page.switch", err, 1)
		}
		return emitResult(stdout, *jsonOut, "page.switch", map[string]any{"activePage": *id}, fmt.Sprintf("Active page: %s\n", *id))
	case "remove":
		if *id == "" || len(doc.Pages) <= 1 {
			return emitErr(stdout, stderr, *jsonOut, "page.remove", fmt.Errorf("--id required and cannot remove last page"), 2)
		}
		kept := doc.Pages[:0]
		removed := false
		for _, p := range doc.Pages {
			if p.ID == *id {
				removed = true
				continue
			}
			kept = append(kept, p)
		}
		if !removed {
			return emitErr(stdout, stderr, *jsonOut, "page.remove", fmt.Errorf("page %q not found", *id), 2)
		}
		doc.Pages = kept
		doc.Normalize()
		if err := graph.SaveAtomic(file, doc); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "page.remove", err, 1)
		}
		return emitResult(stdout, *jsonOut, "page.remove", map[string]any{"removed": *id}, fmt.Sprintf("Removed page %s\n", *id))
	default:
		return emitErr(stdout, stderr, *jsonOut, "page", fmt.Errorf("unknown page subcommand %q", sub), 2)
	}
}

func cmdBatch(args []string, hb *habits.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("batch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	opsFile := fs.String("ops-file", "", "JSON operations file")
	useStdin := fs.Bool("stdin", false, "read JSON operations from stdin")
	autoFormat := fs.Bool("format", false, "run one-click format after applying ops")
	dryRun := fs.Bool("dry-run", false, "validate without writing file")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "batch", fmt.Errorf("usage: drawgraph batch <file> [--ops-file ops.json | --stdin]"), 2)
	}
	file := fs.Arg(0)
	var raw []byte
	var err error
	if *useStdin || *opsFile == "" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(*opsFile)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "batch", err, 1)
	}
	var opsList []ops.BatchOp
	if err := json.Unmarshal(raw, &opsList); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "batch", fmt.Errorf("parse batch ops JSON: %w", err), 2)
	}

	doc, err := loadOrCreateDoc(file, hb)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "batch", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "batch", err, 2)
	}
	if err := ops.ExecuteBatchOps(doc, p, opsList, hb); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "batch", err, 2)
	}
	if *autoFormat {
		layout.FormatPage(doc, p, layout.OptionsFromHabits(hb.Snapshot(), false))
	}
	if !*dryRun {
		if err := saveDoc(file, doc, *live, "batch"); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "batch", err, 1)
		}
	}
	return emitResult(stdout, *jsonOut, "batch", map[string]any{
		"file": file, "ops": len(opsList), "nodes": len(p.Nodes), "edges": len(p.Edges), "dryRun": *dryRun,
	}, fmt.Sprintf("Applied %d ops to %s (%d nodes, %d edges)\n", len(opsList), file, len(p.Nodes), len(p.Edges)))
}

func cmdDSL(args []string, hb *habits.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dsl", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	dslFile := fs.String("dsl-file", "", "DSL text file")
	useStdin := fs.Bool("stdin", false, "read DSL from stdin")
	autoFormat := fs.Bool("format", true, "auto-format after applying DSL")
	strategy := fs.String("strategy", "", "layered | tree | grid | radial | swimlane | compact")
	dir := fs.String("direction", "", "LR | RL | TB | BT layout direction")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "dsl", fmt.Errorf("usage: drawgraph dsl <file> [--dsl-file arch.dg | --stdin]"), 2)
	}
	file := fs.Arg(0)
	var raw []byte
	var err error
	if *useStdin || *dslFile == "" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(*dslFile)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "dsl", err, 1)
	}
	doc, err := loadOrCreateDoc(file, hb)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "dsl", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "dsl", err, 2)
	}
	addedN, addedE, err := ApplyDSL(doc, p, string(raw), hb)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "dsl", err, 2)
	}
	if *autoFormat {
		opts := layout.OptionsFromHabits(hb.Snapshot(), false)
		if *strategy != "" {
			opts.Strategy = *strategy
		}
		if *dir != "" {
			opts.Direction = strings.ToUpper(*dir)
		}
		layout.FormatPage(doc, p, opts)
	}
	if err := saveDoc(file, doc, *live, "dsl"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "dsl", err, 1)
	}
	return emitResult(stdout, *jsonOut, "dsl", map[string]any{
		"file": file, "addedNodes": addedN, "addedEdges": addedE, "totalNodes": len(p.Nodes), "totalEdges": len(p.Edges),
	}, fmt.Sprintf("DSL applied to %s (+%d nodes, +%d edges)\n", file, addedN, addedE))
}

func cmdMermaid(args []string, hb *habits.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("mermaid", flag.ContinueOnError)
	fs.SetOutput(stderr)
	mmdFile := fs.String("mermaid-file", "", "Mermaid source file")
	useStdin := fs.Bool("stdin", false, "read Mermaid source from stdin")
	strategy := fs.String("strategy", "", "optional layout strategy override")
	dir := fs.String("direction", "", "LR | RL | TB | BT override")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "mermaid", fmt.Errorf("usage: drawgraph mermaid <out.drawgraph> [--mermaid-file in.mmd | --stdin]"), 2)
	}
	file := fs.Arg(0)
	var raw []byte
	var err error
	if *useStdin || *mmdFile == "" {
		raw, err = io.ReadAll(stdin)
	} else {
		raw, err = os.ReadFile(*mmdFile)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "mermaid", err, 1)
	}
	doc, err := mermaid.Parse(string(raw), hb)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "mermaid", err, 2)
	}
	p := doc.ActivePage()
	if *strategy != "" || *dir != "" {
		opts := layout.OptionsFromHabits(hb.Snapshot(), false)
		if *strategy != "" {
			opts.Strategy = *strategy
		}
		if *dir != "" {
			opts.Direction = strings.ToUpper(*dir)
		}
		layout.FormatPage(doc, p, opts)
	}
	if err := saveDoc(file, doc, *live, "mermaid"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "mermaid", err, 1)
	}
	return emitResult(stdout, *jsonOut, "mermaid", map[string]any{
		"file": file, "nodes": len(p.Nodes), "edges": len(p.Edges),
	}, fmt.Sprintf("Imported Mermaid into %s (%d nodes, %d edges)\n", file, len(p.Nodes), len(p.Edges)))
}

func cmdAlign(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("align", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	idsStr := fs.String("ids", "", "comma-separated node IDs (default: all nodes)")
	mode := fs.String("mode", "left", "left|center-x|right|top|center-y|bottom|distribute-h|distribute-v")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "align", fmt.Errorf("usage: drawgraph align <file> --ids a,b,c --mode left"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "align", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "align", err, 2)
	}
	var ids []string
	if strings.TrimSpace(*idsStr) != "" {
		for _, part := range strings.Split(*idsStr, ",") {
			if s := strings.TrimSpace(part); s != "" {
				ids = append(ids, s)
			}
		}
	} else {
		for _, n := range p.Nodes {
			ids = append(ids, n.ID)
		}
	}
	changed := graph.AlignNodes(p, ids, graph.AlignMode(*mode))
	if err := saveDoc(file, doc, *live, "align"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "align", err, 1)
	}
	return emitResult(stdout, *jsonOut, "align", map[string]any{"changed": changed, "mode": *mode, "count": len(ids)},
		fmt.Sprintf("Aligned %d nodes (%s)\n", len(ids), *mode))
}

func cmdFormat(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("format", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pageID := fs.String("page", "", "page id")
	strategy := fs.String("strategy", "", "layered | tree | grid | radial | swimlane | compact")
	dir := fs.String("direction", "", "LR | RL | TB | BT")
	routing := fs.String("routing", "", "orthogonal | straight | curved")
	preset := fs.String("preset", "", "lobe-clean | drawio-classic | blueprint | slate-mono | vibrant-tech | warm-paper")
	dark := fs.Bool("dark", false, "harmonize colors for dark theme")
	gapX := fs.Float64("gap-x", 0, "horizontal gap")
	gapY := fs.Float64("gap-y", 0, "vertical gap")
	noStyle := fs.Bool("no-style", false, "skip palette/style harmonization")
	remember := fs.Bool("remember", true, "remember direction/routing/preset in persistent user habits")
	live := fs.Bool("live", false, "push to live GUI")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "format", fmt.Errorf("usage: drawgraph format <file> [--strategy layered|tree|grid|radial|swimlane|compact] [--direction LR|RL|TB|BT]"), 2)
	}
	file := fs.Arg(0)
	doc, err := loadAnyDoc(file)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "format", err, 1)
	}
	p, err := selectPage(doc, *pageID)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "format", err, 2)
	}
	opts := layout.OptionsFromHabits(hb.Snapshot(), *dark)
	if *strategy != "" {
		opts.Strategy = *strategy
	}
	if *dir != "" {
		opts.Direction = strings.ToUpper(*dir)
	}
	if *routing != "" {
		opts.Routing = graph.RoutingStyle(*routing)
	}
	if *preset != "" {
		opts.Preset = *preset
	}
	if *gapX > 0 {
		opts.GapX = *gapX
	}
	if *gapY > 0 {
		opts.GapY = *gapY
	}
	if *noStyle {
		opts.HarmonizeStyles = false
	}
	rep := layout.FormatPage(doc, p, opts)
	if *remember {
		hb.RecordFormatHabit(opts.Direction, opts.Routing, opts.Preset, opts.GapX, opts.GapY, opts.GridSize)
		_ = hb.Update(func(pr *habits.Profile) {
			if opts.Strategy != "" {
				pr.DefaultStrategy = opts.Strategy
			}
		})
	}
	if err := saveDoc(file, doc, *live, "format"); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "format", err, 1)
	}
	return emitResult(stdout, *jsonOut, "format", rep, fmt.Sprintf("Formatted %s (%s %s, %d layers, %d moved, %d edges rerouted)\n",
		file, rep.Strategy, rep.Direction, rep.Layers, rep.MovedNodes, rep.ReroutedEdges))
}

func cmdImport(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("import", flag.ContinueOnError)
	fs.SetOutput(stderr)
	to := fs.String("to", "", "output .drawgraph path")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		return emitErr(stdout, stderr, *jsonOut, "import", fmt.Errorf("usage: drawgraph import <input.drawio> --to <out.drawgraph>"), 2)
	}
	in := fs.Arg(0)
	out := *to
	if out == "" {
		out = strings.TrimSuffix(in, filepath.Ext(in)) + ".drawgraph"
	}
	doc, err := drawio.ImportFile(in)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "import", err, 1)
	}
	if err := graph.SaveAtomic(out, doc); err != nil {
		return emitErr(stdout, stderr, *jsonOut, "import", err, 1)
	}
	p := doc.ActivePage()
	return emitResult(stdout, *jsonOut, "import", map[string]any{
		"source": in, "output": out, "pages": len(doc.Pages), "nodes": len(p.Nodes), "edges": len(p.Edges),
	}, fmt.Sprintf("Imported %s -> %s (%d pages)\n", in, out, len(doc.Pages)))
}

func cmdExport(args []string, reg *icons.Registry, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.SetOutput(stderr)
	to := fs.String("to", "", "output path (.png | .svg | .drawio)")
	pageID := fs.String("page", "", "page id")
	dark := fs.Bool("dark", false, "render in dark theme")
	scale := fs.Float64("scale", 2, "PNG pixel scale")
	jsonOut := fs.Bool("json", false, "output JSON")
	if err := fs.Parse(reorderFlags(args)); err != nil {
		return 2
	}
	if fs.NArg() < 1 || *to == "" {
		return emitErr(stdout, stderr, *jsonOut, "export", fmt.Errorf("usage: drawgraph export <file> --to <out.png|out.svg|out.drawio>"), 2)
	}
	in := fs.Arg(0)
	doc, err := loadAnyDoc(in)
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "export", err, 1)
	}
	ext := strings.ToLower(filepath.Ext(*to))
	if ext == ".drawio" || ext == ".xml" {
		err = drawio.ExportFile(*to, doc)
	} else {
		err = export.ExportFile(*to, doc, *pageID, reg, *dark, *scale)
	}
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "export", err, 1)
	}
	return emitResult(stdout, *jsonOut, "export", map[string]any{"input": in, "output": *to}, fmt.Sprintf("Exported %s -> %s\n", in, *to))
}

func cmdIcon(args []string, reg *icons.Registry, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return emitErr(stdout, stderr, false, "icon", fmt.Errorf("usage: drawgraph icon <list|search|install|sync|import-svg|remove|enable|disable>"), 2)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "list":
		fs := flag.NewFlagSet("icon list", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		packs := reg.Packs()
		var lines []string
		for _, p := range packs {
			lines = append(lines, fmt.Sprintf("%-18s %-8s enabled=%v icons=%d source=%s", p.ID, p.SourceKind, p.Enabled, len(p.Icons), p.Source))
		}
		return emitResult(stdout, *jsonOut, "icon.list", packs, strings.Join(lines, "\n")+"\n")
	case "search":
		fs := flag.NewFlagSet("icon search", flag.ContinueOnError)
		fs.SetOutput(stderr)
		packID := fs.String("pack", "", "filter by pack id")
		cat := fs.String("category", "", "filter by category")
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		q := strings.Join(fs.Args(), " ")
		hits := reg.Search(q, *packID, *cat)
		var lines []string
		for _, ic := range hits {
			lines = append(lines, fmt.Sprintf("%-28s [%s] %s", ic.Ref(), ic.Category, ic.Title))
		}
		txt := fmt.Sprintf("Found %d icons\n%s\n", len(hits), strings.Join(lines, "\n"))
		return emitResult(stdout, *jsonOut, "icon.search", hits, txt)
	case "install":
		fs := flag.NewFlagSet("icon install", flag.ContinueOnError)
		fs.SetOutput(stderr)
		id := fs.String("id", "", "pack id override")
		name := fs.String("name", "", "display name override")
		ref := fs.String("ref", "", "git branch or tag")
		subdir := fs.String("subdir", "", "subdirectory containing SVGs")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "icon.install", fmt.Errorf("usage: drawgraph icon install <git-url-or-dir>"), 2)
		}
		src := fs.Arg(0)
		var pack *icons.IconPack
		var err error
		if st, serr := os.Stat(src); serr == nil && st.IsDir() {
			pack, err = reg.InstallLocal(src, *id, *name, *subdir)
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			pack, err = reg.InstallGit(ctx, icons.GitInstallOptions{
				URL: src, ID: *id, Name: *name, Ref: *ref, Subdir: *subdir,
			})
		}
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "icon.install", err, 1)
		}
		if ep, derr := ipc.Discover(""); derr == nil {
			_ = ipc.ReloadIcons(ep)
		}
		return emitResult(stdout, *jsonOut, "icon.install", pack, fmt.Sprintf("Installed icon pack %q (%d icons)\n", pack.ID, len(pack.Icons)))
	case "sync":
		fs := flag.NewFlagSet("icon sync", flag.ContinueOnError)
		fs.SetOutput(stderr)
		all := fs.Bool("all", false, "sync all external packs")
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		var synced []*icons.IconPack
		if *all || fs.NArg() == 0 {
			for _, p := range reg.Packs() {
				if p.Builtin {
					continue
				}
				pk, err := reg.SyncPack(ctx, p.ID)
				if err != nil {
					return emitErr(stdout, stderr, *jsonOut, "icon.sync", err, 1)
				}
				synced = append(synced, pk)
			}
		} else {
			pk, err := reg.SyncPack(ctx, fs.Arg(0))
			if err != nil {
				return emitErr(stdout, stderr, *jsonOut, "icon.sync", err, 1)
			}
			synced = append(synced, pk)
		}
		return emitResult(stdout, *jsonOut, "icon.sync", synced, fmt.Sprintf("Synced %d icon packs\n", len(synced)))
	case "import-svg":
		fs := flag.NewFlagSet("icon import-svg", flag.ContinueOnError)
		fs.SetOutput(stderr)
		packID := fs.String("pack", "custom", "target pack id")
		name := fs.String("name", "", "icon name")
		cat := fs.String("category", "custom", "icon category")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "icon.import-svg", fmt.Errorf("usage: drawgraph icon import-svg <file.svg>"), 2)
		}
		entry, err := reg.ImportSVG(fs.Arg(0), *packID, *name, *cat)
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "icon.import-svg", err, 1)
		}
		return emitResult(stdout, *jsonOut, "icon.import-svg", entry, fmt.Sprintf("Imported icon %s\n", entry.Ref()))
	case "remove":
		fs := flag.NewFlagSet("icon remove", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "icon.remove", fmt.Errorf("usage: drawgraph icon remove <pack-id>"), 2)
		}
		if err := reg.RemovePack(fs.Arg(0)); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "icon.remove", err, 1)
		}
		return emitResult(stdout, *jsonOut, "icon.remove", map[string]any{"removed": fs.Arg(0)}, fmt.Sprintf("Removed icon pack %s\n", fs.Arg(0)))
	case "enable", "disable":
		fs := flag.NewFlagSet("icon toggle", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "icon."+sub, fmt.Errorf("usage: drawgraph icon %s <pack-id>", sub), 2)
		}
		if err := reg.SetPackEnabled(fs.Arg(0), sub == "enable"); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "icon."+sub, err, 1)
		}
		return emitResult(stdout, *jsonOut, "icon."+sub, map[string]any{"pack": fs.Arg(0), "enabled": sub == "enable"}, fmt.Sprintf("Pack %s enabled=%v\n", fs.Arg(0), sub == "enable"))
	default:
		return emitErr(stdout, stderr, hasJSONFlag(rest), "icon", fmt.Errorf("unknown icon subcommand %q", sub), 2)
	}
}

func cmdHabits(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return emitErr(stdout, stderr, false, "habits", fmt.Errorf("usage: drawgraph habits <show|set|presets|reset|export|import>"), 2)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "show":
		p := hb.Snapshot()
		return emitResult(stdout, hasJSONFlag(rest), "habits.show", p, fmt.Sprintf("Habits (%s): preset=%s direction=%s routing=%s autoRemember=%v\n",
			hb.Path(), p.StylePreset, p.PreferredDirection, p.DefaultRouting, p.AutoRemember))
	case "presets":
		return emitResult(stdout, hasJSONFlag(rest), "habits.presets", habits.BuiltinPresets(), fmt.Sprintf("%d presets available\n", len(habits.BuiltinPresets())))
	case "reset":
		if err := hb.Reset(); err != nil {
			return emitErr(stdout, stderr, hasJSONFlag(rest), "habits.reset", err, 1)
		}
		return emitResult(stdout, hasJSONFlag(rest), "habits.reset", hb.Snapshot(), "Reset habits to defaults\n")
	case "set":
		fs := flag.NewFlagSet("habits set", flag.ContinueOnError)
		fs.SetOutput(stderr)
		theme := fs.String("theme", "", "auto | light | dark")
		preset := fs.String("preset", "", "style preset")
		lang := fs.String("language", "", "auto | en | zh-CN | zh-TW | ja | ko")
		strategy := fs.String("strategy", "", "layered | tree | grid | radial | swimlane | compact")
		wheelMode := fs.String("wheel-mode", "", "pan | zoom")
		dir := fs.String("direction", "", "LR | RL | TB | BT")
		routing := fs.String("routing", "", "orthogonal | straight | curved")
		arrow := fs.String("arrow", "", "arrow | open | diamond | circle | none")
		fill := fs.String("fill", "", "default fill hex")
		stroke := fs.String("stroke", "", "default stroke hex")
		radius := fs.String("radius", "", "default corner radius")
		grid := fs.String("grid", "", "default grid size")
		gapX := fs.String("gap-x", "", "layout horizontal gap")
		gapY := fs.String("gap-y", "", "layout vertical gap")
		autoRem := fs.String("auto-remember", "", "true | false")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		err := hb.Update(func(p *habits.Profile) {
			if *theme != "" {
				p.ThemeMode = *theme
			}
			if *lang != "" {
				p.Language = i18n.NormalizeTag(*lang)
			}
			if *strategy != "" {
				p.DefaultStrategy = *strategy
			}
			if *wheelMode == "pan" || *wheelMode == "zoom" {
				p.WheelMode = *wheelMode
			}
			if *preset != "" {
				p.StylePreset = *preset
			}
			if d := strings.ToUpper(*dir); d == "LR" || d == "RL" || d == "TB" || d == "BT" {
				p.PreferredDirection = d
			}
			if *routing != "" {
				p.DefaultRouting = graph.RoutingStyle(*routing)
			}
			if *arrow != "" {
				p.DefaultArrow = graph.ArrowStyle(*arrow)
			}
			if *fill != "" {
				p.DefaultFill = *fill
			}
			if *stroke != "" {
				p.DefaultStroke = *stroke
			}
			setOptFloat(*radius, &p.DefaultRadius)
			setOptFloat(*grid, &p.GridSize)
			setOptFloat(*gapX, &p.GapX)
			setOptFloat(*gapY, &p.GapY)
			if *autoRem == "true" {
				p.AutoRemember = true
			} else if *autoRem == "false" {
				p.AutoRemember = false
			}
		})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.set", err, 1)
		}
		return emitResult(stdout, *jsonOut, "habits.set", hb.Snapshot(), "Updated global habits\n")
	case "export":
		fs := flag.NewFlagSet("habits export", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "habits.export", fmt.Errorf("usage: drawgraph habits export <out.json>"), 2)
		}
		data, err := json.MarshalIndent(hb.Snapshot(), "", "  ")
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.export", err, 1)
		}
		if err := os.WriteFile(fs.Arg(0), append(data, '\n'), 0o644); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.export", err, 1)
		}
		return emitResult(stdout, *jsonOut, "habits.export", map[string]any{"output": fs.Arg(0)}, fmt.Sprintf("Exported habits to %s\n", fs.Arg(0)))
	case "import":
		fs := flag.NewFlagSet("habits import", flag.ContinueOnError)
		fs.SetOutput(stderr)
		jsonOut := fs.Bool("json", false, "output JSON")
		_ = fs.Parse(reorderFlags(rest))
		if fs.NArg() < 1 {
			return emitErr(stdout, stderr, *jsonOut, "habits.import", fmt.Errorf("usage: drawgraph habits import <in.json>"), 2)
		}
		raw, err := os.ReadFile(fs.Arg(0))
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.import", err, 1)
		}
		var imported habits.Profile
		if err := json.Unmarshal(raw, &imported); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.import", err, 2)
		}
		if err := hb.Update(func(p *habits.Profile) { *p = imported }); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "habits.import", err, 1)
		}
		return emitResult(stdout, *jsonOut, "habits.import", hb.Snapshot(), "Imported global habits\n")
	default:
		return emitErr(stdout, stderr, hasJSONFlag(rest), "habits", fmt.Errorf("unknown habits subcommand %q", sub), 2)
	}
}

func cmdLive(args []string, stdout, stderr io.Writer) int {
	sub := "status"
	rest := args
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		sub = args[0]
		rest = args[1:]
	}
	fs := flag.NewFlagSet("live", flag.ContinueOnError)
	fs.SetOutput(stderr)
	nodeID := fs.String("node", "", "target node ID for focus")
	strategy := fs.String("strategy", "", "layout strategy for live format")
	dir := fs.String("direction", "", "direction for live format")
	jsonOut := fs.Bool("json", false, "output JSON")
	_ = fs.Parse(reorderFlags(rest))

	ep, err := ipc.Discover("")
	if err != nil {
		return emitErr(stdout, stderr, *jsonOut, "live."+sub, err, 1)
	}
	switch sub {
	case "status":
		return emitResult(stdout, *jsonOut, "live.status", map[string]any{
			"connected": true, "pid": ep.PID, "addr": ep.Addr,
		}, fmt.Sprintf("Live GUI connected (pid=%d addr=%s)\n", ep.PID, ep.Addr))
	case "fit", "origin", "focus", "format":
		res, err := ipc.Control(ep, map[string]any{
			"action":    sub,
			"nodeId":    *nodeID,
			"strategy":  *strategy,
			"direction": *dir,
		})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "live."+sub, err, 1)
		}
		return emitResult(stdout, *jsonOut, "live."+sub, res, fmt.Sprintf("Live action %s applied\n", sub))
	default:
		return emitErr(stdout, stderr, *jsonOut, "live", fmt.Errorf("usage: drawgraph live <status|fit|origin|focus|format> [--json]"), 2)
	}
}

func cmdMCP(args []string, reg *icons.Registry, hb *habits.Store, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "config" {
		exe, _ := os.Executable()
		if exe == "" {
			exe = "drawgraph"
		}
		_, _ = io.WriteString(stdout, mcp.ConfigSnippet(exe))
		return 0
	}
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	httpAddr := fs.String("http", "", "optional HTTP listen addr (e.g. 127.0.0.1:7332) instead of stdio")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	srv, err := mcp.NewServer(reg, hb)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "drawgraph mcp: %v\n", err)
		return 1
	}
	if *httpAddr != "" {
		if err := srv.ServeHTTP(context.Background(), *httpAddr); err != nil {
			_, _ = fmt.Fprintf(stderr, "drawgraph mcp http: %v\n", err)
			return 1
		}
		return 0
	}
	if err := srv.ServeStdio(context.Background(), stdin, stdout); err != nil {
		_, _ = fmt.Fprintf(stderr, "drawgraph mcp stdio: %v\n", err)
		return 1
	}
	return 0
}

func cmdUpdate(args []string, hb *habits.Store, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		return emitErr(stdout, stderr, false, "update", fmt.Errorf("usage: drawgraph update <check|download|apply|config>"), 2)
	}
	sub := args[0]
	rest := args[1:]
	switch sub {
	case "check":
		fs := flag.NewFlagSet("update check", flag.ContinueOnError)
		fs.SetOutput(stderr)
		channel := fs.String("channel", hb.Snapshot().UpdateChannel, "stable | prerelease")
		current := fs.String("current", version.Version, "override current version for testing")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		rel, err := update.Check(ctx, update.CheckOptions{
			Current: *current,
			Channel: *channel,
		})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.check", err, 1)
		}
		if rel == nil {
			return emitResult(stdout, *jsonOut, "update.check", map[string]any{
				"upToDate": true,
				"current":  *current,
				"channel":  *channel,
			}, fmt.Sprintf("DrawGraph %s is up to date (%s)\n", *current, *channel))
		}
		return emitResult(stdout, *jsonOut, "update.check", map[string]any{
			"upToDate": false,
			"current":  *current,
			"release":  rel,
		}, fmt.Sprintf("Update available: v%s (%s)\n", rel.Version, rel.AssetName))
	case "download":
		fs := flag.NewFlagSet("update download", flag.ContinueOnError)
		fs.SetOutput(stderr)
		channel := fs.String("channel", hb.Snapshot().UpdateChannel, "stable | prerelease")
		dir := fs.String("dir", update.DefaultCacheDir(), "download directory")
		current := fs.String("current", version.Version, "override current version")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		rel, err := update.Check(ctx, update.CheckOptions{Current: *current, Channel: *channel})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.download", err, 1)
		}
		if rel == nil {
			return emitResult(stdout, *jsonOut, "update.download", map[string]any{"upToDate": true}, "Already up to date\n")
		}
		path, err := update.Download(ctx, rel, *dir, nil)
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.download", err, 1)
		}
		return emitResult(stdout, *jsonOut, "update.download", map[string]any{
			"version": rel.Version, "path": path,
		}, fmt.Sprintf("Downloaded & verified %s\n", path))
	case "apply":
		fs := flag.NewFlagSet("update apply", flag.ContinueOnError)
		fs.SetOutput(stderr)
		channel := fs.String("channel", hb.Snapshot().UpdateChannel, "stable | prerelease")
		silent := fs.Bool("silent", false, "run installer silently without progress UI")
		noRelaunch := fs.Bool("no-relaunch", false, "do not relaunch GUI after updating")
		current := fs.String("current", version.Version, "override current version")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		rel, err := update.Check(ctx, update.CheckOptions{Current: *current, Channel: *channel})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.apply", err, 1)
		}
		if rel == nil {
			return emitResult(stdout, *jsonOut, "update.apply", map[string]any{"upToDate": true}, "Already up to date\n")
		}
		path, err := update.Download(ctx, rel, update.DefaultCacheDir(), nil)
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.apply", err, 1)
		}
		if err := update.Apply(path, update.ApplyOptions{Silent: *silent, Relaunch: !*noRelaunch}); err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.apply", err, 1)
		}
		return emitResult(stdout, *jsonOut, "update.apply", map[string]any{
			"started": true, "version": rel.Version, "path": path,
		}, fmt.Sprintf("Applying update v%s from %s\n", rel.Version, path))
	case "config":
		fs := flag.NewFlagSet("update config", flag.ContinueOnError)
		fs.SetOutput(stderr)
		autoCheck := fs.String("auto-check", "", "true | false")
		autoDownload := fs.String("auto-download", "", "true | false")
		channel := fs.String("channel", "", "stable | prerelease")
		hours := fs.Int("interval-hours", 0, "check interval in hours")
		jsonOut := fs.Bool("json", false, "output JSON")
		if err := fs.Parse(reorderFlags(rest)); err != nil {
			return 2
		}
		err := hb.Update(func(p *habits.Profile) {
			if *autoCheck == "true" {
				p.AutoCheckUpdates = true
			} else if *autoCheck == "false" {
				p.AutoCheckUpdates = false
			}
			if *autoDownload == "true" {
				p.AutoDownloadUpdates = true
			} else if *autoDownload == "false" {
				p.AutoDownloadUpdates = false
			}
			if *channel == "stable" || *channel == "prerelease" {
				p.UpdateChannel = *channel
			}
			if *hours > 0 {
				p.UpdateCheckHours = *hours
			}
		})
		if err != nil {
			return emitErr(stdout, stderr, *jsonOut, "update.config", err, 1)
		}
		prof := hb.Snapshot()
		return emitResult(stdout, *jsonOut, "update.config", map[string]any{
			"autoCheckUpdates":    prof.AutoCheckUpdates,
			"autoDownloadUpdates": prof.AutoDownloadUpdates,
			"updateChannel":       prof.UpdateChannel,
			"updateCheckHours":    prof.UpdateCheckHours,
		}, fmt.Sprintf("Update config: autoCheck=%v autoDownload=%v channel=%s every=%dh\n",
			prof.AutoCheckUpdates, prof.AutoDownloadUpdates, prof.UpdateChannel, prof.UpdateCheckHours))
	default:
		return emitErr(stdout, stderr, hasJSONFlag(rest), "update", fmt.Errorf("unknown update subcommand %q", sub), 2)
	}
}

func loadAnyDoc(path string) (*graph.Document, error) {
	return ops.LoadAnyDoc(path)
}

func loadOrCreateDoc(path string, hb *habits.Store) (*graph.Document, error) {
	return ops.LoadOrCreateDoc(path, hb)
}

func saveDoc(path string, doc *graph.Document, live bool, reason string) error {
	return ops.SaveDoc(path, doc, live, reason)
}

func selectPage(doc *graph.Document, pageID string) (*graph.Page, error) {
	return ops.SelectPage(doc, pageID)
}

func setOptFloat(s string, dst *float64) {
	if strings.TrimSpace(s) == "" {
		return
	}
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		*dst = v
	}
}

func hasJSONFlag(args []string) bool {
	for _, a := range args {
		if a == "--json" || a == "-json" {
			return true
		}
	}
	return false
}

// reorderFlags allows flags before or after positional arguments (agent-friendly!).
func reorderFlags(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		if a == "--" {
			pos = append(pos, args[i:]...)
			break
		}
		if strings.HasPrefix(a, "-") && a != "-" {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && i+1 < len(args) && !isBoolFlagName(strings.TrimLeft(a, "-")) && !strings.HasPrefix(args[i+1], "-") {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		pos = append(pos, a)
	}
	return append(flags, pos...)
}

func isBoolFlagName(name string) bool {
	switch name {
	case "json", "live", "dark", "remember", "format", "stdin", "dry-run", "no-style", "all", "native", "show-bar", "loop", "muted", "silent", "no-relaunch":
		return true
	}
	return false
}

func emitResult(stdout io.Writer, asJSON bool, cmd string, res any, humanText string) int {
	if asJSON {
		_ = writeJSON(stdout, Envelope{OK: true, Command: cmd, Result: res})
		return 0
	}
	if humanText != "" {
		_, _ = io.WriteString(stdout, humanText)
	}
	return 0
}

func emitErr(stdout, stderr io.Writer, asJSON bool, cmd string, err error, code int) int {
	if asJSON {
		_ = writeJSON(stdout, Envelope{OK: false, Command: cmd, Error: err.Error()})
		return code
	}
	_, _ = fmt.Fprintf(stderr, "drawgraph %s: %v\n", cmd, err)
	return code
}

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printHelp(w io.Writer) {
	_, _ = io.WriteString(w, `drawgraph — Agent-native diagramming CLI & draw.io compatible workbench

Usage:
  drawgraph <command> [args] [--json]

Diagram Commands:
  new <file> [--template blank|arch|flow]   Create a new diagram (seeded by persistent habits)
  inspect <file> [--page id] [--live]       Inspect nodes, edges, bounds, and validation issues
  validate <file>                           Verify graph integrity and overlap warnings
  add-node <file> --label ... [flags]       Add a node (shapes, containers, icons)
  add-image <file> --src <img-path> [flags] Import HD image with aspect-preserving node size
  add-video <file> --src <vid-path> [flags] Import video node (MP4/MOV/WebM/MKV)
  update-node <file> --id ... [flags]       Update geometry, style, label, or icon
  remove-node <file> --id ...               Remove node and attached edges
  connect <file> --from A --to B [flags]    Connect two nodes
  disconnect <file> --id ...                Remove an edge
  page <list|add|switch|remove> <file>      Manage diagram pages
  batch <file> [--ops-file | --stdin]       Apply transactional JSON ops atomically
  dsl <file> [--dsl-file | --stdin]         Draw from concise architecture DSL + auto-layout
  format <file> [--strategy layered|tree|grid|radial|swimlane|compact] [--direction LR|RL|TB|BT]
  align <file> --ids a,b --mode <align>    Align or distribute nodes
  mermaid <file> [--stdin|--mermaid-file]   Import Mermaid flowchart into formatted diagram
  import <in.drawio> --to <out.drawgraph>   Import draw.io XML
  export <file> --to <out.png|svg|drawio>   Headless export via MyGo GPU renderer / SVG / draw.io

Icon Plugin Commands (Git / Local / SVG):
  icon list                                 List built-in and installed icon packs
  icon search <query>                       Search available icons across enabled packs
  icon install <git-url-or-dir> [flags]     Install icon pack from Git URL or local directory
  icon sync [pack-id | --all]               Pull updates for Git-installed icon packs
  icon import-svg <file.svg>                Import a single SVG into a custom pack
  icon remove <pack-id>                     Uninstall an external icon pack

Habit Memory Commands (Persisted Across Projects):
  habits show                               Show persistent cross-project preferences & learned habits
  habits set [flags]                        Persist language, preset, strategy, direction, routing, wheel
  habits presets                            List curated style presets
  habits export/import/reset                Manage portable habit profiles

Self-Update Commands (Manual & Automatic):
  update check [--channel stable|prerelease] Check GitHub Releases for newer MSI/DMG
  update download                            Download & SHA-256 verify latest installer
  update apply                               Download, verify & install latest release
  update config [flags]                      Configure auto-check, auto-download, channel

Agent Integration & Native MCP Server:
  mcp [--http addr | config]                Start MCP JSON-RPC 2.0 server or print client config
  schema                                    Print JSON capability schema for AI agents
  skill [--install <dir>]                   Print or install SKILL.md for AI agents
  live <status|fit|origin|focus|format>     Inspect or control running DrawGraph GUI
`)
}

const agentSkillMarkdown = `---
name: drawgraph-cli
description: Create, edit, auto-format, and export diagrams via the native DrawGraph Agent CLI.
---

# DrawGraph Agent CLI Workflow

1. Discover available node kinds, presets, and installed icon packs:
   ` + "`drawgraph schema --json`" + `
2. Search icons (built-in or user-installed Git packs):
   ` + "`drawgraph icon search gateway --json`" + `
3. Draw an entire architecture cleanly in one atomic command using DSL + auto-format:
   ` + "```sh" + `
   drawgraph dsl architecture.drawgraph --format --json <<'EOF'
   container vpc "Production VPC"
   node client "Web Client" icon:builtin/browser
   node gw "API Gateway" icon:builtin/gateway parent:vpc
   node api "Order API" icon:builtin/server parent:vpc
   node db "Orders DB" kind:cylinder icon:builtin/database parent:vpc
   client -> gw : "HTTPS"
   gw -> api : "gRPC"
   api -> db : "SQL"
   EOF
   ` + "```" + `
4. Inspect or validate any diagram:
   ` + "`drawgraph inspect architecture.drawgraph --json`" + `
   ` + "`drawgraph validate architecture.drawgraph --json`" + `
5. Export crisp PNG, SVG, or draw.io XML:
   ` + "`drawgraph export architecture.drawgraph --to architecture.svg`" + `
   ` + "`drawgraph export architecture.drawgraph --to architecture.drawio`" + `
`
