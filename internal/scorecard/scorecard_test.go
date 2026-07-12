package scorecard

import (
	"context"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
)

func TestScorecardGenerator_GenerateReport(t *testing.T) {
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

	gen := NewGenerator(cfg, mockClient)
	report, err := gen.GenerateReport(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(report.Services) != len(mockClient.CatalogServices) {
		t.Errorf("expected %d services in report, got %d", len(mockClient.CatalogServices), len(report.Services))
	}

	var paymentScorecard *ServiceScorecard
	for i := range report.Services {
		if report.Services[i].ServiceName == "payment-service" {
			paymentScorecard = &report.Services[i]
			break
		}
	}

	if paymentScorecard == nil {
		t.Fatalf("payment-service not found in report")
	}

	// payment-service has 118 days stale chaos (> 90) and 12.4% error budget (< 25% healthy but >= 10% marginal) -> Grade C
	if paymentScorecard.ReadinessGrade != "C" {
		t.Errorf("expected payment-service grade C, got %s (reasons: %v)", paymentScorecard.ReadinessGrade, paymentScorecard.Reasons)
	}
}
