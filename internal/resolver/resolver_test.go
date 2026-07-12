package resolver

import (
	"context"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
)

func TestResolveBlastRadius_KnownFlag(t *testing.T) {
	cfg := &config.BlastRadiusConfig{
		StructuralFallback: config.StructuralFallbackConfig{MaxHops: 2},
	}

	mockClient, err := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		t.Fatalf("failed creating mock client: %v", err)
	}

	res := NewResolver(cfg, mockClient)
	nodes, err := res.ResolveBlastRadius(context.Background(), "checkout-v2-express-pay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(nodes) == 0 {
		t.Fatal("expected non-empty blast radius for known flag, got 0")
	}

	// checkout-service depends on payment-service and fraud-check-service (hop 1)
	// payment-service depends on ledger-service (hop 2)
	serviceNames := make(map[string]bool)
	for _, n := range nodes {
		serviceNames[n.ServiceName] = true
	}

	if !serviceNames["payment-service"] {
		t.Error("expected payment-service in blast radius")
	}
	if !serviceNames["fraud-check-service"] {
		t.Error("expected fraud-check-service in blast radius")
	}
	if !serviceNames["ledger-service"] {
		t.Error("expected ledger-service in blast radius (2nd hop via payment-service)")
	}
}

func TestResolveBlastRadius_ConfidenceScoring(t *testing.T) {
	cfg := &config.BlastRadiusConfig{
		StructuralFallback: config.StructuralFallbackConfig{MaxHops: 2},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	res := NewResolver(cfg, mockClient)
	nodes, err := res.ResolveBlastRadius(context.Background(), "checkout-v2-express-pay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, n := range nodes {
		if n.Confidence <= 0 || n.Confidence > 1.0 {
			t.Errorf("expected confidence in (0, 1.0] for %s, got %.2f", n.ServiceName, n.Confidence)
		}
		if n.DetectionMethod == "" {
			t.Errorf("expected non-empty detection_method for %s", n.ServiceName)
		}
	}
}

func TestResolveBlastRadius_SingleHop(t *testing.T) {
	cfg := &config.BlastRadiusConfig{
		StructuralFallback: config.StructuralFallbackConfig{MaxHops: 1},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	res := NewResolver(cfg, mockClient)
	nodes, err := res.ResolveBlastRadius(context.Background(), "checkout-v2-express-pay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With max_hops=1, only direct dependencies should appear (payment-service, fraud-check-service)
	// ledger-service is hop 2, so should NOT appear
	for _, n := range nodes {
		if n.ServiceName == "ledger-service" {
			t.Error("ledger-service should NOT be in blast radius with max_hops=1")
		}
	}
}

func TestResolveBlastRadius_UnknownFlag(t *testing.T) {
	cfg := &config.BlastRadiusConfig{
		StructuralFallback: config.StructuralFallbackConfig{MaxHops: 2},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	res := NewResolver(cfg, mockClient)

	// Unknown flag should fall back to checkout-service as default owner
	nodes, err := res.ResolveBlastRadius(context.Background(), "some-unknown-flag")
	if err != nil {
		t.Fatalf("unexpected error for unknown flag: %v", err)
	}

	if len(nodes) == 0 {
		t.Error("expected non-empty blast radius even for unknown flag (should use fallback owner)")
	}
}

func TestResolveBlastRadius_NoDuplicates(t *testing.T) {
	cfg := &config.BlastRadiusConfig{
		StructuralFallback: config.StructuralFallbackConfig{MaxHops: 2},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	res := NewResolver(cfg, mockClient)
	nodes, _ := res.ResolveBlastRadius(context.Background(), "checkout-v2-express-pay")

	seen := make(map[string]bool)
	for _, n := range nodes {
		if seen[n.ServiceName] {
			t.Errorf("duplicate service in blast radius: %s", n.ServiceName)
		}
		seen[n.ServiceName] = true
	}
}
