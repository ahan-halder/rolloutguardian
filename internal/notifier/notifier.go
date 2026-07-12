package notifier

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rolloutguardian/rolloutguardian/internal/config"
	"github.com/rolloutguardian/rolloutguardian/internal/policy"
)

type Notifier interface {
	Notify(ctx context.Context, flagKey string, reqChange string, fromPct float64, toPct float64, decision *policy.DecisionResult) error
}

type DefaultNotifier struct {
	cfg        *config.NotificationsConfig
	httpClient *http.Client
}

func NewNotifier(cfg *config.NotificationsConfig) Notifier {
	return &DefaultNotifier{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

type WebhookPayload struct {
	Event            string                 `json:"event"`
	Timestamp        string                 `json:"timestamp"`
	FlagKey          string                 `json:"flag_key"`
	RequestedChange  string                 `json:"requested_change"`
	FromPct          float64                `json:"from_pct"`
	ToPct            float64                `json:"to_pct"`
	Decision         *policy.DecisionResult `json:"decision"`
}

func (n *DefaultNotifier) Notify(ctx context.Context, flagKey string, reqChange string, fromPct float64, toPct float64, decision *policy.DecisionResult) error {
	var errs []string

	if n.cfg.Slack.Enabled && n.cfg.Slack.WebhookURL != "" {
		if err := n.sendSlack(ctx, flagKey, reqChange, fromPct, toPct, decision); err != nil {
			errs = append(errs, fmt.Sprintf("slack notification failed: %v", err))
		}
	}

	if n.cfg.Webhook.Enabled && n.cfg.Webhook.URL != "" {
		if err := n.sendWebhook(ctx, flagKey, reqChange, fromPct, toPct, decision); err != nil {
			errs = append(errs, fmt.Sprintf("webhook notification failed: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf(strings.Join(errs, "; "))
	}
	return nil
}

func (n *DefaultNotifier) sendSlack(ctx context.Context, flagKey string, reqChange string, fromPct float64, toPct float64, decision *policy.DecisionResult) error {
	emoji := ":white_check_mark:"
	color := "#36a64f" // green
	if decision.Decision == "block" {
		emoji = ":no_entry:"
		color = "#ff0000" // red
	} else if decision.Decision == "warn" {
		emoji = ":warning:"
		color = "#ffcc00" // yellow
	}

	title := fmt.Sprintf("%s *RolloutGuardian Decision: %s*", emoji, strings.ToUpper(decision.Decision))
	desc := fmt.Sprintf("Flag: `%s` (%s %.0f%% -> %.0f%%)\nResolved Blast Radius: `%d services`", flagKey, reqChange, fromPct, toPct, len(decision.BlastRadius))

	reasonsText := "All services in blast radius satisfy chaos coverage and error budget thresholds."
	if len(decision.Reasons) > 0 {
		reasonsText = strings.Join(decision.Reasons, "\n• ")
	}

	fields := []map[string]interface{}{
		{"title": "Flag", "value": flagKey, "short": true},
		{"title": "Decision", "value": strings.ToUpper(decision.Decision), "short": true},
	}

	if decision.SuggestedRemediation != "" {
		fields = append(fields, map[string]interface{}{
			"title": "Suggested Remediation",
			"value": fmt.Sprintf("Run `%s`", decision.SuggestedRemediation),
			"short": false,
		})
	}

	slackBody := map[string]interface{}{
		"channel": n.cfg.Slack.Channel,
		"text":    fmt.Sprintf("RolloutGuardian Decision: %s for %s", strings.ToUpper(decision.Decision), flagKey),
		"attachments": []map[string]interface{}{
			{
				"color": color,
				"title": title,
				"text":  fmt.Sprintf("%s\n\n*Reasons:*\n%s", desc, reasonsText),
				"fields": fields,
				"footer": "RolloutGuardian v1.0",
				"ts":     time.Now().Unix(),
			},
		},
	}

	data, err := json.Marshal(slackBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.Slack.WebhookURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code from slack: %d", resp.StatusCode)
	}
	return nil
}

func (n *DefaultNotifier) sendWebhook(ctx context.Context, flagKey string, reqChange string, fromPct float64, toPct float64, decision *policy.DecisionResult) error {
	payload := WebhookPayload{
		Event:           "rolloutguardian.decision.evaluated",
		Timestamp:       time.Now().UTC().Format(time.RFC3339),
		FlagKey:         flagKey,
		RequestedChange: reqChange,
		FromPct:         fromPct,
		ToPct:           toPct,
		Decision:        decision,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.Webhook.URL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-RolloutGuardian-Event", "rolloutguardian.decision.evaluated")

	if n.cfg.Webhook.Secret != "" {
		h := hmac.New(sha256.New, []byte(n.cfg.Webhook.Secret))
		h.Write(data)
		sig := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-RolloutGuardian-Signature", "sha256="+sig)
	}

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code from webhook: %d", resp.StatusCode)
	}
	return nil
}
