package protocol

import "time"

type Manifest struct {
	Version     string           `json:"version" toml:"version"`
	SiteName    string           `json:"site_name" toml:"site_name"`
	Repo        string           `json:"repo" toml:"repo"`
	Master      MasterNode       `json:"master" toml:"master"`
	ContentHash string           `json:"content_hash" toml:"content_hash"`
	UpdatedAt   time.Time        `json:"updated_at" toml:"updated_at"`
	Nodes       []RegisteredNode `json:"nodes" toml:"nodes"`
}

type MasterNode struct {
	ID          string `json:"id" toml:"id"`
	Region      string `json:"region" toml:"region"`
	APIBaseURL  string `json:"api_base_url" toml:"api_base_url"`
	SiteBaseURL string `json:"site_base_url" toml:"site_base_url"`
}

type RegisteredNode struct {
	ID                        string    `json:"id" toml:"id"`
	Role                      string    `json:"role" toml:"role"`
	Region                    string    `json:"region" toml:"region"`
	PublicURL                 string    `json:"public_url" toml:"public_url"`
	AdminURL                  string    `json:"admin_url" toml:"admin_url"`
	MaxBandwidthMbps          int       `json:"max_bandwidth_mbps" toml:"max_bandwidth_mbps"`
	MaxStorageMB              int       `json:"max_storage_mb" toml:"max_storage_mb"`
	Status                    string    `json:"status" toml:"status"`
	LastSeenAt                time.Time `json:"last_seen_at" toml:"last_seen_at"`
	UptimeSeconds             int64     `json:"uptime_seconds" toml:"uptime_seconds"`
	LatencyMillis             int64     `json:"latency_millis" toml:"latency_millis"`
	ContentHash               string    `json:"content_hash" toml:"content_hash"`
	ConsecutiveHashFailures   int       `json:"consecutive_hash_failures" toml:"consecutive_hash_failures"`
	ConsecutiveHeartbeatFails int       `json:"consecutive_heartbeat_fails" toml:"consecutive_heartbeat_fails"`
}

type LocalConfig struct {
	NodeID            string        `json:"node_id" toml:"node_id"`
	Role              string        `json:"role" toml:"role"`
	Region            string        `json:"region" toml:"region"`
	Repo              string        `json:"repo" toml:"repo"`
	RepoDir           string        `json:"repo_dir" toml:"repo_dir"`
	SiteDir           string        `json:"site_dir" toml:"site_dir"`
	PublicHost        string        `json:"public_host" toml:"public_host"`
	PublicPort        int           `json:"public_port" toml:"public_port"`
	AdminPort         int           `json:"admin_port" toml:"admin_port"`
	MasterAPI         string        `json:"master_api" toml:"master_api"`
	MasterSite        string        `json:"master_site" toml:"master_site"`
	MaxBandwidthMbps  int           `json:"max_bandwidth_mbps" toml:"max_bandwidth_mbps"`
	MaxStorageMB      int           `json:"max_storage_mb" toml:"max_storage_mb"`
	SyncInterval      time.Duration `json:"sync_interval" toml:"sync_interval"`
	HeartbeatInterval time.Duration `json:"heartbeat_interval" toml:"heartbeat_interval"`
	RedirectMode      string        `json:"redirect_mode" toml:"redirect_mode"`
}

type RegisterRequest struct {
	Node RegisteredNode `json:"node"`
}

type RegisterResponse struct {
	Accepted bool   `json:"accepted"`
	Message  string `json:"message"`
}

type HeartbeatRequest struct {
	NodeID           string    `json:"node_id"`
	Region           string    `json:"region"`
	PublicURL        string    `json:"public_url"`
	AdminURL         string    `json:"admin_url"`
	ContentHash      string    `json:"content_hash"`
	UptimeSeconds    int64     `json:"uptime_seconds"`
	LatencyMillis    int64     `json:"latency_millis"`
	Healthy          bool      `json:"healthy"`
	ObservedAt       time.Time `json:"observed_at"`
	MaxBandwidthMbps int       `json:"max_bandwidth_mbps"`
	MaxStorageMB     int       `json:"max_storage_mb"`
}

type StatusResponse struct {
	LocalNode    RegisteredNode   `json:"local_node"`
	Manifest     *Manifest        `json:"manifest,omitempty"`
	KnownNodes   []RegisteredNode `json:"known_nodes"`
	HealthyNodes []RegisteredNode `json:"healthy_nodes"`
}
