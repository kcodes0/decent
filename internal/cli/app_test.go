package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/protocol"
	"github.com/kcodes0/decent/internal/testutil"
	"github.com/kcodes0/decent/internal/version"
)

func TestSetupCmdWritesConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	app, stdout := newTestApp(t, home, strings.Join([]string{
		"us-west",
		"127.0.0.1",
		"28081",
		"28082",
		"http://127.0.0.1:28080",
		"http://127.0.0.1:28080",
		"150",
		"4096",
		"90",
		"15",
	}, "\n")+"\n")

	if err := app.setupCmd(); err != nil {
		t.Fatalf("setupCmd: %v", err)
	}
	cfg, err := config.ReadLocalConfig()
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if cfg == nil || cfg.Region != "us-west" || cfg.PublicPort != 28081 || cfg.HeartbeatInterval.Seconds() != 15 {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if !strings.Contains(stdout.String(), "Worker setup is saved") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestInitCmdWritesManifestAndCommits(t *testing.T) {
	home := t.TempDir()
	repoDir := filepath.Join(t.TempDir(), "site")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	testutil.MustRun(t, repoDir, "git", "init", "-b", "main")
	testutil.ConfigureGitUser(t, repoDir)
	testutil.MustWriteFile(t, filepath.Join(repoDir, "index.html"), "<h1>hello</h1>")

	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	testutil.MakeExecutable(t, filepath.Join(fakeBin, "gh"), "#!/bin/sh\nif [ \"$1\" = \"auth\" ] && [ \"$2\" = \"status\" ]; then exit 0; fi\nexit 0\n")

	t.Setenv("HOME", home)
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app, stdout := newTestApp(t, repoDir, strings.Join([]string{
		"my-site",
		"kcodes0/my-site",
		"us-west",
		"http://127.0.0.1:18080",
		"http://127.0.0.1:18080",
		"y",
	}, "\n")+"\n")

	if err := app.initCmd(); err != nil {
		t.Fatalf("initCmd: %v", err)
	}

	manifest, err := config.ReadManifest(repoDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if manifest.SiteName != "my-site" || manifest.Master.Region != "us-west" || manifest.ContentHash == "" {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	localCfg, err := config.ReadLocalConfig()
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if localCfg == nil || localCfg.Role != "master" || localCfg.PublicPort != 18080 {
		t.Fatalf("unexpected local config: %#v", localCfg)
	}
	logOut := testutil.MustRun(t, repoDir, "git", "log", "--oneline", "-1")
	if !strings.Contains(logOut, "Initialize decent manifest") {
		t.Fatalf("expected init commit, got %s", logOut)
	}
	if !strings.Contains(stdout.String(), "Your main site is ready.") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestHostCmdClonesRepoAndStartsDaemon(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	baseDir := t.TempDir()
	_, remoteDir, _ := testutil.CreateStaticSiteRepo(t, baseDir, "http://127.0.0.1:18080", "http://127.0.0.1:18080", "us-west", map[string]string{
		"index.html": "<h1>hello worker</h1>",
	})

	initial := defaultConfig("worker")
	initial.PublicHost = "127.0.0.1"
	initial.PublicPort = 28081
	initial.AdminPort = 28082
	if err := config.WriteLocalConfig(initial); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	fakeBin := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	argFile := filepath.Join(t.TempDir(), "daemon-args.txt")
	testutil.MakeExecutable(t, filepath.Join(fakeBin, "decent-node"), "#!/bin/sh\nprintf '%s\n' \"$@\" > \""+argFile+"\"\nexit 0\n")
	t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"))

	app, stdout := newTestApp(t, baseDir, "")
	if err := app.hostCmd(remoteDir); err != nil {
		t.Fatalf("hostCmd: %v", err)
	}

	cfg, err := config.ReadLocalConfig()
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if cfg == nil || cfg.Repo != remoteDir || cfg.RepoDir == "" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
	if _, err := os.Stat(filepath.Join(cfg.RepoDir, ".git")); err != nil {
		t.Fatalf("expected cloned repo: %v", err)
	}
	var argsBody []byte
	testutil.WaitFor(t, 3*time.Second, func() (bool, string) {
		body, err := os.ReadFile(argFile)
		if err != nil {
			return false, err.Error()
		}
		argsBody = body
		return len(body) > 0, "waiting for daemon args"
	})
	if !strings.Contains(string(argsBody), "--config") {
		t.Fatalf("expected daemon start args, got %s", string(argsBody))
	}
	if !strings.Contains(stdout.String(), "This machine is now hosting") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestStatusCmdPrintsLocalAndNetworkStatus(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	adminServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"healthy":      true,
			"current_hash": "abc123",
			"local_node": map[string]any{
				"id":           "worker-1",
				"content_hash": "abc123",
			},
			"known_nodes": []map[string]any{{"id": "worker-1"}},
		})
	}))
	defer adminServer.Close()

	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(protocol.StatusResponse{
			HealthyNodes: []protocol.RegisteredNode{{ID: "worker-1", Region: "us-west", PublicURL: "http://127.0.0.1:28081"}},
		})
	}))
	defer masterServer.Close()

	adminURL, err := url.Parse(adminServer.URL)
	if err != nil {
		t.Fatalf("parse admin URL: %v", err)
	}
	adminPort, err := strconv.Atoi(adminURL.Port())
	if err != nil {
		t.Fatalf("parse admin port: %v", err)
	}

	cfg := &protocol.LocalConfig{
		NodeID:     "worker-1",
		Role:       "worker",
		Region:     "us-west",
		Repo:       "owner/repo",
		PublicHost: adminURL.Hostname(),
		AdminPort:  adminPort,
		MasterAPI:  masterServer.URL,
	}
	if err := config.WriteLocalConfig(cfg); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}

	app, stdout := newTestApp(t, home, "")
	if err := app.statusCmd(); err != nil {
		t.Fatalf("statusCmd: %v", err)
	}
	out := stdout.String()
	if !strings.Contains(out, "Local node: worker-1") || !strings.Contains(out, "Network:") || !strings.Contains(out, "worker-1") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestPushCmdUpdatesManifestCommitsAndPushes(t *testing.T) {
	masterServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(protocol.StatusResponse{
			KnownNodes: []protocol.RegisteredNode{{ID: "worker-1", Region: "us-west", PublicURL: "http://127.0.0.1:28081", Status: "healthy"}},
		})
	}))
	defer masterServer.Close()

	baseDir := t.TempDir()
	repoDir, remoteDir, manifest := testutil.CreateStaticSiteRepo(t, baseDir, masterServer.URL, masterServer.URL, "us-west", map[string]string{
		"index.html": "<h1>v1</h1>",
	})

	testutil.MustWriteFile(t, filepath.Join(repoDir, "index.html"), "<h1>v2</h1>")
	app, stdout := newTestApp(t, repoDir, "")
	before := manifest.ContentHash

	if err := app.pushCmd(); err != nil {
		t.Fatalf("pushCmd: %v", err)
	}

	updated, err := config.ReadManifest(repoDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if updated.ContentHash == before {
		t.Fatalf("expected content hash to change")
	}
	if len(updated.Nodes) != 2 {
		t.Fatalf("expected master + worker nodes in manifest, got %#v", updated.Nodes)
	}
	cloneDir := filepath.Join(t.TempDir(), "clone")
	testutil.MustRun(t, "", "git", "clone", remoteDir, cloneDir)
	remoteManifest, err := config.ReadManifest(cloneDir)
	if err != nil {
		t.Fatalf("ReadManifest remote clone: %v", err)
	}
	if remoteManifest.ContentHash != updated.ContentHash {
		t.Fatalf("expected pushed manifest hash %s, got %s", updated.ContentHash, remoteManifest.ContentHash)
	}
	logOut := testutil.MustRun(t, repoDir, "git", "log", "--oneline", "-1")
	if !strings.Contains(logOut, "Update decent content hash") {
		t.Fatalf("expected push commit, got %s", logOut)
	}
	if !strings.Contains(stdout.String(), "The site manifest is updated") {
		t.Fatalf("unexpected output: %s", stdout.String())
	}
}

func TestUsageAndVersionOutput(t *testing.T) {
	app, stdout := newTestApp(t, t.TempDir(), "")
	if err := app.run([]string{"version"}); err != nil {
		t.Fatalf("version command: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != version.Current {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}

	app, stdout = newTestApp(t, t.TempDir(), "")
	if err := app.usage(); err != nil {
		t.Fatalf("usage: %v", err)
	}
	if !strings.Contains(stdout.String(), version.Current) || !strings.Contains(stdout.String(), "Set up this machine as a worker node") {
		t.Fatalf("unexpected usage output: %s", stdout.String())
	}
}

func newTestApp(t *testing.T, cwd string, input string) (*App, *bytes.Buffer) {
	t.Helper()
	var stdout bytes.Buffer
	app := &App{
		reader: bufio.NewReader(strings.NewReader(input)),
		stdout: &stdout,
		stderr: &stdout,
		cwd:    cwd,
	}
	return app, &stdout
}
