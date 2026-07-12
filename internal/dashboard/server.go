package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
	"github.com/rolloutguardian/rolloutguardian/internal/remediation"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
	"github.com/rolloutguardian/rolloutguardian/internal/scorecard"
)

type SimulateRequest struct {
	FlagKey         string  `json:"flag_key"`
	RequestedChange string  `json:"requested_change"`
	FromPct         float64 `json:"from_pct"`
	ToPct           float64 `json:"to_pct"`
}

type FlagSummary struct {
	Key          string   `json:"key"`
	Name         string   `json:"name"`
	OwnerService string   `json:"owner_service"`
	Dependencies []string `json:"dependencies"`
}

func RegisterRoutes(mux *http.ServeMux, cfg *config.Config, client harnessclient.Client, res resolver.Resolver, agg aggregator.Aggregator, eng policy.Engine, gen remediation.Generator, scoreGen *scorecard.Generator) {
	mux.HandleFunc("/api/scorecard", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		report, err := scoreGen.GenerateReport(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error generating scorecard: %v", err), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(report)
	})

	mux.HandleFunc("/api/flags", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		services, err := client.GetCatalogServices(ctx)
		if err != nil {
			http.Error(w, fmt.Sprintf("Error getting services: %v", err), http.StatusInternalServerError)
			return
		}

		var flags []FlagSummary
		for _, svc := range services {
			for _, fk := range svc.FlagKeys {
				flags = append(flags, FlagSummary{
					Key:          fk,
					Name:         fk,
					OwnerService: svc.Name,
					Dependencies: svc.Dependencies,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"flags": flags})
	})

	mux.HandleFunc("/api/simulate", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req SimulateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, fmt.Sprintf("Bad request JSON: %v", err), http.StatusBadRequest)
			return
		}

		if req.FlagKey == "" {
			req.FlagKey = "checkout-v2-express-pay"
		}
		if req.RequestedChange == "" {
			req.RequestedChange = "increase_rollout"
		}
		if req.ToPct == 0 {
			req.ToPct = 50
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
			if mc, ok := client.(*harnessclient.MockClient); ok {
				_, _ = gen.GenerateManifest(ctx, decisionRes, mc.CatalogServices)
			}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(decisionRes)
	})

	// Static UI serving
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		staticDir := cfg.Dashboard.StaticDir
		if staticDir == "" {
			staticDir = "internal/dashboard/web/dist"
		}

		targetPath := filepath.Join(staticDir, r.URL.Path)
		info, err := os.Stat(targetPath)
		if err != nil || info.IsDir() {
			// Check if index.html exists on disk
			indexPath := filepath.Join(staticDir, "index.html")
			if _, err := os.Stat(indexPath); err == nil {
				http.ServeFile(w, r, indexPath)
				return
			}
			// Fallback to embedded html dashboard if index.html not found on disk
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(embeddedHTMLDashboard))
			return
		}

		http.ServeFile(w, r, targetPath)
	})
}
