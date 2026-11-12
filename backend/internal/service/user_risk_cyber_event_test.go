//go:build unit

package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type cyberRiskFreshSettings struct {
	SettingRepository
	values map[string]string
	err    error
	calls  int
}

func (s *cyberRiskFreshSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	s.calls++
	return s.values, s.err
}

type cyberRiskEventStore struct {
	UserRiskPolicyRepository
	inputs   []UserRiskEventInput
	err      error
	detached bool
}

func (r *cyberRiskEventStore) RecordUserRiskEvent(ctx context.Context, input UserRiskEventInput) (*UserRiskDecision, error) {
	r.inputs = append(r.inputs, input)
	_, bounded := ctx.Deadline()
	r.detached = ctx.Err() == nil && bounded
	if r.err != nil {
		return nil, r.err
	}
	result := &UserRiskDecision{UserID: input.Admission.UserID, EventID: input.Admission.EventID,
		PolicyKey: input.Admission.PolicyKey, Scope: input.Policy.Scope, Generation: input.Admission.Generation,
		PolicyRevision: input.Policy.Revision, HitCount: 1, IgnoredReason: input.ExemptReason}
	if input.ExemptReason != "" {
		result.HitCount = 0
	}
	return result, nil
}
func cyberRiskServiceFixture(t *testing.T, cfg *ContentModerationConfig) (*ContentModerationService, *cyberRiskFreshSettings, *cyberRiskEventStore, UserRiskAdmission) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	settings := &cyberRiskFreshSettings{values: map[string]string{
		SettingKeyRiskControlEnabled: "true", SettingKeyContentModerationConfig: string(raw), SettingKeyCyberPolicyUserAllowlist: "",
	}}
	repo := &cyberRiskEventStore{}
	admission := UserRiskAdmission{UserID: 17, EventID: "server-event", PolicyKey: UserRiskPolicyCyber,
		Scope: UserRiskScopeAPI, Generation: 2, AdmittedAt: time.Unix(10, 0)}
	return &ContentModerationService{settingRepo: settings, userRiskRepo: repo}, settings, repo, admission
}

func TestApplyCyberUserPolicyFreezesMinimalSnapshotAndIgnoresStalePrivilegeHints(t *testing.T) {
	cfg := defaultContentModerationConfig()
	cfg.Revision, cfg.APIKey = 12, "sensitive-test-key"
	svc, settings, repo, admission := cyberRiskServiceFixture(t, cfg)
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := svc.ApplyCyberPolicyUserEvent(cancelled, CyberPolicyRecordInput{UserID: 17, Model: "gpt-5", LogOnly: true}, admission)
	if err != nil {
		t.Fatal(err)
	}
	if result.HitCount != 1 || len(repo.inputs) != 1 || repo.inputs[0].ExemptReason != "" || !repo.detached {
		t.Fatal("stale privilege hint, client cancellation or unbounded work changed durable enforcement")
	}
	if settings.calls != 1 || repo.inputs[0].Policy.Revision != 12 {
		t.Fatal("policy was not read authoritatively")
	}
	raw, err := json.Marshal(repo.inputs[0].Policy)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "sensitive-test-key") || strings.Contains(string(raw), "api_key") {
		t.Fatal("the policy snapshot included unrelated credentials")
	}
	settings.values[SettingKeyCyberPolicyUserAllowlist] = "17"
	result, err = svc.ApplyCyberPolicyUserEvent(context.Background(), CyberPolicyRecordInput{UserID: 17, Model: "gpt-5"}, admission)
	if err != nil {
		t.Fatal(err)
	}
	if result.HitCount != 0 || repo.inputs[1].ExemptReason != "allowlisted" {
		t.Fatal("a fresh authoritative exemption was ignored")
	}
}

func TestApplyCyberUserPolicyDisabledOrOutsideScopeDoesNotUseLegacyPunishment(t *testing.T) {
	for _, kind := range []string{"master", "policy", "group", "model"} {
		t.Run(kind, func(t *testing.T) {
			cfg := defaultContentModerationConfig()
			if kind == "policy" {
				cfg.CyberPolicyUserRestriction.Enabled = false
			}
			if kind == "group" {
				cfg.AllGroups = false
				cfg.GroupIDs = []int64{99}
			}
			if kind == "model" {
				cfg.ModelFilter = ContentModerationModelFilter{Type: "include", Models: []string{"allowed"}}
			}
			svc, settings, repo, admission := cyberRiskServiceFixture(t, cfg)
			if kind == "master" {
				settings.values[SettingKeyRiskControlEnabled] = "false"
			}
			result, err := svc.ApplyCyberPolicyUserEvent(context.Background(), CyberPolicyRecordInput{UserID: 17, Model: "gpt-5"}, admission)
			if err != nil || result != nil || len(repo.inputs) != 0 {
				t.Fatal("disabled or out-of-scope policy attempted punishment")
			}
		})
	}
}

func TestApplyCyberUserPolicyRejectsMissingAdmissionAndUnavailablePersistence(t *testing.T) {
	svc, settings, repo, admission := cyberRiskServiceFixture(t, defaultContentModerationConfig())
	_, err := svc.ApplyCyberPolicyUserEvent(context.Background(), CyberPolicyRecordInput{UserID: 17, Model: "gpt-5"}, UserRiskAdmission{})
	if err == nil || infraerrors.Code(err) != 503 || len(repo.inputs) != 0 {
		t.Fatal("missing admission was silently accepted")
	}
	repo.err = errors.New("database unavailable")
	_, err = svc.ApplyCyberPolicyUserEvent(context.Background(), CyberPolicyRecordInput{UserID: 17, Model: "gpt-5"}, admission)
	if err == nil || infraerrors.Code(err) != 503 || len(repo.inputs) != 3 || settings.calls != 2 {
		t.Fatal("persistence failure was not bounded or policy was reevaluated across retries")
	}
}
