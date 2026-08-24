package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/dashboard"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/notifier"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
	"github.com/rolloutguardian/rolloutguardian/internal/remediation"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
	"github.com/rolloutguardian/rolloutguardian/internal/scorecard"
)

type EvaluateRequest struct {
	FlagKey          string   `json:"flag_key"`
	RequestedChange  string   `json:"requested_change"`
	FromPct          float64  `json:"from_pct"`
	ToPct            float64  `json:"to_pct"`
	TargetRolloutPct *float64 `json:"target_rollout_pct"`
}

type EvaluateResponse struct {
	RolloutGuardianDecision *policy.DecisionResult `json:"rolloutguardian_decision"`
}

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	port := flag.String("port", "8080", "Port to listen on")
	flag.Parse()

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed loading config: %v", err)
	}

	mockClient, err := harnessclient.NewMockClient("examples/catalog-fixtures/catalog.json", "examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		log.Fatalf("Failed creating client: %v", err)
	}

	res := resolver.NewResolver(&cfg.BlastRadius, mockClient)
	agg := aggregator.NewAggregator(&cfg.Signals, mockClient)
	eng := policy.NewOPAEngine(&cfg.Policy)
	gen := remediation.NewGenerator(&cfg.Remediation)
	scoreGen := scorecard.NewGenerator(&cfg.Signals, mockClient)
	notif := notifier.NewNotifier(&cfg.Notifications)

	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"rolloutguardian-server"}`))
	})

	http.HandleFunc("/evaluate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req EvaluateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Bad request JSON: %v", err), http.StatusBadRequest)
			return
		}

		if req.FlagKey == "" {
			http.Error(w, "flag_key is required", http.StatusBadRequest)
			return
		}
		if req.ToPct == 0 && req.TargetRolloutPct != nil {
			req.ToPct = *req.TargetRolloutPct
		}
		if req.RequestedChange == "" {
			req.RequestedChange = "increase_rollout"
		}

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		nodes, err := res.ResolveBlastRadius(ctx, req.FlagKey)
		if err != nil {
			http.Error(w, fmt.Sprintf("Resolution error: %v", err), http.StatusInternalServerError)
			return
		}

		payload, err := agg.AggregateSignals(ctx, req.FlagKey, req.RequestedChange, req.FromPct, req.ToPct, nodes)
		if err != nil {
			http.Error(w, fmt.Sprintf("Aggregation error: %v", err), http.StatusInternalServerError)
			return
		}

		decisionRes, err := eng.Evaluate(ctx, payload)
		if err != nil {
			http.Error(w, fmt.Sprintf("Policy evaluation error: %v", err), http.StatusInternalServerError)
			return
		}

		if decisionRes.Decision == "block" && cfg.Remediation.AutoGenerate {
			_, _ = gen.GenerateManifest(ctx, decisionRes, mockClient.CatalogServices)
		}

		_ = notif.Notify(ctx, req.FlagKey, req.RequestedChange, req.FromPct, req.ToPct, decisionRes)

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(EvaluateResponse{
			RolloutGuardianDecision: decisionRes,
		})
	})

	if cfg.Dashboard.Enabled {
		dashboard.RegisterRoutes(http.DefaultServeMux, cfg, mockClient, res, agg, eng, gen, scoreGen)
	}

	log.Printf("RolloutGuardian Gate Server listening on :%s (Mode: %s)", *port, cfg.Policy.Mode)
	if err := http.ListenAndServe(":"+*port, nil); err != nil {
		log.Fatalf("Server exited with error: %v", err)
	}
}
