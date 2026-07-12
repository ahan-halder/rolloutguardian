package aggregator

import (
	"context"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
)

func TestAggregateSignals_KnownServices(t *testing.T) {
	cfg := &config.SignalsConfig{
		Chaos: config.ChaosSignalsConfig{CoverageFreshnessDays: 90},
		SRM: config.SRMSignalsConfig{
			MinHealthyBudgetPct:  25.0,
			MinMarginalBudgetPct: 10.0,
		},
		STO: config.STOSignalsConfig{BlockOnOpenCritical: true},
	}

	mockClient, err := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		t.Fatalf("failed creating mock client: %v", err)
	}

	nodes := []resolver.ResolvedServiceNode{
		{ServiceName: "payment-service", Confidence: 0.94, DetectionMethod: "trace_association"},
		{ServiceName: "fraud-check-service", Confidence: 0.81, DetectionMethod: "static_callsite"},
	}

	agg := NewAggregator(cfg, mockClient)
	payload, err := agg.AggregateSignals(context.Background(), "checkout-v2-express-pay", "increase_rollout", 25, 50, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if payload.FlagKey != "checkout-v2-express-pay" {
		t.Errorf("expected flag_key checkout-v2-express-pay, got %s", payload.FlagKey)
	}
	if len(payload.BlastRadius) != 2 {
		t.Fatalf("expected 2 services in blast radius, got %d", len(payload.BlastRadius))
	}

	// Verify payment-service signals
	var paymentSignal *AggregatedServiceSignal
	for i := range payload.BlastRadius {
		if payload.BlastRadius[i].Service == "payment-service" {
			paymentSignal = &payload.BlastRadius[i]
			break
		}
	}

	if paymentSignal == nil {
		t.Fatal("payment-service not found in aggregated signals")
	}
	if paymentSignal.Chaos == nil {
		t.Fatal("expected chaos coverage for payment-service, got nil")
	}
	if paymentSignal.Chaos.DaysSinceLastResult != 118 {
		t.Errorf("expected 118 days since last chaos result, got %d", paymentSignal.Chaos.DaysSinceLastResult)
	}
	if paymentSignal.SRM == nil {
		t.Fatal("expected SRM summary for payment-service, got nil")
	}
	if paymentSignal.SRM.ErrorBudgetRemainingPct != 12.4 {
		t.Errorf("expected 12.4%% error budget, got %.1f%%", paymentSignal.SRM.ErrorBudgetRemainingPct)
	}
	if paymentSignal.STO == nil {
		t.Fatal("expected STO summary for payment-service, got nil")
	}
}

func TestAggregateSignals_EmptyBlastRadius(t *testing.T) {
	cfg := &config.SignalsConfig{
		Chaos: config.ChaosSignalsConfig{CoverageFreshnessDays: 90},
		SRM: config.SRMSignalsConfig{
			MinHealthyBudgetPct:  25.0,
			MinMarginalBudgetPct: 10.0,
		},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	agg := NewAggregator(cfg, mockClient)
	payload, err := agg.AggregateSignals(context.Background(), "test-flag", "increase_rollout", 0, 100, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil nodes: %v", err)
	}

	if len(payload.BlastRadius) != 0 {
		t.Errorf("expected empty blast radius, got %d", len(payload.BlastRadius))
	}
}

func TestAggregateSignals_UnknownService(t *testing.T) {
	cfg := &config.SignalsConfig{
		Chaos: config.ChaosSignalsConfig{CoverageFreshnessDays: 90},
		SRM: config.SRMSignalsConfig{
			MinHealthyBudgetPct:  25.0,
			MinMarginalBudgetPct: 10.0,
		},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	nodes := []resolver.ResolvedServiceNode{
		{ServiceName: "nonexistent-service", Confidence: 0.65, DetectionMethod: "structural_neighborhood"},
	}

	agg := NewAggregator(cfg, mockClient)
	payload, err := agg.AggregateSignals(context.Background(), "test-flag", "increase_rollout", 10, 50, nodes)
	if err != nil {
		t.Fatalf("unexpected error for unknown service: %v", err)
	}

	// MockClient returns sensible defaults for unknown services
	if len(payload.BlastRadius) != 1 {
		t.Fatalf("expected 1 service in blast radius, got %d", len(payload.BlastRadius))
	}

	if payload.BlastRadius[0].Chaos == nil {
		t.Error("expected default chaos coverage for unknown service")
	}
	if payload.BlastRadius[0].SRM == nil {
		t.Error("expected default SRM summary for unknown service")
	}
}

func TestAggregateSignals_NameField(t *testing.T) {
	cfg := &config.SignalsConfig{
		Chaos: config.ChaosSignalsConfig{CoverageFreshnessDays: 90},
		SRM: config.SRMSignalsConfig{
			MinHealthyBudgetPct:  25.0,
			MinMarginalBudgetPct: 10.0,
		},
	}

	mockClient, _ := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	nodes := []resolver.ResolvedServiceNode{
		{ServiceName: "fraud-check-service", Confidence: 0.81, DetectionMethod: "static_callsite"},
	}

	agg := NewAggregator(cfg, mockClient)
	payload, err := agg.AggregateSignals(context.Background(), "test-flag", "increase_rollout", 25, 50, nodes)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The `name` field must match `service` for Rego policy compatibility
	if payload.BlastRadius[0].Name != payload.BlastRadius[0].Service {
		t.Errorf("expected Name == Service for Rego compatibility, got Name=%q Service=%q",
			payload.BlastRadius[0].Name, payload.BlastRadius[0].Service)
	}
}
