package master

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kcodes0/decent/internal/protocol"
)

func TestServerRegisterHeartbeatAndStatus(t *testing.T) {
	manifest := &protocol.Manifest{
		ContentHash: "hash-1",
		Master: protocol.MasterNode{
			ID:          "master",
			Region:      "us-west",
			SiteBaseURL: "http://master.example",
		},
	}
	server := httptest.NewServer(NewServer(Config{Manifest: manifest}))
	defer server.Close()

	registerBody, _ := json.Marshal(protocol.RegisterRequest{
		Node: protocol.RegisteredNode{
			ID:          "worker-1",
			Region:      "us-west",
			PublicURL:   "http://worker-1.example",
			Status:      "healthy",
			ContentHash: "hash-1",
		},
	})
	resp, err := http.Post(server.URL+"/api/register", "application/json", bytes.NewReader(registerBody))
	if err != nil {
		t.Fatalf("register request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from register, got %d", resp.StatusCode)
	}

	heartbeatBody, _ := json.Marshal(protocol.HeartbeatRequest{
		NodeID:        "worker-1",
		Region:        "us-west",
		PublicURL:     "http://worker-1.example",
		ContentHash:   "hash-1",
		Healthy:       true,
		UptimeSeconds: 12,
	})
	resp, err = http.Post(server.URL+"/api/heartbeat", "application/json", bytes.NewReader(heartbeatBody))
	if err != nil {
		t.Fatalf("heartbeat request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from heartbeat, got %d", resp.StatusCode)
	}

	resp, err = http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer resp.Body.Close()

	var status protocol.StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if len(status.HealthyNodes) != 1 || status.HealthyNodes[0].ID != "worker-1" {
		t.Fatalf("unexpected healthy nodes: %#v", status.HealthyNodes)
	}
	if status.LocalNode.ID != "master" {
		t.Fatalf("unexpected local node: %#v", status.LocalNode)
	}
}

func TestServerRedirectsAndFallsBack(t *testing.T) {
	manifest := &protocol.Manifest{
		ContentHash: "hash-1",
		Master: protocol.MasterNode{
			ID:          "master",
			Region:      "us-west",
			SiteBaseURL: "http://master.example",
		},
	}
	registry := NewRegistry()
	registry.SetManifest(manifest)
	registry.Register(protocol.RegisteredNode{
		ID:          "worker-1",
		Region:      "us-west",
		PublicURL:   "http://worker-1.example:8081",
		Status:      "healthy",
		ContentHash: "hash-1",
	})

	handler := NewServer(Config{
		Manifest: manifest,
		Registry: registry,
		LocalFallback: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("fallback"))
		}),
	})

	req := httptest.NewRequest(http.MethodGet, "http://master.example/index.html?region=us-west", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("expected redirect, got %d", rec.Code)
	}
	location := rec.Header().Get("Location")
	if !strings.HasPrefix(location, "http://worker-1.example:8081/index.html") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	registry.Drop("worker-1")
	req = httptest.NewRequest(http.MethodGet, "http://master.example/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "fallback" {
		t.Fatalf("expected fallback response, got %d %q", rec.Code, rec.Body.String())
	}
}
