package content

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kcodes0/decent/internal/config"
)

func TestHashTreeIgnoresManifestAndGitState(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "index.html"), "hello")
	mustWriteFile(t, filepath.Join(root, config.ManifestFileName), "ignored")
	mustWriteFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")

	hash1, err := HashTree(root, config.ManifestFileName)
	if err != nil {
		t.Fatalf("hash1: %v", err)
	}

	mustWriteFile(t, filepath.Join(root, config.ManifestFileName), "changed")
	mustWriteFile(t, filepath.Join(root, ".git", "FETCH_HEAD"), "changed")

	hash2, err := HashTree(root, config.ManifestFileName)
	if err != nil {
		t.Fatalf("hash2: %v", err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected manifest/git changes to be ignored: %s != %s", hash1, hash2)
	}

	mustWriteFile(t, filepath.Join(root, "index.html"), "hello world")
	hash3, err := HashTree(root, config.ManifestFileName)
	if err != nil {
		t.Fatalf("hash3: %v", err)
	}
	if hash3 == hash2 {
		t.Fatalf("expected content change to alter hash")
	}
}

func mustWriteFile(t *testing.T, path string, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}
