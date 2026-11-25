// Package mcp implements a pure-Go Model Context Protocol (MCP) server for DrawGraph
// over stdio JSON-RPC 2.0 and optional local HTTP, letting any AI assistant create,
// inspect, format, import, export (with inline PNG vision preview), manage icons, and control the live GUI.
package mcp

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	"github.com/neko233-com/DrawGraph/internal/media"
	"github.com/neko233-com/DrawGraph/internal/mermaid"
	"github.com/neko233-com/DrawGraph/internal/ops"
	"github.com/neko233-com/DrawGraph/internal/version"
)

const ProtocolVersion = "2024-11-05"

// Server exposes DrawGraph's full diagramming, icon plugin, habit, and live GUI capabilities over MCP.
type Server struct {
	Icons  *icons.Registry
	Habits *habits.Store
}

// NewServer initializes an MCP Server bound to the global or overridden stores.
func NewServer(reg *icons.Registry, hb *habits.Store) (*Server, error) {
	if hb == nil {
		var err error
		hb, err = habits.OpenStore("")
		if err != nil {
			return nil, err
		}
	}
	if reg == nil {
		var err error
		reg, err = icons.OpenRegistry("")
		if err != nil {
			return nil, err
		}
	}
	return &Server{Icons: reg, Habits: hb}, nil
}

// ConfigSnippet returns a ready-to-paste `mcpServers` JSON snippet for Cursor / Claude / Antigravity.
func ConfigSnippet(binary string) string {
	if strings.TrimSpace(binary) == "" {
		binary = "drawgraph"
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"drawgraph": map[string]any{
				"command": binary,
				"args":    []string{"mcp"},
			},
		},
	}
	b, _ := json.MarshalIndent(cfg, "", "  ")
	return string(b) + "\n"
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type toolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ServeStdio serves JSON-RPC 2.0 MCP traffic over r/w, supporting both newline-delimited JSON
// and Content-Length LSP-style headers transparently.
func (s *Server) ServeStdio(ctx context.Context, r io.Reader, w io.Writer) error {
	br := bufio.NewReader(r)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		payload, framed, err := readMCPMessage(br)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if len(payload) == 0 {
			continue
		}
		resp := s.HandleJSONRPC(ctx, payload)
		if resp == nil {
			continue
		}
		if err := writeMCPMessage(w, resp, framed); err != nil {
			return err
		}
	}
}

// ServeHTTP serves Streamable JSON-RPC MCP requests over HTTP at addr (`POST /mcp`).
func (s *Server) ServeHTTP(ctx context.Context, addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/mcp", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST required", http.StatusMethodNotAllowed)
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		resp := s.HandleJSONRPC(r.Context(), body)
		w.Header().Set("Content-Type", "application/json")
		if resp == nil {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		c2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(c2)
	}()
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func readMCPMessage(r *bufio.Reader) ([]byte, bool, error) {
	for {
		line, err := r.ReadString('\n')
		if err != nil && len(line) == 0 {
			return nil, false, err
		}
		trim := strings.TrimSpace(line)
		if trim == "" {
			if err != nil {
				return nil, false, err
			}
			continue
		}
		if strings.HasPrefix(strings.ToLower(trim), "content-length:") {
			val := strings.TrimSpace(trim[len("content-length:"):])
			n, perr := strconv.Atoi(val)
			if perr != nil || n <= 0 {
				return nil, true, fmt.Errorf("invalid Content-Length %q", val)
			}
			for {
				hline, herr := r.ReadString('\n')
				if strings.TrimSpace(hline) == "" || herr != nil {
					break
				}
			}
			buf := make([]byte, n)
			if _, rerr := io.ReadFull(r, buf); rerr != nil {
				return nil, true, rerr
			}
			return buf, true, nil
		}
		return []byte(trim), false, nil
	}
}

func writeMCPMessage(w io.Writer, v *rpcResponse, framed bool) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if framed {
		_, err = fmt.Fprintf(w, "Content-Length: %d\r\n\r\n%s", len(b), b)
		return err
	}
	_, err = fmt.Fprintf(w, "%s\n", b)
	return err
}

// HandleJSONRPC executes a single MCP JSON-RPC 2.0 request and returns nil for notifications.
func (s *Server) HandleJSONRPC(ctx context.Context, raw []byte) *rpcResponse {
	var req rpcRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return &rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "parse error: " + err.Error()}}
	}
	if len(req.ID) == 0 && strings.HasPrefix(req.Method, "notifications/") {
		return nil
	}
	okResp := func(res any) *rpcResponse {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Result: res}
	}
	errResp := func(code int, msg string) *rpcResponse {
		return &rpcResponse{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: msg}}
	}

	switch req.Method {
	case "initialize":
		return okResp(map[string]any{
			"protocolVersion": ProtocolVersion,
			"capabilities": map[string]any{
				"tools":     map[string]any{},
				"resources": map[string]any{},
				"prompts":   map[string]any{},
			},
			"serverInfo": map[string]any{
				"name":    "drawgraph-mcp",
				"version": version.Version,
			},
		})
	case "notifications/initialized":
		return nil
	case "ping":
		return okResp(map[string]any{})
	case "tools/list":
		return okResp(map[string]any{"tools": s.toolDefinitions()})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return errResp(-32602, "invalid tools/call params: "+err.Error())
		}
		res, isErr := s.callTool(ctx, params.Name, params.Arguments)
		if res == nil {
			res = map[string]any{"content": []map[string]any{{"type": "text", "text": "ok"}}}
		}
		if isErr {
			res["isError"] = true
		}
		return okResp(res)
	case "resources/list":
		return okResp(map[string]any{
			"resources": []map[string]any{
				{"uri": "drawgraph://schema", "name": "DrawGraph Schema & Extensions", "mimeType": "application/json"},
				{"uri": "drawgraph://habits", "name": "Persistent Cross-Project User Habits", "mimeType": "application/json"},
				{"uri": "drawgraph://live/document", "name": "Live Running GUI Document", "mimeType": "application/json"},
			},
		})
	case "resources/read":
		var params struct {
			URI string `json:"uri"`
		}
		_ = json.Unmarshal(req.Params, &params)
		text, err := s.readResource(params.URI)
		if err != nil {
			return errResp(-32002, err.Error())
		}
		return okResp(map[string]any{
			"contents": []map[string]any{
				{"uri": params.URI, "mimeType": "application/json", "text": text},
			},
		})
	case "prompts/list":
		return okResp(map[string]any{
			"prompts": []map[string]any{
				{
					"name":        "architecture_diagram",
					"description": "Design a clean DrawGraph cloud/microservice architecture diagram using DSL + One-Click Format.",
					"arguments":   []map[string]any{{"name": "topic", "required": true}},
				},
				{
					"name":        "flowchart_diagram",
					"description": "Design a top-down decision flowchart or workflow using DrawGraph.",
					"arguments":   []map[string]any{{"name": "process", "required": true}},
				},
			},
		})
	case "prompts/get":
		var params struct {
			Name      string            `json:"name"`
			Arguments map[string]string `json:"arguments"`
		}
		_ = json.Unmarshal(req.Params, &params)
		return okResp(getPromptResult(params.Name, params.Arguments))
	default:
		return errResp(-32601, "method not found: "+req.Method)
	}
}

func (s *Server) readResource(uri string) (string, error) {
	switch uri {
	case "drawgraph://schema":
		b, _ := json.MarshalIndent(ops.CapabilitiesSchema(s.Icons), "", "  ")
		return string(b), nil
	case "drawgraph://habits":
		b, _ := json.MarshalIndent(s.Habits.Snapshot(), "", "  ")
		return string(b), nil
	case "drawgraph://live/document":
		ep, err := ipc.Discover("")
		if err != nil {
			return "", fmt.Errorf("no running DrawGraph GUI: %w", err)
		}
		doc, err := ipc.PullDocument(ep)
		if err != nil {
			return "", err
		}
		b, _ := json.MarshalIndent(doc, "", "  ")
		return string(b), nil
	default:
		return "", fmt.Errorf("unknown resource %q", uri)
	}
}

func getPromptResult(name string, args map[string]string) map[string]any {
	topic := args["topic"]
	if topic == "" {
		topic = args["process"]
	}
	if topic == "" {
		topic = "distributed system"
	}
	text := fmt.Sprintf("Create a crisp, readable DrawGraph diagram for %q.\n1. Call icon_search if needed to discover available SVG icons.\n2. Use diagram_apply_dsl with containers/nodes/edges.\n3. Run diagram_format (layered/tree/radial/swimlane/grid/compact) and diagram_validate.\n4. Export PNG with diagram_export (inlineImage:true) to inspect the visual result.", topic)
	return map[string]any{
		"description": "DrawGraph diagramming guide",
		"messages": []map[string]any{
			{"role": "user", "content": map[string]any{"type": "text", "text": text}},
		},
	}
}

func (s *Server) toolDefinitions() []toolDef {
	return []toolDef{
		{
			Name:        "diagram_create",
			Description: "Create a new .drawgraph or .drawio file, optionally from a template (blank|arch|flow|microservices|org), seeded with persistent user habits.",
			InputSchema: objSchema(map[string]any{
				"file":     strProp("Output .drawgraph or .drawio path"),
				"title":    strProp("Diagram title"),
				"template": strProp("blank | arch | flow | microservices | org"),
				"live":     boolProp("Push to open DrawGraph GUI window"),
			}, "file"),
		},
		{
			Name:        "diagram_inspect",
			Description: "Inspect nodes, edges, bounding box, pages, and validation issues of a diagram file or the live GUI window.",
			InputSchema: objSchema(map[string]any{
				"file":   strProp("Path to .drawgraph or .drawio file"),
				"pageId": strProp("Optional page ID"),
				"live":   boolProp("Read directly from running DrawGraph GUI"),
			}),
		},
		{
			Name:        "diagram_validate",
			Description: "Validate node/edge referential integrity, container parents, and sibling overlaps.",
			InputSchema: objSchema(map[string]any{
				"file": strProp("Path to diagram file"),
			}, "file"),
		},
		{
			Name:        "diagram_apply_dsl",
			Description: "Apply concise DrawGraph DSL (container/swimlane/node/image/video/edges) and optionally auto-format with One-Click Format.",
			InputSchema: objSchema(map[string]any{
				"file":      strProp("Path to .drawgraph or .drawio (created if missing)"),
				"dsl":       strProp("DSL statements separated by newlines"),
				"pageId":    strProp("Optional page ID"),
				"format":    boolProp("Auto-format diagram after applying DSL (default true)"),
				"strategy":  strProp("layered | tree | grid | radial | swimlane | compact"),
				"direction": strProp("LR | RL | TB | BT"),
				"preset":    strProp("lobe-clean | drawio-classic | blueprint | slate-mono | vibrant-tech | warm-paper"),
				"live":      boolProp("Push to open DrawGraph GUI window"),
			}, "file", "dsl"),
		},
		{
			Name:        "diagram_import_mermaid",
			Description: "Import a Mermaid flowchart / graph (with subgraphs, shapes, and edge labels) into a formatted DrawGraph diagram.",
			InputSchema: objSchema(map[string]any{
				"file":      strProp("Output .drawgraph or .drawio path"),
				"mermaid":   strProp("Mermaid flowchart/graph source text"),
				"strategy":  strProp("Optional layout strategy override"),
				"direction": strProp("Optional direction override: LR|RL|TB|BT"),
				"live":      boolProp("Push to open DrawGraph GUI window"),
			}, "file", "mermaid"),
		},
		{
			Name:        "diagram_import_drawio",
			Description: "Import a .drawio XML diagram into native .drawgraph format.",
			InputSchema: objSchema(map[string]any{
				"source": strProp("Input .drawio or .xml path"),
				"output": strProp("Output .drawgraph path"),
			}, "source"),
		},
		{
			Name:        "diagram_batch",
			Description: "Execute transactional JSON operations (add_node, update_node, remove_node, connect, disconnect, align, zorder, group, ungroup, add_page, format).",
			InputSchema: objSchema(map[string]any{
				"file":   strProp("Path to diagram file"),
				"ops":    map[string]any{"type": "array", "description": "Array of BatchOp objects", "items": map[string]any{"type": "object"}},
				"pageId": strProp("Optional page ID"),
				"format": boolProp("Run one-click format afterwards"),
				"dryRun": boolProp("Validate without saving to disk"),
				"live":   boolProp("Push to running GUI window"),
			}, "file", "ops"),
		},
		{
			Name:        "diagram_format",
			Description: "Run One-Click Elegant Formatting (layered DAG, tree, grid, radial, swimlane, or overlap compacting + routing + palette harmony).",
			InputSchema: objSchema(map[string]any{
				"file":      strProp("Path to diagram file"),
				"pageId":    strProp("Optional page ID"),
				"strategy":  strProp("layered | tree | grid | radial | swimlane | compact"),
				"direction": strProp("LR | RL | TB | BT"),
				"routing":   strProp("orthogonal | curved | straight"),
				"preset":    strProp("Style preset ID"),
				"dark":      boolProp("Harmonize for dark theme"),
				"remember":  boolProp("Save direction/strategy/preset to persistent habits"),
				"live":      boolProp("Push to running GUI window"),
			}, "file"),
		},
		{
			Name:        "diagram_align",
			Description: "Align or distribute multiple nodes on a page.",
			InputSchema: objSchema(map[string]any{
				"file":   strProp("Path to diagram file"),
				"ids":    map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "Node IDs to align"},
				"mode":   strProp("left | center-x | right | top | center-y | bottom | distribute-h | distribute-v"),
				"pageId": strProp("Optional page ID"),
				"live":   boolProp("Push to running GUI window"),
			}, "file", "ids", "mode"),
		},
		{
			Name:        "diagram_add_media",
			Description: "Import a high-resolution image or video file as an aspect-preserved (or 1:1 native) canvas node.",
			InputSchema: objSchema(map[string]any{
				"file":    strProp("Path to diagram file"),
				"src":     strProp("Local image or video file path"),
				"kind":    strProp("image | video (auto-detected if omitted)"),
				"id":      strProp("Optional node ID"),
				"label":   strProp("Optional caption"),
				"maxSide": numProp("Max display size in DIPs"),
				"native":  boolProp("Use 1:1 native source resolution"),
				"x":       numProp("X coordinate"),
				"y":       numProp("Y coordinate"),
				"live":    boolProp("Push to running GUI window"),
			}, "file", "src"),
		},
		{
			Name:        "diagram_export",
			Description: "Export a diagram page to PNG (GPU rendered), SVG, or .drawio XML. When format is png or inlineImage is true, returns the rendered PNG image directly for visual inspection.",
			InputSchema: objSchema(map[string]any{
				"file":        strProp("Input .drawgraph or .drawio path"),
				"output":      strProp("Output file path (.png | .svg | .drawio)"),
				"format":      strProp("png | svg | drawio"),
				"pageId":      strProp("Optional page ID"),
				"dark":        boolProp("Render dark theme"),
				"scale":       numProp("PNG scale multiplier (default 2)"),
				"inlineImage": boolProp("Include base64 PNG image block in MCP tool response"),
			}, "file"),
		},
		{
			Name:        "icon_list",
			Description: "List built-in and Git/local external SVG icon packs.",
			InputSchema: objSchema(map[string]any{}),
		},
		{
			Name:        "icon_search",
			Description: "Search built-in and installed external SVG icon packs by keyword, pack ID, or category.",
			InputSchema: objSchema(map[string]any{
				"query":    strProp("Keyword query"),
				"pack":     strProp("Optional pack ID filter"),
				"category": strProp("Optional category filter"),
			}),
		},
		{
			Name:        "icon_install",
			Description: "Install an extensible SVG icon pack from a Git repository URL or local folder.",
			InputSchema: objSchema(map[string]any{
				"source": strProp("Git URL or local SVG directory path"),
				"id":     strProp("Optional pack ID override"),
				"name":   strProp("Optional display name"),
				"ref":    strProp("Optional git branch or tag"),
				"subdir": strProp("Optional subdirectory containing SVGs"),
			}, "source"),
		},
		{
			Name:        "icon_sync",
			Description: "Pull updates for installed Git icon pack(s).",
			InputSchema: objSchema(map[string]any{
				"packId": strProp("Specific pack ID (omit to sync all)"),
			}),
		},
		{
			Name:        "habits_get",
			Description: "Read global cross-project persisted user habits, language preference, layout defaults, and presets.",
			InputSchema: objSchema(map[string]any{}),
		},
		{
			Name:        "habits_set",
			Description: "Persist cross-project defaults (language, preset, direction, layout strategy, routing, mouse wheel mode, spacing).",
			InputSchema: objSchema(map[string]any{
				"language":     strProp("auto | en | zh-CN | zh-TW | ja | ko"),
				"preset":       strProp("Style preset ID"),
				"direction":    strProp("LR | RL | TB | BT"),
				"strategy":     strProp("layered | tree | grid | radial | swimlane | compact"),
				"routing":      strProp("orthogonal | curved | straight"),
				"wheelMode":    strProp("pan | zoom"),
				"themeMode":    strProp("auto | light | dark"),
				"gridSize":     numProp("Grid snap size"),
				"gapX":         numProp("Layout horizontal gap"),
				"gapY":         numProp("Layout vertical gap"),
				"autoRemember": boolProp("Automatically remember user style changes"),
			}),
		},
		{
			Name:        "live_control",
			Description: "Inspect or control the running DrawGraph GUI window (status, fit canvas, reset origin, focus node, live one-click format).",
			InputSchema: objSchema(map[string]any{
				"action":    strProp("status | fit | origin | focus | format"),
				"nodeId":    strProp("Target node ID for focus action"),
				"strategy":  strProp("Layout strategy for format action"),
				"direction": strProp("Layout direction for format action"),
			}, "action"),
		},
	}
}

func (s *Server) callTool(ctx context.Context, name string, args map[string]any) (map[string]any, bool) {
	jsonText := func(v any) (map[string]any, bool) {
		b, _ := json.MarshalIndent(v, "", "  ")
		return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}, false
	}
	toolErr := func(err error) (map[string]any, bool) {
		return map[string]any{"content": []map[string]any{{"type": "text", "text": err.Error()}}}, true
	}

	switch name {
	case "diagram_create":
		file := strArg(args, "file")
		if file == "" {
			return toolErr(fmt.Errorf("file is required"))
		}
		title := strArg(args, "title")
		if title == "" {
			title = strings.TrimSuffix(filepath.Base(file), filepath.Ext(file))
		}
		doc := graph.NewDocument(title)
		s.Habits.ApplyToDocument(doc)
		if err := ops.ApplyTemplate(doc, doc.ActivePage(), strArg(args, "template"), s.Habits); err != nil {
			return toolErr(err)
		}
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.create"); err != nil {
			return toolErr(err)
		}
		s.Habits.RecordRecentFile(file)
		return jsonText(map[string]any{
			"file": file, "title": doc.Title,
			"nodes": len(doc.ActivePage().Nodes), "edges": len(doc.ActivePage().Edges),
		})

	case "diagram_inspect":
		var doc *graph.Document
		var err error
		file := strArg(args, "file")
		if boolArg(args, "live", false) && file == "" {
			ep, derr := ipc.Discover("")
			if derr != nil {
				return toolErr(derr)
			}
			doc, err = ipc.PullDocument(ep)
		} else {
			if file == "" {
				return toolErr(fmt.Errorf("file is required unless live=true"))
			}
			doc, err = ops.LoadAnyDoc(file)
		}
		if err != nil {
			return toolErr(err)
		}
		p, err := ops.SelectPage(doc, strArg(args, "pageId"))
		if err != nil {
			return toolErr(err)
		}
		bounds, _ := p.Bounds()
		return jsonText(map[string]any{
			"title": doc.Title, "activePage": p.ID, "pages": len(doc.Pages),
			"bounds": bounds, "nodes": p.Nodes, "edges": p.Edges,
			"assets": doc.Assets, "issues": doc.Validate(),
		})

	case "diagram_validate":
		doc, err := ops.LoadAnyDoc(strArg(args, "file"))
		if err != nil {
			return toolErr(err)
		}
		issues := doc.Validate()
		return jsonText(map[string]any{"valid": len(issues) == 0, "issues": issues})

	case "diagram_apply_dsl":
		file := strArg(args, "file")
		dsl := strArg(args, "dsl")
		if file == "" || strings.TrimSpace(dsl) == "" {
			return toolErr(fmt.Errorf("file and dsl are required"))
		}
		doc, err := ops.LoadOrCreateDoc(file, s.Habits)
		if err != nil {
			return toolErr(err)
		}
		p, err := ops.SelectPage(doc, strArg(args, "pageId"))
		if err != nil {
			return toolErr(err)
		}
		an, ae, err := ops.ApplyDSL(doc, p, dsl, s.Habits)
		if err != nil {
			return toolErr(err)
		}
		if boolArg(args, "format", true) {
			fopts := layout.OptionsFromHabits(s.Habits.Snapshot(), false)
			if st := strArg(args, "strategy"); st != "" {
				fopts.Strategy = st
			}
			if dr := strArg(args, "direction"); dr != "" {
				fopts.Direction = dr
			}
			if pr := strArg(args, "preset"); pr != "" {
				fopts.Preset = pr
			}
			layout.FormatPage(doc, p, fopts)
		}
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.dsl"); err != nil {
			return toolErr(err)
		}
		return jsonText(map[string]any{
			"file": file, "addedNodes": an, "addedEdges": ae,
			"totalNodes": len(p.Nodes), "totalEdges": len(p.Edges),
		})

	case "diagram_import_mermaid":
		file := strArg(args, "file")
		src := strArg(args, "mermaid")
		if file == "" || strings.TrimSpace(src) == "" {
			return toolErr(fmt.Errorf("file and mermaid source are required"))
		}
		doc, err := mermaid.Parse(src, s.Habits)
		if err != nil {
			return toolErr(err)
		}
		p := doc.ActivePage()
		if strArg(args, "strategy") != "" || strArg(args, "direction") != "" {
			fopts := layout.OptionsFromHabits(s.Habits.Snapshot(), false)
			if st := strArg(args, "strategy"); st != "" {
				fopts.Strategy = st
			}
			if dr := strArg(args, "direction"); dr != "" {
				fopts.Direction = dr
			}
			layout.FormatPage(doc, p, fopts)
		}
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.mermaid"); err != nil {
			return toolErr(err)
		}
		return jsonText(map[string]any{
			"file": file, "nodes": len(p.Nodes), "edges": len(p.Edges),
		})

	case "diagram_import_drawio":
		src := strArg(args, "source")
		out := strArg(args, "output")
		if out == "" {
			out = strings.TrimSuffix(src, filepath.Ext(src)) + ".drawgraph"
		}
		doc, err := drawio.ImportFile(src)
		if err != nil {
			return toolErr(err)
		}
		if err := graph.SaveAtomic(out, doc); err != nil {
			return toolErr(err)
		}
		return jsonText(map[string]any{"source": src, "output": out, "pages": len(doc.Pages)})

	case "diagram_batch":
		file := strArg(args, "file")
		rawOps, _ := json.Marshal(args["ops"])
		var opsList []ops.BatchOp
		if err := json.Unmarshal(rawOps, &opsList); err != nil {
			return toolErr(fmt.Errorf("invalid ops array: %w", err))
		}
		doc, err := ops.LoadOrCreateDoc(file, s.Habits)
		if err != nil {
			return toolErr(err)
		}
		p, err := ops.SelectPage(doc, strArg(args, "pageId"))
		if err != nil {
			return toolErr(err)
		}
		if err := ops.ExecuteBatchOps(doc, p, opsList, s.Habits); err != nil {
			return toolErr(err)
		}
		if boolArg(args, "format", false) {
			layout.FormatPage(doc, p, layout.OptionsFromHabits(s.Habits.Snapshot(), false))
		}
		dry := boolArg(args, "dryRun", false)
		if !dry {
			if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.batch"); err != nil {
				return toolErr(err)
			}
		}
		return jsonText(map[string]any{
			"file": file, "ops": len(opsList), "dryRun": dry,
			"nodes": len(p.Nodes), "edges": len(p.Edges),
		})

	case "diagram_format":
		file := strArg(args, "file")
		doc, err := ops.LoadAnyDoc(file)
		if err != nil {
			return toolErr(err)
		}
		p, err := ops.SelectPage(doc, strArg(args, "pageId"))
		if err != nil {
			return toolErr(err)
		}
		fopts := layout.OptionsFromHabits(s.Habits.Snapshot(), boolArg(args, "dark", false))
		if st := strArg(args, "strategy"); st != "" {
			fopts.Strategy = st
		}
		if dr := strArg(args, "direction"); dr != "" {
			fopts.Direction = dr
		}
		if rt := strArg(args, "routing"); rt != "" {
			fopts.Routing = graph.RoutingStyle(rt)
		}
		if pr := strArg(args, "preset"); pr != "" {
			fopts.Preset = pr
		}
		rep := layout.FormatPage(doc, p, fopts)
		if boolArg(args, "remember", true) {
			s.Habits.RecordFormatHabit(fopts.Direction, fopts.Routing, fopts.Preset, fopts.GapX, fopts.GapY, fopts.GridSize)
			_ = s.Habits.Update(func(pr *habits.Profile) { pr.DefaultStrategy = fopts.Strategy })
		}
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.format"); err != nil {
			return toolErr(err)
		}
		return jsonText(rep)

	case "diagram_align":
		file := strArg(args, "file")
		doc, err := ops.LoadAnyDoc(file)
		if err != nil {
			return toolErr(err)
		}
		p, err := ops.SelectPage(doc, strArg(args, "pageId"))
		if err != nil {
			return toolErr(err)
		}
		ids := strSliceArg(args, "ids")
		changed := graph.AlignNodes(p, ids, graph.AlignMode(strArg(args, "mode")))
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.align"); err != nil {
			return toolErr(err)
		}
		return jsonText(map[string]any{"changed": changed, "ids": ids})

	case "diagram_add_media":
		file := strArg(args, "file")
		src := strArg(args, "src")
		doc, err := ops.LoadOrCreateDoc(file, s.Habits)
		if err != nil {
			return toolErr(err)
		}
		p := doc.ActivePage()
		kind := strArg(args, "kind")
		if kind == "" {
			kind = media.ClassifyPath(src)
		}
		maxSide := numArg(args, "maxSide", s.Habits.Snapshot().DefaultMediaMax)
		var asset *graph.MediaAsset
		var dw, dh float64
		if kind == media.KindVideo {
			asset, dw, dh, err = media.ImportVideoFile(doc, src, maxSide)
		} else {
			asset, dw, dh, err = media.ImportImageFile(doc, src, maxSide)
		}
		if err != nil {
			return toolErr(err)
		}
		if boolArg(args, "native", false) && asset.NaturalW > 0 {
			dw, dh = float64(asset.NaturalW), float64(asset.NaturalH)
		}
		lbl := strArg(args, "label")
		if lbl == "" {
			lbl = asset.Name
		}
		nk := graph.KindImage
		if kind == media.KindVideo {
			nk = graph.KindVideo
		}
		n := &graph.Node{
			ID: strArg(args, "id"), Kind: nk,
			X: numArg(args, "x", 80), Y: numArg(args, "y", 80), W: dw, H: dh,
			Label: lbl,
			Media: &graph.NodeMedia{AssetID: asset.ID, Fit: "contain", Loop: true, Muted: true, Autoplay: kind == media.KindVideo},
		}
		if err := p.AddNode(n); err != nil {
			return toolErr(err)
		}
		if err := ops.SaveDoc(file, doc, boolArg(args, "live", false), "mcp.media"); err != nil {
			return toolErr(err)
		}
		return jsonText(map[string]any{"node": n, "asset": asset})

	case "diagram_export":
		file := strArg(args, "file")
		doc, err := ops.LoadAnyDoc(file)
		if err != nil {
			return toolErr(err)
		}
		out := strArg(args, "output")
		format := strings.ToLower(strArg(args, "format"))
		if format == "" && out != "" {
			format = strings.TrimPrefix(strings.ToLower(filepath.Ext(out)), ".")
		}
		if format == "" {
			format = "png"
		}
		pageID := strArg(args, "pageId")
		dark := boolArg(args, "dark", false)
		scale := numArg(args, "scale", 2)

		var pngBytes []byte
		switch format {
		case "png":
			pngBytes, err = export.RenderPNG(doc, pageID, s.Icons, dark, scale)
			if err != nil {
				return toolErr(err)
			}
			if out != "" {
				if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
					return toolErr(err)
				}
				if err := os.WriteFile(out, pngBytes, 0o644); err != nil {
					return toolErr(err)
				}
			}
		case "drawio", "xml":
			if out == "" {
				out = strings.TrimSuffix(file, filepath.Ext(file)) + ".drawio"
			}
			if err := drawio.ExportFile(out, doc); err != nil {
				return toolErr(err)
			}
		default:
			if out == "" {
				out = strings.TrimSuffix(file, filepath.Ext(file)) + "." + format
			}
			if err := export.ExportFile(out, doc, pageID, s.Icons, dark, scale); err != nil {
				return toolErr(err)
			}
		}

		content := []map[string]any{{
			"type": "text",
			"text": fmt.Sprintf(`{"input":%q,"output":%q,"format":%q}`, file, out, format),
		}}
		if format == "png" && (boolArg(args, "inlineImage", true)) && len(pngBytes) > 0 {
			content = append(content, map[string]any{
				"type":     "image",
				"data":     base64.StdEncoding.EncodeToString(pngBytes),
				"mimeType": "image/png",
			})
		}
		return map[string]any{"content": content}, false

	case "icon_list":
		return jsonText(s.Icons.Packs())

	case "icon_search":
		hits := s.Icons.Search(strArg(args, "query"), strArg(args, "pack"), strArg(args, "category"))
		return jsonText(hits)

	case "icon_install":
		src := strArg(args, "source")
		if src == "" {
			return toolErr(fmt.Errorf("source is required"))
		}
		var pk *icons.IconPack
		var err error
		if st, serr := os.Stat(src); serr == nil && st.IsDir() {
			pk, err = s.Icons.InstallLocal(src, strArg(args, "id"), strArg(args, "name"), strArg(args, "subdir"))
		} else {
			cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
			defer cancel()
			pk, err = s.Icons.InstallGit(cctx, icons.GitInstallOptions{
				URL: src, ID: strArg(args, "id"), Name: strArg(args, "name"), Ref: strArg(args, "ref"), Subdir: strArg(args, "subdir"),
			})
		}
		if err != nil {
			return toolErr(err)
		}
		if ep, derr := ipc.Discover(""); derr == nil {
			_ = ipc.ReloadIcons(ep)
		}
		return jsonText(pk)

	case "icon_sync":
		cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
		defer cancel()
		packID := strArg(args, "packId")
		var synced []*icons.IconPack
		if packID == "" {
			for _, p := range s.Icons.Packs() {
				if p.Builtin {
					continue
				}
				pk, err := s.Icons.SyncPack(cctx, p.ID)
				if err != nil {
					return toolErr(err)
				}
				synced = append(synced, pk)
			}
		} else {
			pk, err := s.Icons.SyncPack(cctx, packID)
			if err != nil {
				return toolErr(err)
			}
			synced = append(synced, pk)
		}
		return jsonText(synced)

	case "habits_get":
		return jsonText(s.Habits.Snapshot())

	case "habits_set":
		err := s.Habits.Update(func(p *habits.Profile) {
			if v := strArg(args, "language"); v != "" {
				p.Language = i18n.NormalizeTag(v)
			}
			if v := strArg(args, "preset"); v != "" {
				p.StylePreset = v
			}
			if v := strings.ToUpper(strArg(args, "direction")); v == "LR" || v == "RL" || v == "TB" || v == "BT" {
				p.PreferredDirection = v
			}
			if v := strArg(args, "strategy"); v != "" {
				p.DefaultStrategy = v
			}
			if v := strArg(args, "routing"); v != "" {
				p.DefaultRouting = graph.RoutingStyle(v)
			}
			if v := strArg(args, "wheelMode"); v == "pan" || v == "zoom" {
				p.WheelMode = v
			}
			if v := strArg(args, "themeMode"); v == "auto" || v == "light" || v == "dark" {
				p.ThemeMode = v
			}
			if g := numArg(args, "gridSize", 0); g > 0 {
				p.GridSize = g
			}
			if gx := numArg(args, "gapX", 0); gx > 0 {
				p.GapX = gx
			}
			if gy := numArg(args, "gapY", 0); gy > 0 {
				p.GapY = gy
			}
			if _, ok := args["autoRemember"]; ok {
				p.AutoRemember = boolArg(args, "autoRemember", true)
			}
		})
		if err != nil {
			return toolErr(err)
		}
		return jsonText(s.Habits.Snapshot())

	case "live_control":
		ep, err := ipc.Discover("")
		if err != nil {
			return toolErr(err)
		}
		action := strArg(args, "action")
		if action == "" || action == "status" {
			return jsonText(map[string]any{"connected": true, "pid": ep.PID, "addr": ep.Addr})
		}
		res, err := ipc.Control(ep, args)
		if err != nil {
			return toolErr(err)
		}
		return jsonText(res)

	default:
		return toolErr(fmt.Errorf("unknown tool %q", name))
	}
}

func objSchema(props map[string]any, req ...string) map[string]any {
	m := map[string]any{"type": "object", "properties": props}
	if len(req) > 0 {
		m["required"] = req
	}
	return m
}
func strProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}
func boolProp(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}
func numProp(desc string) map[string]any {
	return map[string]any{"type": "number", "description": desc}
}

func strArg(m map[string]any, k string) string {
	v, _ := m[k].(string)
	return v
}
func boolArg(m map[string]any, k string, def bool) bool {
	if m == nil {
		return def
	}
	v, ok := m[k]
	if !ok {
		return def
	}
	b, ok := v.(bool)
	if !ok {
		return def
	}
	return b
}
func numArg(m map[string]any, k string, def float64) float64 {
	if m == nil {
		return def
	}
	v, ok := m[k].(float64)
	if !ok {
		return def
	}
	return v
}
func strSliceArg(m map[string]any, k string) []string {
	raw, _ := m[k].([]any)
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if s, ok := it.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
