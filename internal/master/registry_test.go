package master

import (
	"testing"

	"github.com/kcodes0/decent/internal/protocol"
)

func TestHeartbeatMarksUnhealthyWhenReportedUnhealthy(t *testing.T) {
	reg := NewRegistry()
	node := reg.Register(protocol.RegisteredNode{ID: "node-1", Status: "healthy"})
	if node.Status != "healthy" {
		t.Fatalf("expected healthy registration, got %q", node.Status)
	}

	updated := reg.Heartbeat(protocol.HeartbeatRequest{
		NodeID:      "node-1",
		Healthy:     false,
		ContentHash: "bad",
	})
	if updated.Status == "healthy" {
		t.Fatalf("expected unhealthy status after bad heartbeat")
	}
}

func TestHeartbeatFlagsContentHashMismatch(t *testing.T) {
	reg := NewRegistry()
	reg.SetManifest(&protocol.Manifest{ContentHash: "expected"})
	updated := reg.Heartbeat(protocol.HeartbeatRequest{
		NodeID:      "node-1",
		Healthy:     true,
		ContentHash: "unexpected",
	})
	if updated.Status == "healthy" {
		t.Fatalf("expected degraded or unhealthy status on hash mismatch")
	}
}
