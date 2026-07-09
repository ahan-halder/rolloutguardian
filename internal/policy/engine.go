package policy

import (
	"context"
	"fmt"
	"os"

	"github.com/open-policy-agent/opa/rego"
	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
)

type DecisionResult struct {
	FlagKey              string                               `json:"flag_key"`
	RequestedChange      string                               `json:"requested_change"`
	FromPct              float64                              `json:"from_pct"`
	ToPct                float64                              `json:"to_pct"`
	BlastRadius          []aggregator.AggregatedServiceSignal `json:"blast_radius"`
	Decision             string                               `json:"decision"` // block | warn | allow
	Reasons              []string                             `json:"reasons"`
	SuggestedRemediation string                               `json:"suggested_remediation,omitempty"`
}

type Engine interface {
	Evaluate(ctx context.Context, payload *aggregator.EvaluationPayload) (*DecisionResult, error)
}

type OPAEngine struct {
	cfg *config.PolicyConfig
}

func NewOPAEngine(cfg *config.PolicyConfig) *OPAEngine {
	return &OPAEngine{cfg: cfg}
}

func (e *OPAEngine) Evaluate(ctx context.Context, payload *aggregator.EvaluationPayload) (*DecisionResult, error) {
	bundlePath := e.cfg.Bundle
	if bundlePath == "" {
		bundlePath = "policies/rolloutguardian/authz.rego"
	}

	content, err := loadPolicyFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read policy bundle %s: %w", bundlePath, err)
	}

	r := rego.New(
		rego.Query("data.rolloutguardian"),
		rego.Module(bundlePath, string(content)),
		rego.Input(payload),
	)

	rs, err := r.Eval(ctx)
	if err != nil {
		return nil, fmt.Errorf("rego evaluation failed: %w", err)
	}

	if len(rs) == 0 || len(rs[0].Expressions) == 0 {
		return nil, fmt.Errorf("no expressions returned by rego evaluation")
	}

	resMap, ok := rs[0].Expressions[0].Value.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected rego evaluation result format")
	}

	decision := "allow"
	if d, ok := resMap["decision"].(string); ok && d != "" {
		decision = d
	}

	var reasons []string
	if rList, ok := resMap["reasons"].([]interface{}); ok {
		for _, item := range rList {
			if str, ok := item.(string); ok {
				reasons = append(reasons, str)
			}
		}
	}

	return &DecisionResult{
		FlagKey:         payload.FlagKey,
		RequestedChange: payload.RequestedChange,
		FromPct:         payload.FromPct,
		ToPct:           payload.ToPct,
		BlastRadius:     payload.BlastRadius,
		Decision:        decision,
		Reasons:         reasons,
	}, nil
}

func loadPolicyFile(providedPath string) ([]byte, error) {
	candidates := []string{
		providedPath,
		"policies/rolloutguardian/authz.rego",
		"../../policies/rolloutguardian/authz.rego",
		"../../../policies/rolloutguardian/authz.rego",
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("policy file authz.rego not found in any standard candidate location")
}
