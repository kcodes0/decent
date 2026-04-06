package config

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kcodes0/decent/internal/protocol"
)

func TestManifestAndLocalConfigRoundTrip(t *testing.T) {
	repoDir := t.TempDir()
	cfgDir := t.TempDir()
	t.Setenv("HOME", cfgDir)

	manifest := &protocol.Manifest{
		Version:     "v0",
		SiteName:    "site",
		Repo:        "owner/repo",
		ContentHash: "abc123",
		UpdatedAt:   time.Now().UTC().Truncate(time.Second),
		Master: protocol.MasterNode{
			ID:          "master",
			Region:      "us-west",
			APIBaseURL:  "http://127.0.0.1:8080",
			SiteBaseURL: "http://127.0.0.1:8080",
		},
		Nodes: []protocol.RegisteredNode{{ID: "node-1", Region: "us-west", PublicURL: "http://127.0.0.1:8081", Status: "healthy"}},
	}
	if err := WriteManifest(repoDir, manifest); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	gotManifest, err := ReadManifest(repoDir)
	if err != nil {
		t.Fatalf("ReadManifest: %v", err)
	}
	if gotManifest.SiteName != manifest.SiteName || gotManifest.ContentHash != manifest.ContentHash || gotManifest.Master.APIBaseURL != manifest.Master.APIBaseURL {
		t.Fatalf("manifest mismatch: %#v", gotManifest)
	}

	local := &protocol.LocalConfig{
		NodeID:            "node-1",
		Role:              "worker",
		Region:            "us-west",
		Repo:              "owner/repo",
		RepoDir:           filepath.Join(repoDir, "clone"),
		SiteDir:           filepath.Join(repoDir, "clone"),
		PublicHost:        "127.0.0.1",
		PublicPort:        8081,
		AdminPort:         8082,
		MasterAPI:         "http://127.0.0.1:8080",
		MasterSite:        "http://127.0.0.1:8080",
		MaxBandwidthMbps:  100,
		MaxStorageMB:      1024,
		SyncInterval:      45 * time.Second,
		HeartbeatInterval: 15 * time.Second,
		RedirectMode:      "redirect",
	}
	if err := WriteLocalConfig(local); err != nil {
		t.Fatalf("WriteLocalConfig: %v", err)
	}
	gotLocal, err := ReadLocalConfig()
	if err != nil {
		t.Fatalf("ReadLocalConfig: %v", err)
	}
	if gotLocal.NodeID != local.NodeID || gotLocal.SyncInterval != local.SyncInterval || gotLocal.HeartbeatInterval != local.HeartbeatInterval {
		t.Fatalf("local config mismatch: %#v", gotLocal)
	}
}
