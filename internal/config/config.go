package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kcodes0/decent/internal/protocol"
	toml "github.com/pelletier/go-toml/v2"
)

const (
	ManifestFileName = "decent.toml"
)

func ConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve user config dir: %w", err)
	}
	return filepath.Join(dir, "decent"), nil
}

func LocalConfigPath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "node.toml"), nil
}

func PidFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "decent-node.pid"), nil
}

func LogFilePath() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "decent-node.log"), nil
}

func StateDir() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "state"), nil
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("create dir %s: %w", path, err)
	}
	return nil
}

func ReadManifest(repoDir string) (*protocol.Manifest, error) {
	path := filepath.Join(repoDir, ManifestFileName)
	return ReadManifestPath(path)
}

func ReadManifestPath(path string) (*protocol.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	var manifest protocol.Manifest
	if err := toml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("decode manifest %s: %w", path, err)
	}
	return &manifest, nil
}

func WriteManifest(repoDir string, manifest *protocol.Manifest) error {
	path := filepath.Join(repoDir, ManifestFileName)
	return WriteManifestPath(path, manifest)
}

func WriteManifestPath(path string, manifest *protocol.Manifest) error {
	data, err := toml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("encode manifest %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

func ReadLocalConfig() (*protocol.LocalConfig, error) {
	path, err := LocalConfigPath()
	if err != nil {
		return nil, err
	}
	return ReadLocalConfigPath(path)
}

func ReadLocalConfigPath(path string) (*protocol.LocalConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read local config %s: %w", path, err)
	}
	var cfg protocol.LocalConfig
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("decode local config %s: %w", path, err)
	}
	return &cfg, nil
}

func WriteLocalConfig(cfg *protocol.LocalConfig) error {
	path, err := LocalConfigPath()
	if err != nil {
		return err
	}
	return WriteLocalConfigPath(path, cfg)
}

func WriteLocalConfigPath(path string, cfg *protocol.LocalConfig) error {
	if err := EnsureDir(filepath.Dir(path)); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode local config %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write local config %s: %w", path, err)
	}
	return nil
}
