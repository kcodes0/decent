package discovery

import (
	"net"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/kcodes0/decent/internal/protocol"
)

const (
	ActionRedirect = "redirect"
	ActionFallback = "fallback"
)

type LocationHint struct {
	Region   string
	ClientIP string
	Host     string
}

type Decision struct {
	Action string                   `json:"action"`
	Target string                   `json:"target,omitempty"`
	Node   *protocol.RegisteredNode `json:"node,omitempty"`
	Reason string                   `json:"reason,omitempty"`
}

type Selector struct {
	MasterRegion string
}

func NewSelector(masterRegion string) *Selector {
	return &Selector{MasterRegion: strings.TrimSpace(masterRegion)}
}

func (s *Selector) Resolve(hint LocationHint, nodes []protocol.RegisteredNode) Decision {
	healthy := healthyNodes(nodes)
	if len(healthy) == 0 {
		return Decision{Action: ActionFallback, Reason: "no healthy worker nodes"}
	}

	sort.SliceStable(healthy, func(i, j int) bool {
		return scoreNode(hint, s.MasterRegion, healthy[i]) > scoreNode(hint, s.MasterRegion, healthy[j])
	})

	chosen := healthy[0]
	target := chosen.PublicURL
	if target == "" {
		return Decision{Action: ActionFallback, Reason: "selected node has no public url"}
	}

	return Decision{
		Action: ActionRedirect,
		Target: target,
		Node:   &chosen,
		Reason: "selected nearest healthy worker",
	}
}

func (s *Selector) RegionHint(req *http.Request) LocationHint {
	return LocationHint{
		Region:   firstNonEmpty(req.URL.Query().Get("region"), req.Header.Get("X-Decent-Region"), req.Header.Get("X-Region"), req.Header.Get("CF-IPCountry")),
		ClientIP: clientIP(req),
		Host:     req.Host,
	}
}

func healthyNodes(nodes []protocol.RegisteredNode) []protocol.RegisteredNode {
	out := make([]protocol.RegisteredNode, 0, len(nodes))
	for _, node := range nodes {
		if strings.EqualFold(node.Status, "healthy") || strings.EqualFold(node.Status, "ready") {
			out = append(out, node)
		}
	}
	return out
}

func scoreNode(hint LocationHint, masterRegion string, node protocol.RegisteredNode) int {
	score := 0
	nodeRegion := strings.TrimSpace(node.Region)
	clientRegion := strings.TrimSpace(hint.Region)
	masterRegion = strings.TrimSpace(masterRegion)

	if clientRegion != "" {
		if sameRegion(clientRegion, nodeRegion) {
			score += 10_000
		} else if sharedPrefix(clientRegion, nodeRegion) {
			score += 7_500
		}
	}

	if masterRegion != "" && sameRegion(masterRegion, nodeRegion) {
		score += 1_000
	}

	latency := node.LatencyMillis
	if latency <= 0 {
		latency = 250
	}
	score -= int(latency)

	age := time.Since(node.LastSeenAt)
	if node.LastSeenAt.IsZero() {
		score -= 500
	} else {
		score -= int(age.Minutes())
	}

	if node.ConsecutiveHashFailures > 0 {
		score -= node.ConsecutiveHashFailures * 100
	}
	if node.ConsecutiveHeartbeatFails > 0 {
		score -= node.ConsecutiveHeartbeatFails * 50
	}

	return score
}

func sameRegion(a, b string) bool {
	a = normalizeRegion(a)
	b = normalizeRegion(b)
	return a != "" && a == b
}

func sharedPrefix(a, b string) bool {
	aParts := strings.FieldsFunc(normalizeRegion(a), splitRegion)
	bParts := strings.FieldsFunc(normalizeRegion(b), splitRegion)
	if len(aParts) == 0 || len(bParts) == 0 {
		return false
	}
	if len(aParts) < len(bParts) {
		for i := range aParts {
			if aParts[i] != bParts[i] {
				return false
			}
		}
		return true
	}
	for i := range bParts {
		if aParts[i] != bParts[i] {
			return false
		}
	}
	return true
}

func splitRegion(r rune) bool {
	switch r {
	case '-', '_', '/', ':':
		return true
	default:
		return false
	}
}

func normalizeRegion(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func clientIP(req *http.Request) string {
	if xff := req.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	host, _, err := net.SplitHostPort(req.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(req.RemoteAddr)
}
