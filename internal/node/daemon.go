package node

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	sharedconfig "github.com/kcodes0/decent/internal/config"
	"github.com/kcodes0/decent/internal/discovery"
	"github.com/kcodes0/decent/internal/master"
	"github.com/kcodes0/decent/internal/protocol"
)

type Daemon struct {
	cfg       protocol.LocalConfig
	startedAt time.Time
	logger    *log.Logger
	master    *masterClient

	mu            sync.RWMutex
	manifest      *protocol.Manifest
	healthy       bool
	currentHash   string
	lastError     string
	lastSync      time.Time
	lastHeartbeat time.Time
	heartbeatRTT  time.Duration

	serverMu  sync.Mutex
	publicSrv *http.Server
	adminSrv  *http.Server
}

func NewDaemon(cfg protocol.LocalConfig) *Daemon {
	cfg = normalizeConfig(cfg)
	return &Daemon{
		cfg:       cfg,
		startedAt: time.Now(),
		logger:    log.Default(),
		master:    newMasterClient(cfg.MasterAPI),
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	if strings.EqualFold(d.cfg.Role, "master") {
		return d.runMaster(ctx)
	}
	return d.runWorker(ctx)
}

func (d *Daemon) runWorker(ctx context.Context) error {
	if err := ensureRepo(ctx, d.cfg.Repo, d.cfg.RepoDir); err != nil {
		return err
	}
	if err := d.syncOnce(ctx); err != nil {
		return err
	}
	if err := d.startWorkerServers(); err != nil {
		return err
	}
	if err := d.registerWithMaster(ctx); err != nil {
		d.logger.Printf("master registration failed: %v", err)
	}
	if err := d.sendHeartbeat(ctx); err != nil {
		d.logger.Printf("initial heartbeat failed: %v", err)
	}

	go d.syncLoop(ctx)
	go d.heartbeatLoop(ctx)

	<-ctx.Done()
	return d.shutdownServers()
}

func (d *Daemon) runMaster(ctx context.Context) error {
	manifest, err := sharedconfig.ReadManifest(d.cfg.RepoDir)
	if err != nil {
		return err
	}
	d.mu.Lock()
	d.manifest = manifest
	d.healthy = true
	d.currentHash = manifest.ContentHash
	d.lastSync = time.Now().UTC()
	d.mu.Unlock()

	registry := master.NewRegistry()
	registry.SetManifest(manifest)
	handler := master.NewServer(master.Config{
		Manifest:       manifest,
		Registry:       registry,
		Selector:       discovery.NewSelector(d.cfg.Region),
		LocalFallback:  http.FileServer(http.Dir(d.serveRoot())),
		RedirectStatus: http.StatusFound,
	})

	addr := net.JoinHostPort(bindHost(d.cfg.PublicHost), strconv.Itoa(d.cfg.PublicPort))
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if next, err := sharedconfig.ReadManifest(d.cfg.RepoDir); err == nil {
					registry.SetManifest(next)
					d.mu.Lock()
					d.manifest = next
					d.currentHash = next.ContentHash
					d.lastSync = time.Now().UTC()
					d.mu.Unlock()
				}
			}
		}
	}()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	d.logger.Printf("serving master on %s", addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (d *Daemon) startWorkerServers() error {
	d.serverMu.Lock()
	defer d.serverMu.Unlock()

	publicAddr := net.JoinHostPort(bindHost(d.cfg.PublicHost), strconv.Itoa(d.cfg.PublicPort))
	adminAddr := net.JoinHostPort(bindHost(d.cfg.PublicHost), strconv.Itoa(d.cfg.AdminPort))

	publicLn, err := net.Listen("tcp", publicAddr)
	if err != nil {
		return fmt.Errorf("listen public %s: %w", publicAddr, err)
	}
	adminLn, err := net.Listen("tcp", adminAddr)
	if err != nil {
		_ = publicLn.Close()
		return fmt.Errorf("listen admin %s: %w", adminAddr, err)
	}

	d.publicSrv = &http.Server{Handler: d.publicHandler(), ReadHeaderTimeout: 5 * time.Second}
	d.adminSrv = &http.Server{Handler: d.adminHandler(), ReadHeaderTimeout: 5 * time.Second}

	go func() {
		if err := d.publicSrv.Serve(publicLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.logger.Printf("public server stopped: %v", err)
		}
	}()
	go func() {
		if err := d.adminSrv.Serve(adminLn); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.logger.Printf("admin server stopped: %v", err)
		}
	}()

	d.logger.Printf("serving worker site on %s", publicAddr)
	d.logger.Printf("serving worker admin on %s", adminAddr)
	return nil
}

func (d *Daemon) shutdownServers() error {
	d.serverMu.Lock()
	defer d.serverMu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if d.publicSrv != nil {
		_ = d.publicSrv.Shutdown(ctx)
	}
	if d.adminSrv != nil {
		_ = d.adminSrv.Shutdown(ctx)
	}
	return nil
}

func (d *Daemon) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.SyncInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.syncOnce(ctx); err != nil {
				d.setError(err)
				d.logger.Printf("sync failed: %v", err)
			}
		}
	}
}

func (d *Daemon) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(d.cfg.HeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			if err := d.sendHeartbeat(ctx); err != nil {
				d.setError(err)
				d.logger.Printf("heartbeat failed: %v", err)
				continue
			}
			d.mu.Lock()
			d.lastHeartbeat = time.Now().UTC()
			d.heartbeatRTT = time.Since(start)
			d.mu.Unlock()
		}
	}
}

func (d *Daemon) sendHeartbeat(ctx context.Context) error {
	if d.master == nil {
		return nil
	}
	node := d.localNodeSnapshot()
	return d.master.Heartbeat(ctx, protocol.HeartbeatRequest{
		NodeID:           node.ID,
		Region:           node.Region,
		PublicURL:        node.PublicURL,
		AdminURL:         node.AdminURL,
		ContentHash:      node.ContentHash,
		UptimeSeconds:    int64(time.Since(d.startedAt).Seconds()),
		LatencyMillis:    d.heartbeatRTT.Milliseconds(),
		Healthy:          node.Status == "healthy",
		ObservedAt:       time.Now().UTC(),
		MaxBandwidthMbps: node.MaxBandwidthMbps,
		MaxStorageMB:     node.MaxStorageMB,
	})
}

func (d *Daemon) registerWithMaster(ctx context.Context) error {
	if d.master == nil {
		return nil
	}
	return d.master.Register(ctx, d.localNodeSnapshot())
}

func (d *Daemon) syncOnce(ctx context.Context) error {
	if err := syncRepo(ctx, d.cfg.RepoDir); err != nil {
		return err
	}

	manifest, err := sharedconfig.ReadManifest(d.cfg.RepoDir)
	if err != nil {
		return err
	}
	hash, err := HashTree(d.contentRoot())
	if err != nil {
		return err
	}
	if manifest.ContentHash != "" && hash != manifest.ContentHash {
		if err := hardResetRepo(ctx, d.cfg.RepoDir); err != nil {
			return err
		}
		manifest, err = sharedconfig.ReadManifest(d.cfg.RepoDir)
		if err != nil {
			return err
		}
		hash, err = HashTree(d.contentRoot())
		if err != nil {
			return err
		}
		if hash != manifest.ContentHash {
			return fmt.Errorf("content hash mismatch after reset: local=%s expected=%s", hash, manifest.ContentHash)
		}
	}

	d.mu.Lock()
	d.manifest = manifest
	d.currentHash = hash
	d.lastSync = time.Now().UTC()
	d.lastError = ""
	d.healthy = true
	d.mu.Unlock()
	return nil
}

func (d *Daemon) contentRoot() string {
	if d.cfg.SiteDir != "" {
		return filepath.Clean(d.cfg.SiteDir)
	}
	return filepath.Clean(d.cfg.RepoDir)
}

func (d *Daemon) serveRoot() string {
	return d.contentRoot()
}

func (d *Daemon) manifestSnapshot() (protocol.Manifest, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.manifest == nil {
		return protocol.Manifest{}, false
	}
	return *d.manifest, true
}

func (d *Daemon) manifestPointer() *protocol.Manifest {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.manifest == nil {
		return nil
	}
	copy := *d.manifest
	return &copy
}

func (d *Daemon) snapshot() statusPayload {
	return statusPayload{
		StatusResponse: protocol.StatusResponse{
			LocalNode: d.localNodeSnapshot(),
			Manifest:  d.manifestPointer(),
		},
		Healthy:       d.isHealthy(),
		CurrentHash:   d.currentHashSnapshot(),
		LastError:     d.lastErrorSnapshot(),
		LastSync:      d.lastSyncSnapshot(),
		LastHeartbeat: d.lastHeartbeatSnapshot(),
		StartedAt:     d.startedAt,
	}
}

func (d *Daemon) localNodeSnapshot() protocol.RegisteredNode {
	d.mu.RLock()
	defer d.mu.RUnlock()
	status := "starting"
	if d.healthy {
		status = "healthy"
	}
	if d.lastError != "" {
		status = "degraded"
	}
	return protocol.RegisteredNode{
		ID:               d.cfg.NodeID,
		Role:             d.cfg.Role,
		Region:           d.cfg.Region,
		PublicURL:        joinURL(bindHost(d.cfg.PublicHost), d.cfg.PublicPort),
		AdminURL:         joinURL(bindHost(d.cfg.PublicHost), d.cfg.AdminPort),
		MaxBandwidthMbps: d.cfg.MaxBandwidthMbps,
		MaxStorageMB:     d.cfg.MaxStorageMB,
		Status:           status,
		LastSeenAt:       time.Now().UTC(),
		UptimeSeconds:    int64(time.Since(d.startedAt).Seconds()),
		LatencyMillis:    d.heartbeatRTT.Milliseconds(),
		ContentHash:      d.currentHash,
	}
}

func (d *Daemon) isHealthy() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.healthy
}

func (d *Daemon) currentHashSnapshot() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentHash
}

func (d *Daemon) lastErrorSnapshot() string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastError
}

func (d *Daemon) lastSyncSnapshot() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastSync
}

func (d *Daemon) lastHeartbeatSnapshot() time.Time {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.lastHeartbeat
}

func (d *Daemon) setError(err error) {
	if err == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.healthy = false
	d.lastError = err.Error()
}

type statusPayload struct {
	protocol.StatusResponse
	Healthy       bool      `json:"healthy"`
	CurrentHash   string    `json:"current_hash"`
	LastError     string    `json:"last_error,omitempty"`
	LastSync      time.Time `json:"last_sync,omitempty"`
	LastHeartbeat time.Time `json:"last_heartbeat,omitempty"`
	StartedAt     time.Time `json:"started_at"`
}

func bindHost(host string) string {
	if host == "" {
		return "127.0.0.1"
	}
	return host
}
