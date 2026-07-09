package backtest

import (
	"context"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
)

func TestRunSimulatedBacktest(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Policy.Bundle = "../policies/rolloutguardian/authz.rego"

	report, err := RunSimulatedBacktest(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected backtest error: %v", err)
	}

	if report.TotalEventsAnalyzed != 142 {
		t.Errorf("expected 142 events, got %d", report.TotalEventsAnalyzed)
	}
	if report.FlaggedCorrelated != 7 {
		t.Errorf("expected 7 flagged correlated, got %d", report.FlaggedCorrelated)
	}

	summary := FormatSummary(report)
	if len(summary) == 0 {
		t.Errorf("expected non-empty formatted summary")
	}
}
