package master

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/kcodes0/decent/internal/discovery"
	"github.com/kcodes0/decent/internal/protocol"
)

type Config struct {
	Manifest              *protocol.Manifest
	Registry              *Registry
	Selector              *discovery.Selector
	LocalFallback         http.Handler
	RedirectStatus        int
	RoutePath             string
	HealthTTL             time.Duration
	HashFailureLimit      int
	HeartbeatFailureLimit int
}

type Server struct {
	cfg      Config
	registry *Registry
	selector *discovery.Selector
	fallback http.Handler
}

func NewServer(cfg Config) *Server {
	reg := cfg.Registry
	if reg == nil {
		reg = NewRegistry()
	}
	if cfg.Manifest != nil {
		reg.SetManifest(cfg.Manifest)
	}
	if cfg.HealthTTL > 0 || cfg.HashFailureLimit > 0 || cfg.HeartbeatFailureLimit > 0 {
		reg.SetHealthPolicy(cfg.HealthTTL, cfg.HashFailureLimit, cfg.HeartbeatFailureLimit)
	}
	selector := cfg.Selector
	if selector == nil {
		selector = discovery.NewSelector(masterRegionFromManifest(cfg.Manifest))
	}
	fallback := cfg.LocalFallback
	if fallback == nil {
		fallback = http.HandlerFunc(defaultFallback)
	}
	if cfg.RedirectStatus == 0 {
		cfg.RedirectStatus = http.StatusFound
	}
	if cfg.RoutePath == "" {
		cfg.RoutePath = "/"
	}
	return &Server{
		cfg:      cfg,
		registry: reg,
		selector: selector,
		fallback: fallback,
	}
}

func (s *Server) Registry() *Registry {
	return s.registry
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch {
	case strings.HasPrefix(r.URL.Path, "/api/register"):
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.handleRegister(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/heartbeat"):
		if r.Method != http.MethodPost {
			methodNotAllowed(w)
			return
		}
		s.handleHeartbeat(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/status"):
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleStatus(w, r)
	case strings.HasPrefix(r.URL.Path, "/api/route"):
		if r.Method != http.MethodGet {
			methodNotAllowed(w)
			return
		}
		s.handleRouteDecision(w, r)
	default:
		s.handleRequestRouting(w, r)
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req protocol.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	node := s.registry.Register(req.Node)
	writeJSON(w, http.StatusOK, protocol.RegisterResponse{
		Accepted: true,
		Message:  fmt.Sprintf("node %s registered", node.ID),
	})
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req protocol.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	node := s.registry.Heartbeat(req)
	writeJSON(w, http.StatusOK, node)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	nodes, manifest := s.registry.Snapshot()
	writeJSON(w, http.StatusOK, protocol.StatusResponse{
		LocalNode:    s.registry.LocalNode(),
		Manifest:     manifest,
		KnownNodes:   nodes,
		HealthyNodes: s.registry.HealthyNodes(),
	})
}

func (s *Server) handleRouteDecision(w http.ResponseWriter, r *http.Request) {
	nodes := s.registry.HealthyNodes()
	decision := s.selector.Resolve(s.selector.RegionHint(r), nodes)
	writeJSON(w, http.StatusOK, decision)
}

func (s *Server) handleRequestRouting(w http.ResponseWriter, r *http.Request) {
	nodes, _ := s.registry.Snapshot()
	decision := s.selector.Resolve(s.selector.RegionHint(r), nodes)
	switch decision.Action {
	case discovery.ActionRedirect:
		http.Redirect(w, r, buildRedirectURL(decision.Target, r.URL), s.cfg.RedirectStatus)
	default:
		s.fallback.ServeHTTP(w, r)
	}
}

func buildRedirectURL(target string, u *url.URL) string {
	base, err := url.Parse(target)
	if err != nil {
		return target
	}
	joined := *base
	joined.Path = path.Join(strings.TrimRight(base.Path, "/"), u.Path)
	if strings.HasSuffix(u.Path, "/") && !strings.HasSuffix(joined.Path, "/") {
		joined.Path += "/"
	}
	joined.RawQuery = u.RawQuery
	joined.Fragment = u.Fragment
	return joined.String()
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func defaultFallback(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, "decent master fallback\npath=%s\nremote=%s\n", r.URL.Path, clientAddr(r.RemoteAddr))
}

func clientAddr(remote string) string {
	host, _, err := net.SplitHostPort(remote)
	if err == nil && host != "" {
		return host
	}
	return remote
}

func masterRegionFromManifest(manifest *protocol.Manifest) string {
	if manifest == nil {
		return ""
	}
	return manifest.Master.Region
}
