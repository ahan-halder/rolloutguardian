package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rolloutguardian/rolloutguardian/internal/aggregator"
	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
)

func TestNotifier_SlackAndWebhook(t *testing.T) {
	var slackBody map[string]interface{}
	var webhookPayload WebhookPayload
	var sigHeader string

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path == "/slack" {
			_ = json.Unmarshal(body, &slackBody)
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/webhook" {
			_ = json.Unmarshal(body, &webhookPayload)
			sigHeader = r.Header.Get("X-RolloutGuardian-Signature")
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	cfg := &config.NotificationsConfig{
		Slack: config.SlackConfig{
			Enabled:    true,
			WebhookURL: ts.URL + "/slack",
			Channel:    "#test-alerts",
		},
		Webhook: config.WebhookConfig{
			Enabled: true,
			URL:     ts.URL + "/webhook",
			Secret:  "mysecret",
		},
	}

	notif := NewNotifier(cfg)
	decision := &policy.DecisionResult{
		FlagKey:         "checkout-v2-express-pay",
		RequestedChange: "increase_rollout",
		FromPct:         25,
		ToPct:           50,
		Decision:        "block",
		Reasons:         []string{"payment-service stale chaos"},
		BlastRadius: []aggregator.AggregatedServiceSignal{
			{Service: "payment-service", Confidence: 0.94},
		},
		SuggestedRemediation: "payment-service-pod-delete-min.yaml",
	}

	err := notif.Notify(context.Background(), "checkout-v2-express-pay", "increase_rollout", 25, 50, decision)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if slackBody["channel"] != "#test-alerts" {
		t.Errorf("expected channel #test-alerts, got %v", slackBody["channel"])
	}

	if webhookPayload.FlagKey != "checkout-v2-express-pay" {
		t.Errorf("expected webhook flag_key checkout-v2-express-pay, got %s", webhookPayload.FlagKey)
	}
	if sigHeader == "" || len(sigHeader) <= 10 {
		t.Errorf("expected valid HMAC signature header, got %s", sigHeader)
	}
}
