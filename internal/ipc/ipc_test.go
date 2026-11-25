package ipc_test

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/neko233-com/DrawGraph/internal/graph"
	"github.com/neko233-com/DrawGraph/internal/ipc"
)

type mockHandler struct {
	doc     *graph.Document
	reloads int
}

func (m *mockHandler) GetDocument() *graph.Document { return m.doc }
func (m *mockHandler) ReplaceDocument(d *graph.Document, _ string) error {
	m.doc = d
	return nil
}
func (m *mockHandler) ReloadIcons() error {
	m.reloads++
	return nil
}

func TestIPCAuthRoundTripAndCleanup(t *testing.T) {
	infoPath := filepath.Join(t.TempDir(), "run", "live.json")
	h := &mockHandler{doc: graph.NewDocument("Initial")}
	srv, err := ipc.Start(infoPath, h)
	if err != nil {
		t.Fatal(err)
	}

	ep, err := ipc.Discover(infoPath)
	if err != nil {
		t.Fatal(err)
	}

	// Unauthenticated request must be rejected.
	resp, err := http.Get("http://" + ep.Addr + "/v1/document")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 without bearer token, got %d", resp.StatusCode)
	}

	pulled, err := ipc.PullDocument(ep)
	if err != nil || pulled.Title != "Initial" {
		t.Fatalf("PullDocument failed: %v %+v", err, pulled)
	}

	updated := graph.NewDocument("Updated Live")
	if err := ipc.PushDocument(ep, updated, "test"); err != nil {
		t.Fatal(err)
	}
	if h.doc.Title != "Updated Live" {
		t.Fatalf("expected handler doc updated, got %q", h.doc.Title)
	}
	if err := ipc.ReloadIcons(ep); err != nil || h.reloads != 1 {
		t.Fatalf("ReloadIcons failed: err=%v reloads=%d", err, h.reloads)
	}

	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(infoPath); !os.IsNotExist(err) {
		t.Fatalf("expected endpoint discovery file removed on Close")
	}
}
