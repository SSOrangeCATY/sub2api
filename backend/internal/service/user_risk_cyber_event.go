package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// ApplyCyberPolicyUserEvent is the durable observation path. The gateway must
// await it before admitting another logical turn; mail and supplementary logs
// are separate. One authoritative policy snapshot is frozen across retries.
func (s *ContentModerationService) ApplyCyberPolicyUserEvent(parent context.Context, in CyberPolicyRecordInput, admission UserRiskAdmission) (*UserRiskDecision, error) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), 5*time.Second)
	defer cancel()
	unavailable := func(reason string) error {
		return infraerrors.ServiceUnavailable(reason, "用户限制状态暂不可用，请稍后重试。")
	}
	if s == nil || s.settingRepo == nil {
		return nil, unavailable("USER_RISK_CONFIG_UNAVAILABLE")
	}
	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyRiskControlEnabled, SettingKeyContentModerationConfig, SettingKeyCyberPolicyUserAllowlist,
	})
	if err != nil {
		return nil, unavailable("USER_RISK_CONFIG_UNAVAILABLE")
	}
	if values[SettingKeyRiskControlEnabled] != "true" {
		return nil, nil
	}
	cfg, err := parseContentModerationConfig(values[SettingKeyContentModerationConfig])
	if err != nil {
		return nil, unavailable("USER_RISK_CONFIG_UNAVAILABLE")
	}
	if !cfg.CyberPolicyUserRestriction.Enabled || !cfg.includesGroup(in.GroupID) || !cfg.includesModel(in.Model) {
		return nil, nil
	}
	if in.UserID <= 0 || admission.UserID != in.UserID || admission.PolicyKey != UserRiskPolicyCyber ||
		admission.Scope != UserRiskScopeAPI || admission.Generation <= 0 || admission.EventID == "" || admission.AdmittedAt.IsZero() {
		return nil, unavailable("INVALID_USER_RISK_ADMISSION")
	}
	allowlist, err := ParseCyberPolicyUserAllowlist(values[SettingKeyCyberPolicyUserAllowlist])
	if err != nil {
		return nil, unavailable("USER_RISK_CONFIG_UNAVAILABLE")
	}
	repo, err := s.userRiskRepository()
	if err != nil {
		return nil, unavailable("USER_RISK_REPOSITORY_UNAVAILABLE")
	}
	input := UserRiskEventInput{
		Admission: admission, RequestID: in.RequestID, GroupID: cloneInt64Ptr(in.GroupID), Model: in.Model, Endpoint: in.Endpoint,
		Policy: UserRiskPolicySnapshot{Revision: cfg.Revision, Scope: UserRiskScopeAPI,
			WindowDays: cfg.CyberPolicyUserRestriction.WindowDays,
			Actions:    append([]UserRiskActionRule(nil), cfg.CyberPolicyUserRestriction.Actions...)},
	}
	// The caller's cached LogOnly hint cannot preserve a removed exemption.
	// The repository independently rereads the platform role under the user lock.
	if _, allowed := allowlist[in.UserID]; allowed {
		input.ExemptReason = "allowlisted"
	}
	var lastErr error
	for attempt := range 3 {
		result, err := repo.RecordUserRiskEvent(ctx, input)
		if err == nil && result != nil {
			if result.NewlyApplied && !result.Duplicate && s.authCacheInvalidator != nil {
				s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, in.UserID)
			}
			return result, nil
		}
		if err == nil {
			err = fmt.Errorf("risk repository returned no decision")
		}
		lastErr = err
		if attempt == 2 || infraerrors.Code(err) < 500 || ctx.Err() != nil {
			break
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 50 * time.Millisecond)
		select {
		case <-ctx.Done():
			lastErr = ctx.Err()
		case <-timer.C:
		}
		timer.Stop()
		if ctx.Err() != nil {
			break
		}
	}
	slog.Error("user_risk.durable_observation_failed", "event_id", admission.EventID, "error", lastErr)
	return nil, unavailable("USER_RISK_PERSISTENCE_UNAVAILABLE")
}
