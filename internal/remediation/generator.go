package remediation

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
)

type ExperimentManifest struct {
	Experiment ExperimentSpec `yaml:"experiment" json:"experiment"`
}

type ExperimentSpec struct {
	Name             string       `yaml:"name" json:"name"`
	GeneratedBy      string       `yaml:"generated_by" json:"generated_by"`
	GeneratedForFlag string       `yaml:"generated_for_flag" json:"generated_for_flag"`
	InfrastructureID string       `yaml:"infrastructure_id" json:"infrastructure_id"`
	Target           TargetSpec   `yaml:"target" json:"target"`
	Faults           []FaultSpec  `yaml:"faults" json:"faults"`
}

type TargetSpec struct {
	Namespace     string `yaml:"namespace" json:"namespace"`
	LabelSelector string `yaml:"label_selector" json:"label_selector"`
}

type FaultSpec struct {
	Name     string       `yaml:"name" json:"name"`
	Weight   int          `yaml:"weight" json:"weight"`
	Tunables TunablesSpec `yaml:"tunables" json:"tunables"`
	Probes   []ProbeSpec  `yaml:"probes" json:"probes"`
}

type TunablesSpec struct {
	TotalChaosDurationSec int  `yaml:"total_chaos_duration_sec" json:"total_chaos_duration_sec"`
	ChaosIntervalSec      int  `yaml:"chaos_interval_sec" json:"chaos_interval_sec"`
	Force                 bool `yaml:"force" json:"force"`
}

type ProbeSpec struct {
	Name     string `yaml:"name" json:"name"`
	Type     string `yaml:"type" json:"type"`
	Mode     string `yaml:"mode" json:"mode"`
	Expected string `yaml:"expected" json:"expected"`
}

type Generator interface {
	GenerateManifest(ctx context.Context, res *policy.DecisionResult, catalog []harnessclient.CatalogService) (string, error)
}

type DefaultGenerator struct {
	cfg *config.RemediationConfig
}

func NewGenerator(cfg *config.RemediationConfig) *DefaultGenerator {
	return &DefaultGenerator{cfg: cfg}
}

func (g *DefaultGenerator) GenerateManifest(ctx context.Context, res *policy.DecisionResult, catalog []harnessclient.CatalogService) (string, error) {
	if res.Decision != "block" || !g.cfg.AutoGenerate {
		return "", nil
	}

	// Find the first service causing the block from Reasons or BlastRadius
	targetService := ""
	for _, r := range res.Reasons {
		parts := strings.SplitN(r, " ", 2)
		if len(parts) > 0 {
			targetService = parts[0]
			break
		}
	}
	if targetService == "" && len(res.BlastRadius) > 0 {
		targetService = res.BlastRadius[0].Service
	}

	// Lookup K8s info from catalog
	var k8sInfo *harnessclient.K8sTargetInfo
	for _, svc := range catalog {
		if svc.Name == targetService {
			k8sInfo = svc.K8sTarget
			break
		}
	}
	if k8sInfo == nil {
		k8sInfo = &harnessclient.K8sTargetInfo{
			Namespace:     strings.TrimSuffix(targetService, "-service"),
			LabelSelector: fmt.Sprintf("app=%s", targetService),
			ClusterID:     fmt.Sprintf("prod-%s-cluster", strings.TrimSuffix(targetService, "-service")),
		}
	}

	manifest := ExperimentManifest{
		Experiment: ExperimentSpec{
			Name:             fmt.Sprintf("%s-pod-delete-min", targetService),
			GeneratedBy:      "rolloutguardian",
			GeneratedForFlag: res.FlagKey,
			InfrastructureID: k8sInfo.ClusterID,
			Target: TargetSpec{
				Namespace:     k8sInfo.Namespace,
				LabelSelector: k8sInfo.LabelSelector,
			},
			Faults: []FaultSpec{
				{
					Name:   "pod-delete",
					Weight: 6,
					Tunables: TunablesSpec{
						TotalChaosDurationSec: 60,
						ChaosIntervalSec:      10,
						Force:                 false,
					},
					Probes: []ProbeSpec{
						{
							Name:     fmt.Sprintf("%s-latency-steady-state", strings.TrimSuffix(targetService, "-service")),
							Type:     "prometheus",
							Mode:     "Continuous",
							Expected: "p99 < 200ms",
						},
					},
				},
			},
		},
	}

	outputDir := g.cfg.OutputDir
	if outputDir == "" {
		outputDir = "examples/experiments/generated"
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating output dir %s: %w", outputDir, err)
	}

	filename := fmt.Sprintf("%s-pod-delete-min.yaml", targetService)
	fullPath := filepath.Join(outputDir, filename)

	data, err := yaml.Marshal(manifest)
	if err != nil {
		return "", fmt.Errorf("failed marshaling experiment manifest: %w", err)
	}

	if err := os.WriteFile(fullPath, data, 0644); err != nil {
		return "", fmt.Errorf("failed writing manifest %s: %w", fullPath, err)
	}

	// Normalize with forward slashes for clean output matching README
	cleanPath := filepath.ToSlash(fullPath)
	res.SuggestedRemediation = cleanPath
	return cleanPath, nil
}
