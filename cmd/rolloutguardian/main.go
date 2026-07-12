package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rolloutguardian/rolloutguardian/backtest"
	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/notifier"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
	"github.com/rolloutguardian/rolloutguardian/internal/remediation"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
	"github.com/rolloutguardian/rolloutguardian/internal/scorecard"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	subcommand := os.Args[1]

	switch subcommand {
	case "evaluate":
		runEvaluate(os.Args[2:])
	case "explain":
		runExplain(os.Args[2:])
	case "scorecard":
		runScorecard(os.Args[2:])
	case "backtest":
		runBacktest(os.Args[2:])
	default:
		fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subcommand)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage: rolloutguardian <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  evaluate   Evaluate a proposed feature flag rollout change against resilience gates")
	fmt.Println("  explain    Explain the reasoning trail and audit breakdown for a feature flag")
	fmt.Println("  scorecard  Generate a multi-service resilience readiness scorecard across the org")
	fmt.Println("  backtest   Replay scoring logic against historical rollout timestamps to prove impact")
}

func runEvaluate(args []string) {
	fs := flag.NewFlagSet("evaluate", flag.ExitOnError)
	flagKey := fs.String("flag", "", "Target feature flag key (required)")
	targetRollout := fs.Float64("target-rollout", 100.0, "Proposed target rollout percentage")
	fromRollout := fs.Float64("from-rollout", 25.0, "Current rollout percentage")
	dryRun := fs.Bool("dry-run", false, "Simulate evaluation without executing side effects")
	configPath := fs.String("config", "", "Path to configuration file (.rolloutguardian.yaml)")
	jsonOutput := fs.Bool("json", false, "Output structured JSON decision result")

	_ = fs.Parse(args)

	if *flagKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --flag is required")
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Use MockClient by default or HTTPClient if API key is real
	client := resolveClient(cfg)

	res := resolver.NewResolver(&cfg.BlastRadius, client)
	nodes, err := res.ResolveBlastRadius(ctx, *flagKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving blast radius: %v\n", err)
		os.Exit(1)
	}

	agg := aggregator.NewAggregator(&cfg.Signals, client)
	payload, err := agg.AggregateSignals(ctx, *flagKey, "increase_rollout", *fromRollout, *targetRollout, nodes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error aggregating signals: %v\n", err)
		os.Exit(1)
	}

	eng := policy.NewOPAEngine(&cfg.Policy)
	decisionRes, err := eng.Evaluate(ctx, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error evaluating policy: %v\n", err)
		os.Exit(1)
	}

	if decisionRes.Decision == "block" && cfg.Remediation.AutoGenerate && !*dryRun {
		gen := remediation.NewGenerator(&cfg.Remediation)
		_, err := gen.GenerateManifest(ctx, decisionRes, getMockServices(client))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed generating remediation manifest: %v\n", err)
		}
	} else if *dryRun && decisionRes.Decision == "block" {
		decisionRes.SuggestedRemediation = fmt.Sprintf("examples/experiments/generated/%s-pod-delete-min.yaml", getFirstTarget(decisionRes))
	}

	if !*dryRun {
		notif := notifier.NewNotifier(&cfg.Notifications)
		if err := notif.Notify(ctx, *flagKey, "increase_rollout", *fromRollout, *targetRollout, decisionRes); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed sending notification: %v\n", err)
		}
	}

	if *jsonOutput {
		out, _ := json.MarshalIndent(decisionRes, "", "  ")
		fmt.Println(string(out))
		return
	}

	// Print formatted human-readable output matching README Section 15
	fmt.Printf("\nBlast radius resolved: %d services (confidence >= 0.8)\n", len(decisionRes.BlastRadius))
	for _, s := range decisionRes.BlastRadius {
		status := "[ OK ]"
		chaosDesc := "fresh"
		if s.Chaos != nil && s.Chaos.DaysSinceLastResult > cfg.Signals.Chaos.CoverageFreshnessDays {
			status = "[FAIL]"
			chaosDesc = fmt.Sprintf("stale (%dd)", s.Chaos.DaysSinceLastResult)
		} else if s.Chaos != nil {
			chaosDesc = fmt.Sprintf("fresh (%dd)", s.Chaos.DaysSinceLastResult)
		}

		srmDesc := "N/A"
		if s.SRM != nil {
			srmDesc = fmt.Sprintf("%.1f%%", s.SRM.ErrorBudgetRemainingPct)
			if s.SRM.ErrorBudgetRemainingPct < cfg.Signals.SRM.MinMarginalBudgetPct {
				status = "[FAIL]"
				srmDesc += fmt.Sprintf("  (below %.0f%% marginal threshold)", cfg.Signals.SRM.MinMarginalBudgetPct)
			} else if s.SRM.ErrorBudgetRemainingPct < cfg.Signals.SRM.MinHealthyBudgetPct {
				if status != "[FAIL]" {
					status = "[WARN]"
				}
				srmDesc += fmt.Sprintf("  (below %.0f%% healthy threshold)", cfg.Signals.SRM.MinHealthyBudgetPct)
			}
		}

		fmt.Printf("  %-6s %-22s chaos: %-15s error budget: %s\n", status, s.Service, chaosDesc, srmDesc)
	}

	fmt.Printf("\nDecision: %s\n", strings.ToUpper(decisionRes.Decision))
	if len(decisionRes.Reasons) > 0 {
		fmt.Printf("Reason:   %s\n", strings.Join(decisionRes.Reasons, "\n          "))
	} else if decisionRes.Decision == "allow" {
		fmt.Println("Reason:   All services in blast radius satisfy chaos coverage and error budget thresholds.")
	}

	if decisionRes.SuggestedRemediation != "" {
		fmt.Println("\nSuggested next step:")
		fmt.Printf("  -> %s\n", decisionRes.SuggestedRemediation)
		fmt.Println("     (~10 minute pod-delete experiment scoped to exactly this blast radius)")
	}

	fmt.Printf("\nRun `rolloutguardian explain --flag %s` for the full signal breakdown.\n\n", *flagKey)
}

func runExplain(args []string) {
	fs := flag.NewFlagSet("explain", flag.ExitOnError)
	flagKey := fs.String("flag", "", "Target feature flag key (required)")
	configPath := fs.String("config", "", "Path to configuration file")

	_ = fs.Parse(args)
	if *flagKey == "" {
		fmt.Fprintln(os.Stderr, "Error: --flag is required")
		fs.Usage()
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	client := resolveClient(cfg)

	res := resolver.NewResolver(&cfg.BlastRadius, client)
	nodes, err := res.ResolveBlastRadius(ctx, *flagKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error resolving blast radius: %v\n", err)
		os.Exit(1)
	}

	agg := aggregator.NewAggregator(&cfg.Signals, client)
	payload, err := agg.AggregateSignals(ctx, *flagKey, "increase_rollout", 25, 50, nodes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error aggregating signals: %v\n", err)
		os.Exit(1)
	}

	// Determine owning service from resolved flag
	flagDetails, _ := client.GetFlagDetails(ctx, *flagKey)
	ownerService := "unknown"
	if flagDetails != nil && flagDetails.OwnerService != "" {
		ownerService = flagDetails.OwnerService
	}

	fmt.Printf("\n============================================================\n")
	fmt.Printf("RolloutGuardian Explain Report: %s\n", *flagKey)
	fmt.Printf("============================================================\n\n")
	fmt.Printf("Flag Key:        %s\n", *flagKey)
	fmt.Printf("Owning Service:  %s\n", ownerService)
	fmt.Printf("Resolved Blast Radius:\n")

	for _, n := range payload.BlastRadius {
		fmt.Printf("  - Service:          %s\n", n.Service)
		fmt.Printf("    Confidence:       %.2f (%s)\n", n.Confidence, n.DetectionMethod)
		if n.Chaos != nil {
			scoreStr := "nil"
			if n.Chaos.ResilienceScore != nil {
				scoreStr = fmt.Sprintf("%.2f", *n.Chaos.ResilienceScore)
			}
			fmt.Printf("    Chaos Coverage:   %d days old (Resilience Score: %s, Experiment: %s)\n", n.Chaos.DaysSinceLastResult, scoreStr, n.Chaos.LastExperimentID)
		}
		if n.SRM != nil {
			fmt.Printf("    Error Budget:     %.1f%% remaining (SLO: %s, 24h Burn Rate: %.1fx)\n", n.SRM.ErrorBudgetRemainingPct, n.SRM.SLO, n.SRM.BurnRate24H)
		}
		if n.STO != nil {
			fmt.Printf("    STO Findings:     %d critical, %d high\n", n.STO.OpenCritical, n.STO.OpenHigh)
		}
		fmt.Println()
	}

	eng := policy.NewOPAEngine(&cfg.Policy)
	decisionRes, err := eng.Evaluate(ctx, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error evaluating policy: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Computed Governance Decision: %s\n", strings.ToUpper(decisionRes.Decision))
	for _, r := range decisionRes.Reasons {
		fmt.Printf("  * %s\n", r)
	}
	fmt.Println()
}

func getMockServices(c harnessclient.Client) []harnessclient.CatalogService {
	if mc, ok := c.(*harnessclient.MockClient); ok {
		return mc.CatalogServices
	}
	return nil
}

func getFirstTarget(res *policy.DecisionResult) string {
	if len(res.BlastRadius) > 0 {
		return res.BlastRadius[0].Service
	}
	return "target-service"
}

func runScorecard(args []string) {
	fs := flag.NewFlagSet("scorecard", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	jsonOutput := fs.Bool("json", false, "Output structured JSON report")

	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	client := resolveClient(cfg)

	ctx := context.Background()
	gen := scorecard.NewGenerator(&cfg.Signals, client)
	report, err := gen.GenerateReport(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating scorecard report: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Printf("\n================================================================================\n")
	fmt.Printf("RolloutGuardian Resilience Readiness Scorecard\n")
	fmt.Printf("Generated: %s\n", report.GeneratedAt.Format(time.RFC1123))
	fmt.Printf("================================================================================\n")
	fmt.Printf("Summary: Grade A: %d | Grade B: %d | Grade C: %d | Grade F: %d\n", report.TotalGradeA, report.TotalGradeB, report.TotalGradeC, report.TotalGradeF)
	fmt.Printf("--------------------------------------------------------------------------------\n")
	fmt.Printf("%-22s %-18s %-7s %-11s %-13s %s\n", "SERVICE NAME", "OWNER TEAM", "GRADE", "CHAOS COV", "ERROR BUDGET", "STO")
	fmt.Printf("--------------------------------------------------------------------------------\n")

	for _, s := range report.Services {
		grade := fmt.Sprintf("[ %s ]", s.ReadinessGrade)
		chaosDesc := fmt.Sprintf("%dd stale", s.ChaosDaysStale)
		if s.ChaosDaysStale <= cfg.Signals.Chaos.CoverageFreshnessDays {
			chaosDesc = fmt.Sprintf("%dd fresh", s.ChaosDaysStale)
		}
		srmDesc := fmt.Sprintf("%.1f%%", s.ErrorBudgetRemaining)
		stoDesc := fmt.Sprintf("%d/%d", s.STOCritical, s.STOHigh)

		fmt.Printf("%-22s %-18s %-7s %-11s %-13s %s\n", s.ServiceName, s.OwnerTeam, grade, chaosDesc, srmDesc, stoDesc)
	}
	fmt.Printf("--------------------------------------------------------------------------------\n\n")
}

func runBacktest(args []string) {
	fs := flag.NewFlagSet("backtest", flag.ExitOnError)
	configPath := fs.String("config", "", "Path to configuration file")
	jsonOutput := fs.Bool("json", false, "Output structured JSON report")

	_ = fs.Parse(args)

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	report, err := backtest.RunSimulatedBacktest(ctx, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error running backtest: %v\n", err)
		os.Exit(1)
	}

	if *jsonOutput {
		out, _ := json.MarshalIndent(report, "", "  ")
		fmt.Println(string(out))
		return
	}

	fmt.Println(backtest.FormatSummary(report))
}

// resolveClient returns a MockClient for local testing or an HTTPClient when a real API key is set.
func resolveClient(cfg *config.Config) harnessclient.Client {
	apiKey := os.Getenv(cfg.Harness.Auth.APIKeyEnv)
	if apiKey != "" && apiKey != "mock-key" && !strings.HasPrefix(apiKey, "pat.") {
		return harnessclient.NewHTTPClient(cfg.Harness.BaseURL, cfg.Harness.AccountID, apiKey)
	}
	mockClient, err := harnessclient.NewMockClient("examples/catalog-fixtures/catalog.json", "examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating mock client: %v\n", err)
		os.Exit(1)
	}
	return mockClient
}
