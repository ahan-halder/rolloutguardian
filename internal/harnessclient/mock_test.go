package harnessclient

import (
	"context"
	"testing"
)

func TestMockClient_FixtureLoading(t *testing.T) {
	mc, err := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		t.Fatalf("failed creating mock client from fixtures: %v", err)
	}

	if len(mc.CatalogServices) == 0 {
		t.Fatal("expected catalog services to be loaded from fixture")
	}
	if len(mc.ResilienceMap) == 0 {
		t.Fatal("expected resilience map to be loaded from fixture")
	}
}

func TestMockClient_DefaultFallbacks(t *testing.T) {
	// When fixture files don't exist, should fall back to hardcoded defaults
	mc, err := NewMockClient("/nonexistent/catalog.json", "/nonexistent/chaos-map.json")
	if err != nil {
		t.Fatalf("unexpected error with missing fixtures: %v", err)
	}

	if len(mc.CatalogServices) == 0 {
		t.Fatal("expected default catalog services when fixtures are missing")
	}
	if len(mc.ResilienceMap) == 0 {
		t.Fatal("expected default resilience map when fixtures are missing")
	}
}

func TestMockClient_GetCatalogServices(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	services, err := mc.GetCatalogServices(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(services) < 3 {
		t.Errorf("expected at least 3 services, got %d", len(services))
	}

	// Verify checkout-service has expected dependencies
	for _, svc := range services {
		if svc.Name == "checkout-service" {
			if len(svc.Dependencies) != 2 {
				t.Errorf("expected 2 dependencies for checkout-service, got %d", len(svc.Dependencies))
			}
			return
		}
	}
	t.Error("checkout-service not found in catalog services")
}

func TestMockClient_GetChaosCoverage_KnownService(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	chaos, err := mc.GetChaosCoverage(context.Background(), "payment-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if chaos.DaysSinceLastResult != 118 {
		t.Errorf("expected 118 days since last result, got %d", chaos.DaysSinceLastResult)
	}
	if chaos.ResilienceScore != nil {
		t.Errorf("expected nil resilience score for payment-service, got %v", *chaos.ResilienceScore)
	}
}

func TestMockClient_GetChaosCoverage_UnknownService(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	chaos, err := mc.GetChaosCoverage(context.Background(), "unknown-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unknown services should get sensible defaults
	if chaos.DaysSinceLastResult != 30 {
		t.Errorf("expected default 30 days for unknown service, got %d", chaos.DaysSinceLastResult)
	}
	if chaos.ResilienceScore == nil {
		t.Fatal("expected non-nil default resilience score for unknown service")
	}
}

func TestMockClient_GetSRMSummary_KnownService(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	srm, err := mc.GetSRMSummary(context.Background(), "payment-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if srm.ErrorBudgetRemainingPct != 12.4 {
		t.Errorf("expected 12.4%% error budget, got %.1f%%", srm.ErrorBudgetRemainingPct)
	}
	if srm.SLO != "payment-service-availability" {
		t.Errorf("expected SLO payment-service-availability, got %s", srm.SLO)
	}
}

func TestMockClient_GetSTOSummary(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	sto, err := mc.GetSTOSummary(context.Background(), "payment-service")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sto.OpenCritical != 0 {
		t.Errorf("expected 0 open critical, got %d", sto.OpenCritical)
	}
	if sto.OpenHigh != 1 {
		t.Errorf("expected 1 open high, got %d", sto.OpenHigh)
	}
}

func TestMockClient_GetFlagDetails_KnownFlag(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	flag, err := mc.GetFlagDetails(context.Background(), "checkout-v2-express-pay")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if flag.Key != "checkout-v2-express-pay" {
		t.Errorf("expected key checkout-v2-express-pay, got %s", flag.Key)
	}
	if flag.OwnerService != "checkout-service" {
		t.Errorf("expected owner checkout-service, got %s", flag.OwnerService)
	}
}

func TestMockClient_GetFlagDetails_UnknownFlag(t *testing.T) {
	mc, _ := NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")

	flag, err := mc.GetFlagDetails(context.Background(), "nonexistent-flag")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Unknown flags should default to checkout-service as owner
	if flag.OwnerService != "checkout-service" {
		t.Errorf("expected fallback owner checkout-service, got %s", flag.OwnerService)
	}
}
