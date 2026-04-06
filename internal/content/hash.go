package content

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func HashTree(root string, excludes ...string) (string, error) {
	excluded := make(map[string]struct{}, len(excludes))
	for _, item := range excludes {
		clean := filepath.ToSlash(filepath.Clean(item))
		if clean != "." && clean != "" {
			excluded[clean] = struct{}{}
		}
	}

	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if shouldIgnore(rel, d.IsDir(), excluded) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walk tree %s: %w", root, err)
	}

	sort.Strings(files)
	hash := sha256.New()
	for _, rel := range files {
		if _, err := io.WriteString(hash, rel+"\n"); err != nil {
			return "", err
		}
		f, err := os.Open(filepath.Join(root, rel))
		if err != nil {
			return "", fmt.Errorf("open %s: %w", rel, err)
		}
		if _, err := io.Copy(hash, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("hash %s: %w", rel, err)
		}
		_ = f.Close()
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func shouldIgnore(rel string, isDir bool, excluded map[string]struct{}) bool {
	clean := filepath.ToSlash(filepath.Clean(rel))
	if _, ok := excluded[clean]; ok {
		return true
	}
	for item := range excluded {
		if strings.HasPrefix(clean, item+"/") {
			return true
		}
	}

	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case ".git", ".decent":
		return true
	}
	if rel == "decent-node.pid" || rel == "decent-node.log" {
		return true
	}
	if strings.HasPrefix(parts[0], ".DS_Store") {
		return true
	}
	return false
}
