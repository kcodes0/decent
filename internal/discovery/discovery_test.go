package discovery

import (
	"net/http"
	"testing"

	"github.com/kcodes0/decent/internal/protocol"
)

func TestResolvePrefersMatchingRegion(t *testing.T) {
	selector := NewSelector("us-east-1")
	decision := selector.Resolve(LocationHint{Region: "us-east-1"}, []protocol.RegisteredNode{
		{ID: "b", Region: "us-west-2", PublicURL: "https://west.example.com", Status: "healthy", LatencyMillis: 5},
		{ID: "a", Region: "us-east-1", PublicURL: "https://east.example.com", Status: "healthy", LatencyMillis: 50},
	})

	if decision.Action != ActionRedirect {
		t.Fatalf("expected redirect action, got %q", decision.Action)
	}
	if decision.Target != "https://east.example.com" {
		t.Fatalf("expected east node target, got %q", decision.Target)
	}
}

func TestResolveFallsBackWithoutHealthyNodes(t *testing.T) {
	selector := NewSelector("")
	decision := selector.Resolve(LocationHint{}, []protocol.RegisteredNode{
		{ID: "a", Region: "us-east-1", PublicURL: "https://east.example.com", Status: "stale"},
	})

	if decision.Action != ActionFallback {
		t.Fatalf("expected fallback action, got %q", decision.Action)
	}
}

func TestRegionHintUsesHeaders(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.com", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("X-Decent-Region", "eu-west-1")
	req.RemoteAddr = "127.0.0.1:12345"

	hint := NewSelector("").RegionHint(req)
	if hint.Region != "eu-west-1" {
		t.Fatalf("expected header region, got %q", hint.Region)
	}
}
