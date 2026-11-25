package mcp_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/ipc"
	"github.com/neko233-com/DrawGraph/internal/mcp"
)

func TestMCPStdioLifecycleToolsResourcesAndPNGVision(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DRAWGRAPH_HOME", filepath.Join(tmp, "home"))
	srv, err := mcp.NewServer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	diagPath := filepath.Join(tmp, "mcp-arch.drawgraph")
	pngPath := filepath.Join(tmp, "mcp-arch.png")

	requests := []map[string]any{
		{"jsonrpc": "2.0", "id": 1, "method": "initialize", "params": map[string]any{"protocolVersion": mcp.ProtocolVersion}},
		{"jsonrpc": "2.0", "method": "notifications/initialized"},
		{"jsonrpc": "2.0", "id": 2, "method": "tools/list"},
		{"jsonrpc": "2.0", "id": 3, "method": "tools/call", "params": map[string]any{
			"name": "diagram_apply_dsl",
			"arguments": map[string]any{
				"file":     diagPath,
				"strategy": "tree",
				"dsl":      "node edge_gw \"Edge Gateway\" icon:builtin/gateway\nnode api \"Orders API\" icon:builtin/server\nnode db \"Postgres\" kind:cylinder icon:builtin/database\nedge_gw -> api : \"gRPC\"\napi -> db : \"SQL\"\n",
			},
		}},
		{"jsonrpc": "2.0", "id": 4, "method": "tools/call", "params": map[string]any{
			"name": "diagram_export",
			"arguments": map[string]any{
				"file":        diagPath,
				"output":      pngPath,
				"format":      "png",
				"inlineImage": true,
			},
		}},
		{"jsonrpc": "2.0", "id": 5, "method": "tools/call", "params": map[string]any{
			"name": "habits_set",
			"arguments": map[string]any{
				"language": "zh-CN",
				"strategy": "radial",
			},
		}},
		{"jsonrpc": "2.0", "id": 6, "method": "resources/read", "params": map[string]any{
			"uri": "drawgraph://schema",
		}},
	}

	var in strings.Builder
	for _, r := range requests {
		b, _ := json.Marshal(r)
		in.Write(b)
		in.WriteByte('\n')
	}

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(in.String()), &out); err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 {
		t.Fatalf("expected 6 JSON-RPC responses (notification omitted), got %d:\n%s", len(lines), out.String())
	}

	if !strings.Contains(lines[1], "diagram_apply_dsl") || !strings.Contains(lines[1], "diagram_import_mermaid") {
		t.Fatalf("tools/list missing expected tools: %s", lines[1])
	}
	var resp3 struct {
		Result struct {
			Content []struct {
				Type     string `json:"type"`
				Text     string `json:"text"`
				MimeType string `json:"mimeType"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &resp3); err != nil || len(resp3.Result.Content) == 0 || !strings.Contains(resp3.Result.Content[0].Text, `"addedNodes": 3`) {
		t.Fatalf("diagram_apply_dsl failed: %s", lines[2])
	}
	if err := json.Unmarshal([]byte(lines[3]), &resp3); err != nil || len(resp3.Result.Content) < 2 || resp3.Result.Content[1].MimeType != "image/png" {
		t.Fatalf("expected inline PNG vision content block from diagram_export: %s", lines[3])
	}
	if err := json.Unmarshal([]byte(lines[4]), &resp3); err != nil || !strings.Contains(resp3.Result.Content[0].Text, `"language": "zh-CN"`) {
		t.Fatalf("habits_set did not persist language zh-CN: %s", lines[4])
	}
	if !strings.Contains(lines[5], "layoutStrategies") {
		t.Fatalf("resources/read schema missing layoutStrategies: %s", lines[5])
	}
}

func TestMCPLiveControlAndMermaidImport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("DRAWGRAPH_HOME", filepath.Join(tmp, "home"))
	srv, err := mcp.NewServer(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	ctrlCalled := ""
	ipcsrv, err := ipc.Start("", &mockIPC{onCtrl: func(a string) { ctrlCalled = a }})
	if err != nil {
		t.Fatal(err)
	}
	defer ipcsrv.Close()

	in := fmtReq(1, "tools/call", map[string]any{
		"name": "diagram_import_mermaid",
		"arguments": map[string]any{
			"file":    filepath.Join(tmp, "m.drawgraph"),
			"mermaid": "flowchart TB\n A[Start] --> B[Finish]",
			"live":    true,
		},
	}) + fmtReq(2, "tools/call", map[string]any{
		"name":      "live_control",
		"arguments": map[string]any{"action": "fit"},
	})

	var out bytes.Buffer
	if err := srv.ServeStdio(context.Background(), strings.NewReader(in), &out); err != nil {
		t.Fatal(err)
	}
	if ctrlCalled != "fit" || !strings.Contains(out.String(), `\"action\": \"fit\"`) {
		t.Fatalf("expected live_control fit to invoke GUI handler, ctrl=%q out=%s", ctrlCalled, out.String())
	}
}

func fmtReq(id int, method string, params any) string {
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params})
	return string(b) + "\n"
}

type mockIPC struct {
	doc    *graph.Document
	onCtrl func(string)
}

func (m *mockIPC) GetDocument() *graph.Document {
	if m.doc == nil {
		return graph.NewDocument("Mock")
	}
	return m.doc
}
func (m *mockIPC) ReplaceDocument(d *graph.Document, _ string) error { m.doc = d; return nil }
func (m *mockIPC) ReloadIcons() error                                { return nil }
func (m *mockIPC) HandleControl(action string, _ map[string]any) (map[string]any, error) {
	if m.onCtrl != nil {
		m.onCtrl(action)
	}
	return map[string]any{"ok": true, "action": action}, nil
}
