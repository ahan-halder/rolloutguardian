package aggregator

import (
	"context"
	"fmt"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
)

type AggregatedServiceSignal struct {
	Service         string                              `json:"service"`
	Name            string                              `json:"name"` // duplicate for rego policy compatibility
	Confidence      float64                             `json:"confidence"`
	DetectionMethod string                              `json:"detection_method"`
	Chaos           *harnessclient.ChaosCoverageSummary `json:"chaos"`
	SRM             *harnessclient.SRMSummary           `json:"srm"`
	STO             *harnessclient.STOSummary           `json:"sto"`
}

type EvaluationPayload struct {
	FlagKey         string                    `json:"flag_key"`
	RequestedChange string                    `json:"requested_change"`
	FromPct         float64                   `json:"from_pct"`
	ToPct           float64                   `json:"to_pct"`
	BlastRadius     []AggregatedServiceSignal `json:"blast_radius"`
	SignalsConfig   config.SignalsConfig      `json:"signals_config"`
}

type Aggregator interface {
	AggregateSignals(ctx context.Context, flagKey, requestedChange string, fromPct, toPct float64, nodes []resolver.ResolvedServiceNode) (*EvaluationPayload, error)
}

type DefaultAggregator struct {
	cfg    *config.SignalsConfig
	client harnessclient.Client
}

func NewAggregator(cfg *config.SignalsConfig, client harnessclient.Client) *DefaultAggregator {
	return &DefaultAggregator{
		cfg:    cfg,
		client: client,
	}
}

func (a *DefaultAggregator) AggregateSignals(ctx context.Context, flagKey, requestedChange string, fromPct, toPct float64, nodes []resolver.ResolvedServiceNode) (*EvaluationPayload, error) {
	var aggregated []AggregatedServiceSignal

	for _, node := range nodes {
		chaos, err := a.client.GetChaosCoverage(ctx, node.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("failed fetching chaos coverage for %s: %w", node.ServiceName, err)
		}

		srm, err := a.client.GetSRMSummary(ctx, node.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("failed fetching srm summary for %s: %w", node.ServiceName, err)
		}

		sto, err := a.client.GetSTOSummary(ctx, node.ServiceName)
		if err != nil {
			return nil, fmt.Errorf("failed fetching sto summary for %s: %w", node.ServiceName, err)
		}

		aggregated = append(aggregated, AggregatedServiceSignal{
			Service:         node.ServiceName,
			Name:            node.ServiceName,
			Confidence:      node.Confidence,
			DetectionMethod: node.DetectionMethod,
			Chaos:           chaos,
			SRM:             srm,
			STO:             sto,
		})
	}

	return &EvaluationPayload{
		FlagKey:         flagKey,
		RequestedChange: requestedChange,
		FromPct:         fromPct,
		ToPct:           toPct,
		BlastRadius:     aggregated,
		SignalsConfig:   *a.cfg,
	}, nil
}
