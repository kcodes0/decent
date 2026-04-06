package node

import (
	"fmt"
	"path/filepath"
	"time"

	sharedconfig "github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/protocol"
)

const (
	defaultSyncInterval      = 60 * time.Second
	defaultHeartbeatInterval = 30 * time.Second
	defaultPublicPort        = 8081
	defaultAdminPort         = 8082
	defaultRedirectMode      = "redirect"
)

func LoadConfig(path string) (protocol.LocalConfig, error) {
	cfg, err := sharedconfig.ReadLocalConfigPath(path)
	if err != nil {
		return protocol.LocalConfig{}, err
	}
	if cfg == nil {
		return protocol.LocalConfig{}, fmt.Errorf("missing config at %s", path)
	}
	return normalizeConfig(*cfg), nil
}

func SaveConfig(path string, cfg protocol.LocalConfig) error {
	cfg = normalizeConfig(cfg)
	return sharedconfig.WriteLocalConfigPath(path, &cfg)
}

func normalizeConfig(cfg protocol.LocalConfig) protocol.LocalConfig {
	if cfg.NodeID == "" {
		cfg.NodeID = fmt.Sprintf("%s-%d", cfg.Role, time.Now().UnixNano())
	}
	if cfg.Role == "" {
		cfg.Role = "worker"
	}
	if cfg.Region == "" {
		cfg.Region = "local"
	}
	if cfg.RepoDir == "" {
		cfg.RepoDir = filepath.Join(".", "site")
	}
	if cfg.SiteDir == "" {
		cfg.SiteDir = cfg.RepoDir
	}
	if cfg.PublicHost == "" {
		cfg.PublicHost = "127.0.0.1"
	}
	if cfg.PublicPort == 0 {
		cfg.PublicPort = defaultPublicPort
	}
	if cfg.AdminPort == 0 {
		cfg.AdminPort = defaultAdminPort
	}
	if cfg.SyncInterval <= 0 {
		cfg.SyncInterval = defaultSyncInterval
	}
	if cfg.HeartbeatInterval <= 0 {
		cfg.HeartbeatInterval = defaultHeartbeatInterval
	}
	if cfg.RedirectMode == "" {
		cfg.RedirectMode = defaultRedirectMode
	}
	return cfg
}
