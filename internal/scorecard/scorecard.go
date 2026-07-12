package scorecard

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
)

type ServiceScorecard struct {
	ServiceName          string   `json:"service_name"`
	OwnerTeam            string   `json:"owner_team"`
	ReadinessGrade       string   `json:"readiness_grade"` // A | B | C | F
	ChaosDaysStale       int      `json:"chaos_days_stale"`
	ChaosScore           *float64 `json:"chaos_score"`
	ErrorBudgetRemaining float64  `json:"error_budget_remaining"`
	BurnRate24H          float64  `json:"burn_rate_24h"`
	STOCritical          int      `json:"sto_critical"`
	STOHigh              int      `json:"sto_high"`
	FlagCount            int      `json:"flag_count"`
	Reasons              []string `json:"reasons"`
}

type ScorecardReport struct {
	GeneratedAt time.Time          `json:"generated_at"`
	TotalGradeA int                `json:"total_grade_a"`
	TotalGradeB int                `json:"total_grade_b"`
	TotalGradeC int                `json:"total_grade_c"`
	TotalGradeF int                `json:"total_grade_f"`
	Services    []ServiceScorecard `json:"services"`
}

type Generator struct {
	cfg    *config.SignalsConfig
	client harnessclient.Client
}

func NewGenerator(cfg *config.SignalsConfig, client harnessclient.Client) *Generator {
	return &Generator{
		cfg:    cfg,
		client: client,
	}
}

func (g *Generator) GenerateReport(ctx context.Context) (*ScorecardReport, error) {
	services, err := g.client.GetCatalogServices(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed fetching catalog services: %w", err)
	}

	report := &ScorecardReport{
		GeneratedAt: time.Now().UTC(),
		Services:    make([]ServiceScorecard, 0, len(services)),
	}

	for _, svc := range services {
		sc := ServiceScorecard{
			ServiceName: svc.Name,
			OwnerTeam:   svc.Owner,
			FlagCount:   len(svc.FlagKeys),
		}

		// Chaos signal
		chaos, err := g.client.GetChaosCoverage(ctx, svc.Name)
		if err == nil && chaos != nil {
			sc.ChaosDaysStale = chaos.DaysSinceLastResult
			sc.ChaosScore = chaos.ResilienceScore
		} else {
			sc.ChaosDaysStale = 999
		}

		// SRM signal
		srm, err := g.client.GetSRMSummary(ctx, svc.Name)
		if err == nil && srm != nil {
			sc.ErrorBudgetRemaining = srm.ErrorBudgetRemainingPct
			sc.BurnRate24H = srm.BurnRate24H
		}

		// STO signal
		sto, err := g.client.GetSTOSummary(ctx, svc.Name)
		if err == nil && sto != nil {
			sc.STOCritical = sto.OpenCritical
			sc.STOHigh = sto.OpenHigh
		}

		grade, reasons := g.computeGrade(sc)
		sc.ReadinessGrade = grade
		sc.Reasons = reasons

		switch grade {
		case "A":
			report.TotalGradeA++
		case "B":
			report.TotalGradeB++
		case "C":
			report.TotalGradeC++
		case "F":
			report.TotalGradeF++
		}

		report.Services = append(report.Services, sc)
	}

	sort.Slice(report.Services, func(i, j int) bool {
		gradeWeight := map[string]int{"F": 0, "C": 1, "B": 2, "A": 3}
		if gradeWeight[report.Services[i].ReadinessGrade] != gradeWeight[report.Services[j].ReadinessGrade] {
			return gradeWeight[report.Services[i].ReadinessGrade] < gradeWeight[report.Services[j].ReadinessGrade]
		}
		return report.Services[i].ServiceName < report.Services[j].ServiceName
	})

	return report, nil
}

func (g *Generator) computeGrade(sc ServiceScorecard) (string, []string) {
	var reasons []string
	isF := false
	isC := false
	isB := false

	if sc.STOCritical > 0 && g.cfg.STO.BlockOnOpenCritical {
		reasons = append(reasons, fmt.Sprintf("%d open critical STO security findings", sc.STOCritical))
		isF = true
	}

	if sc.ChaosDaysStale > g.cfg.Chaos.CoverageFreshnessDays {
		reasons = append(reasons, fmt.Sprintf("Chaos coverage stale (%d days > %d days threshold)", sc.ChaosDaysStale, g.cfg.Chaos.CoverageFreshnessDays))
		if sc.ErrorBudgetRemaining < g.cfg.SRM.MinMarginalBudgetPct {
			isF = true
		} else {
			isC = true
		}
	}

	if sc.ErrorBudgetRemaining < g.cfg.SRM.MinMarginalBudgetPct {
		reasons = append(reasons, fmt.Sprintf("Error budget remaining (%.1f%%) below marginal threshold (%.1f%%)", sc.ErrorBudgetRemaining, g.cfg.SRM.MinMarginalBudgetPct))
		isF = true
	} else if sc.ErrorBudgetRemaining < g.cfg.SRM.MinHealthyBudgetPct {
		reasons = append(reasons, fmt.Sprintf("Error budget remaining (%.1f%%) below healthy threshold (%.1f%%)", sc.ErrorBudgetRemaining, g.cfg.SRM.MinHealthyBudgetPct))
		isC = true
	}

	if isF {
		return "F", reasons
	}
	if isC {
		return "C", reasons
	}

	if sc.ChaosScore != nil && *sc.ChaosScore < 0.8 {
		reasons = append(reasons, fmt.Sprintf("Chaos resilience score (%.2f) below 0.8 target", *sc.ChaosScore))
		isB = true
	}
	if sc.STOHigh > 0 {
		reasons = append(reasons, fmt.Sprintf("%d open high STO security findings", sc.STOHigh))
		isB = true
	}

	if isB {
		return "B", reasons
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "Meets all resilience and reliability readiness thresholds")
	}
	return "A", reasons
}
