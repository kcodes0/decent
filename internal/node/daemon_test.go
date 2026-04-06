package node

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kcodes0/decent/internal/protocol"
	"github.com/kcodes0/decent/internal/testutil"
)

func TestWorkerDaemonRegistersHeartbeatsAndServesContent(t *testing.T) {
	var mu sync.Mutex
	registers := 0
	heartbeats := 0

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/register":
			mu.Lock()
			registers++
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(protocol.RegisterResponse{Accepted: true})
		case "/api/heartbeat":
			mu.Lock()
			heartbeats++
			mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer masterServer.Close()

	baseDir := t.TempDir()
	_, remoteDir, _ := testutil.CreateStaticSiteRepo(t, baseDir, masterServer.URL, masterServer.URL, "us-west", map[string]string{
		"index.html": "<h1>worker</h1>",
	})

	cfg := protocol.LocalConfig{
		NodeID:            "worker-1",
		Role:              "worker",
		Region:            "us-west",
		Repo:              remoteDir,
		RepoDir:           filepath.Join(baseDir, "worker-clone"),
		SiteDir:           filepath.Join(baseDir, "worker-clone"),
		PublicHost:        "127.0.0.1",
		PublicPort:        testutil.FreePort(t),
		AdminPort:         testutil.FreePort(t),
		MasterAPI:         masterServer.URL,
		MasterSite:        masterServer.URL,
		MaxBandwidthMbps:  100,
		MaxStorageMB:      1024,
		SyncInterval:      200 * time.Millisecond,
		HeartbeatInterval: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- NewDaemon(cfg).Run(ctx)
	}()

	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(testutil.URL("127.0.0.1", cfg.PublicPort) + "/")
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err.Error()
		}
		return strings.Contains(string(body), "worker"), string(body)
	})

	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		mu.Lock()
		defer mu.Unlock()
		return registers >= 1 && heartbeats >= 1, "waiting for register and heartbeat"
	})

	resp, err := http.Get(testutil.URL("127.0.0.1", cfg.AdminPort) + "/health")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected healthy admin endpoint, got %d", resp.StatusCode)
	}

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("daemon exit: %v", err)
	}
}

func TestWorkerDaemonSyncRepairsTamperingAndAppliesRemoteUpdate(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/register":
			_ = json.NewEncoder(w).Encode(protocol.RegisterResponse{Accepted: true})
		case "/api/heartbeat":
			_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
		default:
			http.NotFound(w, r)
		}
	}))
	defer masterServer.Close()

	baseDir := t.TempDir()
	repoDir, remoteDir, _ := testutil.CreateStaticSiteRepo(t, baseDir, masterServer.URL, masterServer.URL, "us-west", map[string]string{
		"index.html": "<h1>v1</h1>",
	})

	cfg := protocol.LocalConfig{
		NodeID:            "worker-1",
		Role:              "worker",
		Region:            "us-west",
		Repo:              remoteDir,
		RepoDir:           filepath.Join(baseDir, "worker-clone"),
		SiteDir:           filepath.Join(baseDir, "worker-clone"),
		PublicHost:        "127.0.0.1",
		PublicPort:        testutil.FreePort(t),
		AdminPort:         testutil.FreePort(t),
		MasterAPI:         masterServer.URL,
		MasterSite:        masterServer.URL,
		MaxBandwidthMbps:  100,
		MaxStorageMB:      1024,
		SyncInterval:      time.Hour,
		HeartbeatInterval: 100 * time.Millisecond,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() {
		errCh <- NewDaemon(cfg).Run(ctx)
	}()

	workerURL := testutil.URL("127.0.0.1", cfg.PublicPort)
	adminURL := testutil.URL("127.0.0.1", cfg.AdminPort)
	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(workerURL + "/")
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(body), "v1"), string(body)
	})

	testutil.MustWriteFile(t, filepath.Join(cfg.RepoDir, "index.html"), "<h1>tampered</h1>")
	req, _ := http.NewRequest(http.MethodPost, adminURL+"/sync", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sync request after tamper: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected sync 200, got %d", resp.StatusCode)
	}
	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(workerURL + "/")
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(body), "v1"), string(body)
	})

	testutil.UpdateStaticSiteRepo(t, repoDir, map[string]string{
		"index.html": "<h1>v2</h1>",
	})
	req, _ = http.NewRequest(http.MethodPost, adminURL+"/sync", nil)
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("sync request after update: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected sync 200 after update, got %d", resp.StatusCode)
	}
	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(workerURL + "/")
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(body), "v2"), string(body)
	})

	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("daemon exit: %v", err)
	}
}
