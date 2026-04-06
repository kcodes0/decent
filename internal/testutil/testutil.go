package testutil

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/content"
	"github.com/kcodes0/decent/internal/protocol"
)

func MustWriteFile(t testing.TB, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func MustRun(t testing.TB, dir string, name string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("%s %v failed: %v\nstdout:\n%s\nstderr:\n%s", name, args, err, stdout.String(), stderr.String())
	}
	return stdout.String()
}

func ConfigureGitUser(t testing.TB, repoDir string) {
	t.Helper()
	MustRun(t, repoDir, "git", "config", "user.name", "decent-test")
	MustRun(t, repoDir, "git", "config", "user.email", "decent-test@example.com")
}

func CreateStaticSiteRepo(t testing.TB, baseDir string, masterAPI string, masterSite string, region string, files map[string]string) (string, string, *protocol.Manifest) {
	t.Helper()

	repoDir := filepath.Join(baseDir, "repo")
	remoteDir := filepath.Join(baseDir, "remote.git")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}

	MustRun(t, repoDir, "git", "init", "-b", "main")
	ConfigureGitUser(t, repoDir)

	writeFiles(t, repoDir, files)
	hash, err := content.HashTree(repoDir, config.ManifestFileName)
	if err != nil {
		t.Fatalf("hash repo: %v", err)
	}

	manifest := &protocol.Manifest{
		Version:     "v0",
		SiteName:    "test-site",
		Repo:        remoteDir,
		ContentHash: hash,
		UpdatedAt:   time.Now().UTC(),
		Master: protocol.MasterNode{
			ID:          "master",
			Region:      region,
			APIBaseURL:  masterAPI,
			SiteBaseURL: masterSite,
		},
	}
	if err := config.WriteManifest(repoDir, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	MustRun(t, repoDir, "git", "add", ".")
	MustRun(t, repoDir, "git", "commit", "-m", "initial site")
	MustRun(t, baseDir, "git", "clone", "--bare", repoDir, remoteDir)
	MustRun(t, repoDir, "git", "remote", "add", "origin", remoteDir)
	MustRun(t, repoDir, "git", "push", "-u", "origin", "main")

	return repoDir, remoteDir, manifest
}

func UpdateStaticSiteRepo(t testing.TB, repoDir string, files map[string]string) *protocol.Manifest {
	t.Helper()

	writeFiles(t, repoDir, files)
	manifest, err := config.ReadManifest(repoDir)
	if err != nil {
		t.Fatalf("read manifest: %v", err)
	}
	hash, err := content.HashTree(repoDir, config.ManifestFileName)
	if err != nil {
		t.Fatalf("hash repo: %v", err)
	}
	manifest.ContentHash = hash
	manifest.UpdatedAt = time.Now().UTC()
	if err := config.WriteManifest(repoDir, manifest); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	MustRun(t, repoDir, "git", "add", ".")
	MustRun(t, repoDir, "git", "commit", "-m", "update site")
	MustRun(t, repoDir, "git", "push", "origin", "main")
	return manifest
}

func FreePort(t testing.TB) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for free port: %v", err)
	}
	defer ln.Close()
	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected addr type %T", ln.Addr())
	}
	return addr.Port
}

func WaitFor(t testing.TB, timeout time.Duration, fn func() (bool, string)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var detail string
	for time.Now().Before(deadline) {
		ok, nextDetail := fn()
		if ok {
			return
		}
		detail = nextDetail
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s: %s", timeout, detail)
}

func MakeExecutable(t testing.TB, path string, body string) {
	t.Helper()
	MustWriteFile(t, path, body)
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatalf("chmod %s: %v", path, err)
	}
}

func writeFiles(t testing.TB, root string, files map[string]string) {
	t.Helper()
	keys := make([]string, 0, len(files))
	for path := range files {
		keys = append(keys, path)
	}
	sort.Strings(keys)
	for _, path := range keys {
		MustWriteFile(t, filepath.Join(root, path), files[path])
	}
}

func URL(host string, port int) string {
	return fmt.Sprintf("http://%s:%d", host, port)
}
