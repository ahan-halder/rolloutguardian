package dashboard

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/harnessclient"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
	"github.com/rolloutguardian/rolloutguardian/internal/remediation"
	"github.com/rolloutguardian/rolloutguardian/internal/resolver"
	"github.com/rolloutguardian/rolloutguardian/internal/scorecard"
)

func TestServerRoutes(t *testing.T) {
	cfg := config.DefaultConfig()
	mockClient, err := harnessclient.NewMockClient("../../examples/catalog-fixtures/catalog.json", "../../examples/catalog-fixtures/chaos-map.json")
	if err != nil {
		t.Fatalf("failed creating mock client: %v", err)
	}

	res := resolver.NewResolver(&cfg.BlastRadius, mockClient)
	agg := aggregator.NewAggregator(&cfg.Signals, mockClient)
	eng := policy.NewOPAEngine(&cfg.Policy)
	gen := remediation.NewGenerator(&cfg.Remediation)
	scoreGen := scorecard.NewGenerator(&cfg.Signals, mockClient)

	mux := http.NewServeMux()
	RegisterRoutes(mux, cfg, mockClient, res, agg, eng, gen, scoreGen)

	// Test GET /api/scorecard
	reqScorecard := httptest.NewRequest(http.MethodGet, "/api/scorecard", nil)
	recScorecard := httptest.NewRecorder()
	mux.ServeHTTP(recScorecard, reqScorecard)

	if recScorecard.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/scorecard, got %d", recScorecard.Code)
	}
	var report scorecard.ScorecardReport
	if err := json.Unmarshal(recScorecard.Body.Bytes(), &report); err != nil {
		t.Errorf("failed unmarshaling scorecard response: %v", err)
	}
	if len(report.Services) == 0 {
		t.Errorf("expected services in scorecard report, got 0")
	}

	// Test GET /api/flags
	reqFlags := httptest.NewRequest(http.MethodGet, "/api/flags", nil)
	recFlags := httptest.NewRecorder()
	mux.ServeHTTP(recFlags, reqFlags)

	if recFlags.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/flags, got %d", recFlags.Code)
	}

	// Test POST /api/simulate
	simPayload := []byte(`{"flag_key":"checkout-v2-express-pay","requested_change":"increase_rollout","from_pct":25,"to_pct":50}`)
	reqSim := httptest.NewRequest(http.MethodPost, "/api/simulate", bytes.NewReader(simPayload))
	recSim := httptest.NewRecorder()
	mux.ServeHTTP(recSim, reqSim)

	if recSim.Code != http.StatusOK {
		t.Errorf("expected 200 for /api/simulate, got %d (body: %s)", recSim.Code, recSim.Body.String())
	}
	var decisionRes policy.DecisionResult
	if err := json.Unmarshal(recSim.Body.Bytes(), &decisionRes); err != nil {
		t.Errorf("failed unmarshaling simulate response: %v", err)
	}
	if decisionRes.Decision != "block" && decisionRes.Decision != "warn" {
		t.Errorf("expected block or warn decision in simulation, got %s", decisionRes.Decision)
	}

	// Test GET / (HTML UI serving)
	reqUI := httptest.NewRequest(http.MethodGet, "/", nil)
	recUI := httptest.NewRecorder()
	mux.ServeHTTP(recUI, reqUI)

	if recUI.Code != http.StatusOK {
		t.Errorf("expected 200 for /, got %d", recUI.Code)
	}
	if !strings.Contains(recUI.Body.String(), "RolloutGuardian") {
		t.Errorf("expected HTML content containing RolloutGuardian, got: %s", recUI.Body.String()[:100])
	}
}
