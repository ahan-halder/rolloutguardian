package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Harness       HarnessConfig       `yaml:"harness" json:"harness"`
	BlastRadius   BlastRadiusConfig   `yaml:"blast_radius" json:"blast_radius"`
	Signals       SignalsConfig       `yaml:"signals" json:"signals"`
	Policy        PolicyConfig        `yaml:"policy" json:"policy"`
	Remediation   RemediationConfig   `yaml:"remediation" json:"remediation"`
	Dashboard     DashboardConfig     `yaml:"dashboard" json:"dashboard"`
	Notifications NotificationsConfig `yaml:"notifications" json:"notifications"`
}

type DashboardConfig struct {
	Enabled   bool   `yaml:"enabled" json:"enabled"`
	Port      int    `yaml:"port" json:"port"`
	StaticDir string `yaml:"static_dir" json:"static_dir"`
}

type NotificationsConfig struct {
	Slack   SlackConfig   `yaml:"slack" json:"slack"`
	Webhook WebhookConfig `yaml:"webhook" json:"webhook"`
}

type SlackConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`
	WebhookURL string `yaml:"webhook_url" json:"webhook_url"`
	Channel    string `yaml:"channel" json:"channel"`
}

type WebhookConfig struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	URL     string `yaml:"url" json:"url"`
	Secret  string `yaml:"secret" json:"secret"`
}

type HarnessConfig struct {
	BaseURL   string     `yaml:"base_url" json:"base_url"`
	AccountID string     `yaml:"account_id" json:"account_id"`
	Auth      AuthConfig `yaml:"auth" json:"auth"`
}

type AuthConfig struct {
	APIKeyEnv string `yaml:"api_key_env" json:"api_key_env"`
}

type BlastRadiusConfig struct {
	StaticAnalysis     StaticAnalysisConfig     `yaml:"static_analysis" json:"static_analysis"`
	TraceMining        TraceMiningConfig        `yaml:"trace_mining" json:"trace_mining"`
	StructuralFallback StructuralFallbackConfig `yaml:"structural_fallback" json:"structural_fallback"`
}

type StaticAnalysisConfig struct {
	Languages []string `yaml:"languages" json:"languages"`
}

type TraceMiningConfig struct {
	Enabled           bool    `yaml:"enabled" json:"enabled"`
	Source            string  `yaml:"source" json:"source"`
	MinSamples        int     `yaml:"min_samples" json:"min_samples"`
	SignificanceAlpha float64 `yaml:"significance_alpha" json:"significance_alpha"`
}

type StructuralFallbackConfig struct {
	Source  string `yaml:"source" json:"source"`
	MaxHops int    `yaml:"max_hops" json:"max_hops"`
}

type SignalsConfig struct {
	Chaos ChaosSignalsConfig `yaml:"chaos" json:"chaos"`
	SRM   SRMSignalsConfig   `yaml:"srm" json:"srm"`
	STO   STOSignalsConfig   `yaml:"sto" json:"sto"`
}

type ChaosSignalsConfig struct {
	CoverageFreshnessDays int `yaml:"coverage_freshness_days" json:"coverage_freshness_days"`
}

type SRMSignalsConfig struct {
	MinHealthyBudgetPct  float64 `yaml:"min_healthy_budget_pct" json:"min_healthy_budget_pct"`
	MinMarginalBudgetPct float64 `yaml:"min_marginal_budget_pct" json:"min_marginal_budget_pct"`
}

type STOSignalsConfig struct {
	BlockOnOpenCritical bool `yaml:"block_on_open_critical" json:"block_on_open_critical"`
}

type PolicyConfig struct {
	Bundle string `yaml:"bundle" json:"bundle"`
	Mode   string `yaml:"mode" json:"mode"` // audit | enforce
}

type RemediationConfig struct {
	AutoGenerate bool   `yaml:"auto_generate" json:"auto_generate"`
	OutputDir    string `yaml:"output_dir" json:"output_dir"`
}

func DefaultConfig() *Config {
	return &Config{
		Harness: HarnessConfig{
			BaseURL:   "https://app.harness.io",
			AccountID: "mock-account",
			Auth: AuthConfig{
				APIKeyEnv: "HARNESS_API_KEY",
			},
		},
		BlastRadius: BlastRadiusConfig{
			StaticAnalysis: StaticAnalysisConfig{
				Languages: []string{"go", "typescript", "java", "python"},
			},
			TraceMining: TraceMiningConfig{
				Enabled:           true,
				Source:            "otlp_file",
				MinSamples:        30,
				SignificanceAlpha: 0.01,
			},
			StructuralFallback: StructuralFallbackConfig{
				Source:  "idp_catalog",
				MaxHops: 2,
			},
		},
		Signals: SignalsConfig{
			Chaos: ChaosSignalsConfig{
				CoverageFreshnessDays: 90,
			},
			SRM: SRMSignalsConfig{
				MinHealthyBudgetPct:  25.0,
				MinMarginalBudgetPct: 10.0,
			},
			STO: STOSignalsConfig{
				BlockOnOpenCritical: true,
			},
		},
		Policy: PolicyConfig{
			Bundle: "policies/rolloutguardian/authz.rego",
			Mode:   "audit",
		},
		Remediation: RemediationConfig{
			AutoGenerate: true,
			OutputDir:    "examples/experiments/generated",
		},
		Dashboard: DashboardConfig{
			Enabled:   true,
			Port:      8080,
			StaticDir: "",
		},
		Notifications: NotificationsConfig{
			Slack: SlackConfig{
				Enabled: false,
			},
			Webhook: WebhookConfig{
				Enabled: false,
			},
		},
	}
}

func LoadConfig(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = os.Getenv("ROLLOUTGUARDIAN_CONFIG")
	}
	if configPath == "" {
		configPath = ".rolloutguardian.yaml"
	}

	cfg := DefaultConfig()

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) && configPath == ".rolloutguardian.yaml" {
			// Try reading example if default doesn't exist
			data, err = os.ReadFile(".rolloutguardian.yaml.example")
			if err != nil {
				return cfg, nil // Return default if neither exists
			}
		} else {
			return cfg, nil
		}
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config %s: %w", configPath, err)
	}

	// Resolve environment variables inside fields if needed
	if accountID := os.Getenv("HARNESS_ACCOUNT_ID"); accountID != "" {
		cfg.Harness.AccountID = accountID
	}
	if slackWebhook := os.Getenv("SLACK_WEBHOOK_URL"); slackWebhook != "" {
		cfg.Notifications.Slack.WebhookURL = slackWebhook
		cfg.Notifications.Slack.Enabled = true
	}
	if webhookURL := os.Getenv("WEBHOOK_URL"); webhookURL != "" {
		cfg.Notifications.Webhook.URL = webhookURL
		cfg.Notifications.Webhook.Enabled = true
	}

	// Normalize bundle path relative to working directory if needed
	if !filepath.IsAbs(cfg.Policy.Bundle) {
		cfg.Policy.Bundle = filepath.Clean(cfg.Policy.Bundle)
	}

	return cfg, nil
}
