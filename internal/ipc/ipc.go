// Package ipc provides a local authenticated loopback endpoint so the Agent CLI
// can inspect or push live diagram updates into a running DrawGraph GUI instance.
package ipc

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neko233-com/DrawGraph/internal/graph"
)

// EndpointFile records the running GUI server address and bearer token.
type EndpointFile struct {
	PID       int       `json:"pid"`
	Addr      string    `json:"addr"`
	Token     string    `json:"token"`
	StartedAt time.Time `json:"startedAt"`
}

// EndpointPath returns the path to the live IPC discovery file.
func EndpointPath() string {
	if home := strings.TrimSpace(os.Getenv("DRAWGRAPH_HOME")); home != "" {
		return filepath.Join(home, "run", "live.json")
	}
	base, err := os.UserConfigDir()
	if err != nil || base == "" {
		base, _ = os.UserHomeDir()
		return filepath.Join(base, ".drawgraph", "run", "live.json")
	}
	return filepath.Join(base, "DrawGraph", "run", "live.json")
}

// Handler receives live requests from CLI / agents on behalf of the GUI.
type Handler interface {
	GetDocument() *graph.Document
	ReplaceDocument(doc *graph.Document, reason string) error
	ReloadIcons() error
}

// ControlHandler is an optional extension implemented by GUIs supporting remote viewport & layout driving.
type ControlHandler interface {
	HandleControl(action string, params map[string]any) (map[string]any, error)
}

// Server exposes authenticated local HTTP JSON endpoints for live agent driving.
type Server struct {
	ln       net.Listener
	srv      *http.Server
	token    string
	infoPath string
}

// Start launches the local IPC listener and writes the discovery endpoint file.
func Start(infoPath string, h Handler) (*Server, error) {
	if strings.TrimSpace(infoPath) == "" {
		infoPath = EndpointPath()
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	tok := hex.EncodeToString(buf)

	mux := http.NewServeMux()
	auth := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer "+tok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return false
		}
		return true
	}
	mux.HandleFunc("/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "pid": os.Getpid()})
	})
	mux.HandleFunc("/v1/document", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		switch r.Method {
		case http.MethodGet:
			doc := h.GetDocument()
			_ = json.NewEncoder(w).Encode(doc)
		case http.MethodPut, http.MethodPost:
			var payload struct {
				Document *graph.Document `json:"document"`
				Reason   string          `json:"reason"`
			}
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if payload.Document == nil {
				http.Error(w, "missing document", http.StatusBadRequest)
				return
			}
			payload.Document.Normalize()
			if err := h.ReplaceDocument(payload.Document, payload.Reason); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/icons/reload", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		if err := h.ReloadIcons(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	})
	mux.HandleFunc("/v1/control", func(w http.ResponseWriter, r *http.Request) {
		if !auth(w, r) {
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ch, ok := h.(ControlHandler)
		if !ok {
			http.Error(w, "live GUI does not implement viewport control", http.StatusNotImplemented)
			return
		}
		action, _ := payload["action"].(string)
		res, err := ch.HandleControl(action, payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if res == nil {
			res = map[string]any{"ok": true}
		}
		_ = json.NewEncoder(w).Encode(res)
	})

	s := &Server{
		ln:       ln,
		token:    tok,
		infoPath: infoPath,
		srv:      &http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second},
	}
	s.srv.SetKeepAlivesEnabled(false)
	if err := os.MkdirAll(filepath.Dir(infoPath), 0o755); err != nil {
		_ = ln.Close()
		return nil, err
	}
	ep := EndpointFile{
		PID:       os.Getpid(),
		Addr:      ln.Addr().String(),
		Token:     tok,
		StartedAt: time.Now().UTC(),
	}
	data, _ := json.MarshalIndent(ep, "", "  ")
	if err := os.WriteFile(infoPath, data, 0o600); err != nil {
		_ = ln.Close()
		return nil, err
	}
	go func() { _ = s.srv.Serve(ln) }()
	return s, nil
}

// Addr returns the listening host:port.
func (s *Server) Addr() string { return s.ln.Addr().String() }

// Close shuts down the server and removes the endpoint discovery file.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	_ = os.Remove(s.infoPath)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		return s.srv.Close()
	}
	return nil
}

// Discover reads the live GUI endpoint file and pings it.
func Discover(infoPath string) (*EndpointFile, error) {
	if strings.TrimSpace(infoPath) == "" {
		infoPath = EndpointPath()
	}
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return nil, err
	}
	var ep EndpointFile
	if err := json.Unmarshal(data, &ep); err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodGet, "http://"+ep.Addr+"/v1/ping", nil)
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	cl := &http.Client{Timeout: 2 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, fmt.Errorf("live gui unreachable at %s: %w", ep.Addr, err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("live gui ping returned %s", resp.Status)
	}
	return &ep, nil
}

// PushDocument pushes an updated document to the running GUI instance.
func PushDocument(ep *EndpointFile, doc *graph.Document, reason string) error {
	body, err := json.Marshal(map[string]any{
		"document": doc,
		"reason":   reason,
	})
	if err != nil {
		return err
	}
	req, _ := http.NewRequest(http.MethodPut, "http://"+ep.Addr+"/v1/document", bytes.NewReader(body))
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("live update failed: %s", resp.Status)
	}
	return nil
}

// PullDocument fetches the active document currently open in the live GUI.
func PullDocument(ep *EndpointFile) (*graph.Document, error) {
	req, _ := http.NewRequest(http.MethodGet, "http://"+ep.Addr+"/v1/document", nil)
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("live get failed: %s", resp.Status)
	}
	var doc graph.Document
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return nil, err
	}
	doc.Normalize()
	return &doc, nil
}

// ReloadIcons tells the running GUI to reload icon plugins from the global registry.
func ReloadIcons(ep *EndpointFile) error {
	req, _ := http.NewRequest(http.MethodPost, "http://"+ep.Addr+"/v1/icons/reload", nil)
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	cl := &http.Client{Timeout: 3 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	return nil
}

// Control invokes a live GUI camera/layout action ("fit", "origin", "focus", "format", "zoom").
func Control(ep *EndpointFile, payload map[string]any) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, _ := http.NewRequest(http.MethodPost, "http://"+ep.Addr+"/v1/control", bytes.NewReader(body))
	req.Close = true
	req.Header.Set("Authorization", "Bearer "+ep.Token)
	req.Header.Set("Content-Type", "application/json")
	cl := &http.Client{Timeout: 5 * time.Second}
	resp, err := cl.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("live control failed (%s): %s", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out, nil
}
