package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/tidwall/gjson"
)

// CyberPolicyAccountRecorder is implemented by the persistent account repository.
// Counting and disabling must be atomic across requests and gateway instances.
type CyberPolicyAccountRecorder interface {
	RecordCyberPolicyError(ctx context.Context, accountID int64, threshold int) (count int64, disabled bool, err error)
}

const CyberPolicyErrorCountExtraKey = "cyber_policy_error_count"

// IsCyberPolicyError examines error fields only: echoed prompts, successful
// output, and a generic mention of "cyber" are not evidence of a policy block.
// Callers must first establish that this is an upstream error response/event.
func IsCyberPolicyError(payload []byte) bool {
	matchMessage := func(message string) bool {
		message = strings.ToLower(strings.Join(strings.Fields(message), " "))
		return strings.Contains(message, "this content was flagged for possible cybersecurity risk") ||
			strings.Contains(message, "this request was flagged for possible cybersecurity risk")
	}
	if !gjson.ValidBytes(payload) {
		// A partially consumed JSON error may contain echoed request text.
		// Never reinterpret malformed/truncated structured data as plain text.
		trimmed := strings.TrimSpace(string(payload))
		if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
			return false
		}
		return matchMessage(string(payload))
	}
	root := gjson.ParseBytes(payload)
	for _, path := range []string{"error", "response.error"} {
		err := root.Get(path)
		if err.IsObject() {
			if cyberPolicyErrorFields(err, matchMessage) {
				return true
			}
		} else if err.Type == gjson.String && matchMessage(err.String()) {
			return true
		}
	}
	// Some compatible providers return a bare error or a string detail.
	return cyberPolicyErrorFields(root, matchMessage) ||
		(root.Get("detail").Type == gjson.String && matchMessage(root.Get("detail").String()))
}

func cyberPolicyErrorFields(value gjson.Result, matchMessage func(string) bool) bool {
	for _, field := range []string{"code", "type"} {
		if strings.EqualFold(strings.TrimSpace(value.Get(field).String()), "cyber_policy") {
			return true
		}
	}
	return matchMessage(value.Get("message").String())
}

// CyberPolicyAccountObservation counts at most once for one upstream attempt.
// A provider can send both error and response.failed for the same attempt.
// It is independent of the existing client response/moderation classifiers.
type CyberPolicyAccountObservation struct {
	once   sync.Once
	record func()
}

func NewCyberPolicyAccountObservation(ctx context.Context, repo AccountRepository, cfg *config.Config, accountID int64) *CyberPolicyAccountObservation {
	recorder, ok := repo.(CyberPolicyAccountRecorder)
	if !ok || accountID <= 0 || cfg == nil {
		return nil
	}
	return &CyberPolicyAccountObservation{record: func() {
		// Client disconnects must not discard an already observed policy error.
		stateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		count, disabled, err := recorder.RecordCyberPolicyError(stateCtx, accountID, cfg.Gateway.CyberPolicyDisableThreshold)
		if err != nil {
			slog.Warn("cyber_policy_account_record_failed", "account_id", accountID, "error", err)
			return
		}
		slog.Info("cyber_policy_account_recorded", "account_id", accountID, "count", count, "disabled", disabled)
	}}
}

func (o *CyberPolicyAccountObservation) ObserveError(payload []byte) {
	if o != nil && IsCyberPolicyError(payload) {
		o.once.Do(o.record)
	}
}

// ObserveEvent accepts only upstream failure envelopes, including failures
// carried over a successful HTTP/SSE/WebSocket connection.
func (o *CyberPolicyAccountObservation) ObserveEvent(payload []byte) {
	if o == nil || !gjson.ValidBytes(payload) {
		return
	}
	root := gjson.ParseBytes(payload)
	switch root.Get("type").String() {
	case "error", "response.failed", "cyber_policy":
		o.ObserveError(payload)
	default:
		if root.Get("status").String() == "failed" || root.Get("response.status").String() == "failed" {
			o.ObserveError(payload)
		} else if root.Get("type").String() == "" && root.Get("error").Exists() {
			// Chat Completions streams can emit a bare error envelope.
			o.ObserveError(payload)
		}
	}
}
