package master

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/kcodes0/decent/internal/protocol"
)

type Registry struct {
	mu sync.RWMutex

	nodes    map[string]protocol.RegisteredNode
	manifest *protocol.Manifest

	healthTTL             time.Duration
	hashFailureThreshold  int
	heartbeatFailureLimit int
}

func NewRegistry() *Registry {
	return &Registry{
		nodes:                 make(map[string]protocol.RegisteredNode),
		healthTTL:             2 * time.Minute,
		hashFailureThreshold:  3,
		heartbeatFailureLimit: 5,
	}
}

func (r *Registry) SetManifest(manifest *protocol.Manifest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifest = cloneManifest(manifest)
}

func (r *Registry) SetHealthPolicy(ttl time.Duration, hashFailures int, heartbeatFails int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if ttl > 0 {
		r.healthTTL = ttl
	}
	if hashFailures > 0 {
		r.hashFailureThreshold = hashFailures
	}
	if heartbeatFails > 0 {
		r.heartbeatFailureLimit = heartbeatFails
	}
}

func (r *Registry) Register(node protocol.RegisteredNode) protocol.RegisteredNode {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	existing := r.nodes[node.ID]
	node.LastSeenAt = now
	node.Status = normalizeNodeStatus(node.Status, existing.Status)
	if node.PublicURL == "" {
		node.PublicURL = existing.PublicURL
	}
	if node.AdminURL == "" {
		node.AdminURL = existing.AdminURL
	}
	if node.Region == "" {
		node.Region = existing.Region
	}
	if node.Role == "" {
		node.Role = "worker"
	}
	if node.MaxBandwidthMbps == 0 {
		node.MaxBandwidthMbps = existing.MaxBandwidthMbps
	}
	if node.MaxStorageMB == 0 {
		node.MaxStorageMB = existing.MaxStorageMB
	}
	if node.ContentHash == "" {
		node.ContentHash = existing.ContentHash
	}
	if node.ConsecutiveHashFailures == 0 {
		node.ConsecutiveHashFailures = existing.ConsecutiveHashFailures
	}
	if node.ConsecutiveHeartbeatFails == 0 {
		node.ConsecutiveHeartbeatFails = existing.ConsecutiveHeartbeatFails
	}
	r.nodes[node.ID] = node
	return node
}

func (r *Registry) Heartbeat(req protocol.HeartbeatRequest) protocol.RegisteredNode {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	node, ok := r.nodes[req.NodeID]
	if !ok {
		node = protocol.RegisteredNode{ID: req.NodeID, Role: "worker"}
	}

	node.Region = chooseString(req.Region, node.Region)
	node.PublicURL = chooseString(req.PublicURL, node.PublicURL)
	node.AdminURL = chooseString(req.AdminURL, node.AdminURL)
	node.MaxBandwidthMbps = chooseInt(req.MaxBandwidthMbps, node.MaxBandwidthMbps)
	node.MaxStorageMB = chooseInt(req.MaxStorageMB, node.MaxStorageMB)
	node.LatencyMillis = req.LatencyMillis
	node.UptimeSeconds = req.UptimeSeconds
	node.LastSeenAt = now

	status := node.Status
	hashMismatch := false
	if req.Healthy {
		node.ConsecutiveHeartbeatFails = 0
	} else {
		node.ConsecutiveHeartbeatFails++
		status = "unhealthy"
	}

	if r.manifest != nil && strings.TrimSpace(r.manifest.ContentHash) != "" {
		if strings.TrimSpace(req.ContentHash) == strings.TrimSpace(r.manifest.ContentHash) {
			node.ConsecutiveHashFailures = 0
			node.ContentHash = req.ContentHash
		} else {
			hashMismatch = true
			node.ConsecutiveHashFailures++
			if node.ConsecutiveHashFailures >= r.hashFailureThreshold {
				status = "unhealthy"
			} else if status != "unhealthy" {
				status = "degraded"
			}
		}
	}

	if req.Healthy && !hashMismatch && node.ConsecutiveHeartbeatFails < r.heartbeatFailureLimit && node.ConsecutiveHashFailures < r.hashFailureThreshold {
		status = "healthy"
	}
	if status == "" {
		status = "unhealthy"
	}
	node.Status = status

	r.nodes[node.ID] = node
	return node
}

func (r *Registry) Drop(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.nodes, id)
}

func (r *Registry) Snapshot() ([]protocol.RegisteredNode, *protocol.Manifest) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	nodes := make([]protocol.RegisteredNode, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, r.computeHealth(node))
	}
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})
	return nodes, cloneManifest(r.manifest)
}

func (r *Registry) HealthyNodes() []protocol.RegisteredNode {
	nodes, _ := r.Snapshot()
	out := make([]protocol.RegisteredNode, 0, len(nodes))
	for _, node := range nodes {
		if strings.EqualFold(node.Status, "healthy") || strings.EqualFold(node.Status, "ready") {
			out = append(out, node)
		}
	}
	return out
}

func (r *Registry) LocalNode() protocol.RegisteredNode {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.manifest == nil {
		return protocol.RegisteredNode{Role: "master", Status: "healthy"}
	}
	return protocol.RegisteredNode{
		ID:        r.manifest.Master.ID,
		Role:      "master",
		Region:    r.manifest.Master.Region,
		PublicURL: r.manifest.Master.SiteBaseURL,
		Status:    "healthy",
	}
}

func (r *Registry) computeHealth(node protocol.RegisteredNode) protocol.RegisteredNode {
	if node.ID == "" {
		return node
	}
	if time.Since(node.LastSeenAt) > r.healthTTL {
		if !strings.EqualFold(node.Status, "unhealthy") {
			node.Status = "stale"
		}
	}
	if r.manifest != nil && strings.TrimSpace(r.manifest.ContentHash) != "" && strings.TrimSpace(node.ContentHash) != "" {
		if strings.TrimSpace(node.ContentHash) != strings.TrimSpace(r.manifest.ContentHash) {
			if !strings.EqualFold(node.Status, "unhealthy") {
				node.Status = "degraded"
			}
		}
	}
	return node
}

func normalizeNodeStatus(next, current string) string {
	next = strings.TrimSpace(strings.ToLower(next))
	if next != "" {
		return next
	}
	current = strings.TrimSpace(strings.ToLower(current))
	if current != "" {
		return current
	}
	return "healthy"
}

func chooseString(next, current string) string {
	if strings.TrimSpace(next) != "" {
		return next
	}
	return current
}

func chooseInt(next, current int) int {
	if next > 0 {
		return next
	}
	return current
}

func cloneManifest(manifest *protocol.Manifest) *protocol.Manifest {
	if manifest == nil {
		return nil
	}
	cloned := *manifest
	if manifest.Nodes != nil {
		cloned.Nodes = append([]protocol.RegisteredNode(nil), manifest.Nodes...)
	}
	return &cloned
}
