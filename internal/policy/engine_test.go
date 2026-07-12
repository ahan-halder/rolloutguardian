package policy

import (
	"context"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
)

func TestEngineEvaluate_Block(t *testing.T) {
	cfg := &config.PolicyConfig{Bundle: "../../policies/rolloutguardian/authz.rego"}
	engine := NewOPAEngine(cfg)

	score := 0.50
	payload := &aggregator.EvaluationPayload{
		FlagKey:         "checkout-v2-express-pay",
		RequestedChange: "increase_rollout",
		FromPct:         25,
		ToPct:           50,
		SignalsConfig:   config.SignalsConfig{STO: config.STOSignalsConfig{BlockOnOpenCritical: true}},
		BlastRadius: []aggregator.AggregatedServiceSignal{
			{
				Service: "payment-service",
				Name:    "payment-service",
				Chaos: &harnessclient.ChaosCoverageSummary{
					DaysSinceLastResult: 118,
					ResilienceScore:     &score,
				},
				SRM: &harnessclient.SRMSummary{
					ErrorBudgetRemainingPct: 8.5,
				},
				STO: &harnessclient.STOSummary{
					OpenCritical: 0,
				},
			},
		},
	}

	res, err := engine.Evaluate(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}

	if res.Decision != "block" {
		t.Errorf("expected decision 'block', got %q", res.Decision)
	}
	if len(res.Reasons) == 0 {
		t.Errorf("expected at least one block reason, got empty")
	}
}

func TestEngineEvaluate_Allow(t *testing.T) {
	cfg := &config.PolicyConfig{Bundle: "../../policies/rolloutguardian/authz.rego"}
	engine := NewOPAEngine(cfg)

	score := 0.95
	payload := &aggregator.EvaluationPayload{
		FlagKey:         "checkout-v2-express-pay",
		RequestedChange: "increase_rollout",
		FromPct:         25,
		ToPct:           50,
		SignalsConfig:   config.SignalsConfig{STO: config.STOSignalsConfig{BlockOnOpenCritical: true}},
		BlastRadius: []aggregator.AggregatedServiceSignal{
			{
				Service: "fraud-check-service",
				Name:    "fraud-check-service",
				Chaos: &harnessclient.ChaosCoverageSummary{
					DaysSinceLastResult: 14,
					ResilienceScore:     &score,
				},
				SRM: &harnessclient.SRMSummary{
					ErrorBudgetRemainingPct: 61.2,
				},
				STO: &harnessclient.STOSummary{
					OpenCritical: 0,
				},
			},
		},
	}

	res, err := engine.Evaluate(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}

	if res.Decision != "allow" {
		t.Errorf("expected decision 'allow', got %q", res.Decision)
	}
}

func TestEngineEvaluate_Warn(t *testing.T) {
	cfg := &config.PolicyConfig{Bundle: "../../policies/rolloutguardian/authz.rego"}
	engine := NewOPAEngine(cfg)

	score := 0.88
	payload := &aggregator.EvaluationPayload{
		FlagKey:         "checkout-v2-express-pay",
		RequestedChange: "increase_rollout",
		FromPct:         25,
		ToPct:           50,
		SignalsConfig:   config.SignalsConfig{STO: config.STOSignalsConfig{BlockOnOpenCritical: true}},
		BlastRadius: []aggregator.AggregatedServiceSignal{
			{
				Service: "checkout-service",
				Name:    "checkout-service",
				Chaos: &harnessclient.ChaosCoverageSummary{
					DaysSinceLastResult: 10,
					ResilienceScore:     &score,
				},
				SRM: &harnessclient.SRMSummary{
					ErrorBudgetRemainingPct: 18.5,
				},
				STO: &harnessclient.STOSummary{
					OpenCritical: 0,
				},
			},
		},
	}

	res, err := engine.Evaluate(context.Background(), payload)
	if err != nil {
		t.Fatalf("unexpected evaluate error: %v", err)
	}

	if res.Decision != "warn" {
		t.Errorf("expected decision 'warn', got %q", res.Decision)
	}
	if len(res.Reasons) == 0 {
		t.Errorf("expected at least one warn reason, got empty")
	}
}
