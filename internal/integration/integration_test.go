package integration

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kcodes0/decent/internal/node"
	"github.com/kcodes0/decent/internal/protocol"
	"github.com/kcodes0/decent/internal/testutil"
	"github.com/kcodes0/decent/internal/version"
)

func TestMasterAndWorkerEndToEndRouting(t *testing.T) {
	baseDir := t.TempDir()
	masterPort := testutil.FreePort(t)
	workerPort := testutil.FreePort(t)
	workerAdminPort := testutil.FreePort(t)
	masterURL := testutil.URL("127.0.0.1", masterPort)

	repoDir, remoteDir, _ := testutil.CreateStaticSiteRepo(t, baseDir, masterURL, masterURL, "us-west", map[string]string{
		"index.html": "<h1>end-to-end</h1>",
	})

	masterCfg := protocol.LocalConfig{
		NodeID:            "master",
		Role:              "master",
		Region:            "us-west",
		Repo:              remoteDir,
		RepoDir:           repoDir,
		SiteDir:           repoDir,
		PublicHost:        "127.0.0.1",
		PublicPort:        masterPort,
		AdminPort:         masterPort,
		MasterAPI:         masterURL,
		MasterSite:        masterURL,
		MaxBandwidthMbps:  1000,
		MaxStorageMB:      102400,
		SyncInterval:      time.Hour,
		HeartbeatInterval: time.Hour,
		RedirectMode:      "redirect",
	}
	workerCfg := protocol.LocalConfig{
		NodeID:            "worker-1",
		Role:              "worker",
		Region:            "us-west",
		Repo:              remoteDir,
		RepoDir:           filepath.Join(baseDir, "worker-clone"),
		SiteDir:           filepath.Join(baseDir, "worker-clone"),
		PublicHost:        "127.0.0.1",
		PublicPort:        workerPort,
		AdminPort:         workerAdminPort,
		MasterAPI:         masterURL,
		MasterSite:        masterURL,
		MaxBandwidthMbps:  100,
		MaxStorageMB:      4096,
		SyncInterval:      time.Hour,
		HeartbeatInterval: 100 * time.Millisecond,
		RedirectMode:      "redirect",
	}

	masterCtx, masterCancel := context.WithCancel(context.Background())
	defer masterCancel()
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	masterErrCh := make(chan error, 1)
	go func() {
		masterErrCh <- node.NewDaemon(masterCfg).Run(masterCtx)
	}()

	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(masterURL + "/api/status")
		if err != nil {
			return false, err.Error()
		}
		resp.Body.Close()
		return resp.StatusCode == http.StatusOK, resp.Status
	})

	workerErrCh := make(chan error, 1)
	go func() {
		workerErrCh <- node.NewDaemon(workerCfg).Run(workerCtx)
	}()

	testutil.WaitFor(t, 5*time.Second, func() (bool, string) {
		resp, err := http.Get(masterURL + "/api/status")
		if err != nil {
			return false, err.Error()
		}
		defer resp.Body.Close()
		var status protocol.StatusResponse
		if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
			return false, err.Error()
		}
		return len(status.HealthyNodes) == 1 && status.HealthyNodes[0].ID == "worker-1", "waiting for healthy worker"
	})

	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(masterURL + "/index.html?region=us-west")
	if err != nil {
		t.Fatalf("master request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, testutil.URL("127.0.0.1", workerPort)+"/index.html") {
		t.Fatalf("unexpected redirect location: %s", location)
	}

	workerCancel()
	if err := <-workerErrCh; err != nil {
		t.Fatalf("worker exit: %v", err)
	}
	masterCancel()
	if err := <-masterErrCh; err != nil {
		t.Fatalf("master exit: %v", err)
	}
}

func TestInstallScriptInstallsFromLocalSource(t *testing.T) {
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("repo root: %v", err)
	}
	home := t.TempDir()
	installDir := filepath.Join(home, "bin")
	goModCache := filepath.Join(home, "gomodcache")
	goCache := filepath.Join(home, "gocache")

	cmd := exec.Command("sh", filepath.Join(repoRoot, "install.sh"))
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"DECENT_INSTALL_DIR="+installDir,
		"DECENT_SOURCE_DIR="+repoRoot,
		"GOMODCACHE="+goModCache,
		"GOCACHE="+goCache,
		"GOFLAGS=-modcacherw",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("install.sh failed: %v\n%s", err, string(out))
	}

	out, err := exec.Command(filepath.Join(installDir, "decent"), "version").CombinedOutput()
	if err != nil {
		t.Fatalf("decent version failed: %v\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != version.Current {
		t.Fatalf("unexpected decent version: %q", string(out))
	}

	out, err = exec.Command(filepath.Join(installDir, "decent-node"), "version").CombinedOutput()
	if err != nil {
		t.Fatalf("decent-node version failed: %v\n%s", err, string(out))
	}
	if strings.TrimSpace(string(out)) != version.Current {
		t.Fatalf("unexpected decent-node version: %q", string(out))
	}
}
