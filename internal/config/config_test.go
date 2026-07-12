package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Harness.BaseURL != "https://app.harness.io" {
		t.Errorf("expected base_url https://app.harness.io, got %s", cfg.Harness.BaseURL)
	}
	if cfg.Harness.Auth.APIKeyEnv != "HARNESS_API_KEY" {
		t.Errorf("expected api_key_env HARNESS_API_KEY, got %s", cfg.Harness.Auth.APIKeyEnv)
	}
	if cfg.Signals.Chaos.CoverageFreshnessDays != 90 {
		t.Errorf("expected 90 chaos freshness days, got %d", cfg.Signals.Chaos.CoverageFreshnessDays)
	}
	if cfg.Signals.SRM.MinHealthyBudgetPct != 25.0 {
		t.Errorf("expected 25.0 healthy budget pct, got %.1f", cfg.Signals.SRM.MinHealthyBudgetPct)
	}
	if cfg.Signals.SRM.MinMarginalBudgetPct != 10.0 {
		t.Errorf("expected 10.0 marginal budget pct, got %.1f", cfg.Signals.SRM.MinMarginalBudgetPct)
	}
	if !cfg.Signals.STO.BlockOnOpenCritical {
		t.Error("expected block_on_open_critical to be true by default")
	}
	if cfg.Policy.Mode != "audit" {
		t.Errorf("expected policy mode 'audit', got %s", cfg.Policy.Mode)
	}
	if !cfg.Remediation.AutoGenerate {
		t.Error("expected auto_generate to be true by default")
	}
	if !cfg.Dashboard.Enabled {
		t.Error("expected dashboard to be enabled by default")
	}
	if cfg.BlastRadius.StructuralFallback.MaxHops != 2 {
		t.Errorf("expected max_hops 2, got %d", cfg.BlastRadius.StructuralFallback.MaxHops)
	}
}

func TestLoadConfig_FromYAML(t *testing.T) {
	yamlContent := `
harness:
  base_url: https://custom.harness.io
  account_id: test-account
signals:
  chaos:
    coverage_freshness_days: 60
  srm:
    min_healthy_budget_pct: 30
    min_marginal_budget_pct: 12
policy:
  mode: enforce
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".rolloutguardian.yaml")
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Harness.BaseURL != "https://custom.harness.io" {
		t.Errorf("expected custom base_url, got %s", cfg.Harness.BaseURL)
	}
	if cfg.Harness.AccountID != "test-account" {
		t.Errorf("expected test-account, got %s", cfg.Harness.AccountID)
	}
	if cfg.Signals.Chaos.CoverageFreshnessDays != 60 {
		t.Errorf("expected 60 freshness days, got %d", cfg.Signals.Chaos.CoverageFreshnessDays)
	}
	if cfg.Signals.SRM.MinHealthyBudgetPct != 30.0 {
		t.Errorf("expected 30.0, got %.1f", cfg.Signals.SRM.MinHealthyBudgetPct)
	}
	if cfg.Signals.SRM.MinMarginalBudgetPct != 12.0 {
		t.Errorf("expected 12.0, got %.1f", cfg.Signals.SRM.MinMarginalBudgetPct)
	}
	if cfg.Policy.Mode != "enforce" {
		t.Errorf("expected policy mode enforce, got %s", cfg.Policy.Mode)
	}
}

func TestLoadConfig_MissingFile_ReturnsDefaults(t *testing.T) {
	// When no config file exists and path is default, should return defaults without error
	cfg, err := LoadConfig("/nonexistent/path/.rolloutguardian.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing non-default config, got: %v", err)
	}

	if cfg.Harness.BaseURL != "https://app.harness.io" {
		t.Errorf("expected default base_url, got %s", cfg.Harness.BaseURL)
	}
}

func TestLoadConfig_EnvVarOverrides(t *testing.T) {
	t.Setenv("HARNESS_ACCOUNT_ID", "env-account-override")

	// Need a valid config file so LoadConfig reaches the env-var override path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".rolloutguardian.yaml")
	if err := os.WriteFile(configPath, []byte("harness:\n  account_id: original\n"), 0644); err != nil {
		t.Fatalf("failed writing test config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Harness.AccountID != "env-account-override" {
		t.Errorf("expected env override for account_id, got %s", cfg.Harness.AccountID)
	}
}
