package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userRiskRestrictionState struct {
	exists                              bool
	generation, version, policyRevision int64
	hitCount, threshold                 int
	manual                              bool
	restrictedAt                        time.Time
	expires, cleared                    sql.NullTime
	scope, sourceEventID                string
}

func (s userRiskRestrictionState) active(now time.Time) bool {
	return s.exists && !s.cleared.Valid && (s.manual || (s.expires.Valid && s.expires.Time.After(now)))
}

// Audit display fields cannot prevent a real violation from being committed.
// Iterating runes replaces invalid UTF-8; PostgreSQL text also requires NUL
// removal. The server-generated event identity is never normalized.
func userRiskAuditText(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	var result strings.Builder
	result.Grow(min(len(value), limit*4))
	count := 0
	for _, char := range value {
		if char == 0 {
			continue
		}
		if count == limit {
			break
		}
		result.WriteRune(char)
		count++
	}
	return result.String()
}

func loadUserRiskRestriction(ctx context.Context, tx userRiskTransaction, userID int64, policyKey string) (userRiskRestrictionState, error) {
	var state userRiskRestrictionState
	err := scanSingleRow(ctx, tx,
		"SELECT generation,version,decision_hit_count,threshold,admin_release,restricted_at,expires_at,cleared_at,policy_revision,scope,source_event_id FROM user_risk_restrictions WHERE user_id=$1 AND policy_key=$2 FOR UPDATE",
		[]any{userID, policyKey}, &state.generation, &state.version, &state.hitCount, &state.threshold,
		&state.manual, &state.restrictedAt, &state.expires, &state.cleared, &state.policyRevision, &state.scope, &state.sourceEventID)
	if errors.Is(err, sql.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	state.exists = true
	return state, nil
}

func userRiskStateDecision(userID int64, policyKey string, state userRiskRestrictionState, now time.Time) service.UserRiskDecision {
	result := service.UserRiskDecision{
		UserID: userID, PolicyKey: policyKey, Scope: state.scope, Generation: state.generation,
		HitCount: state.hitCount, Threshold: state.threshold, Restricted: state.active(now),
		AdminRelease: state.manual, RestrictionVersion: state.version, PolicyRevision: state.policyRevision, RecordedAt: now,
	}
	if state.expires.Valid {
		value := state.expires.Time
		result.ExpiresAt = &value
	}
	return result
}

func insertUserRiskAction(ctx context.Context, tx userRiskTransaction, action, operationID, sourceEventID string, actorID any, reason string, decision service.UserRiskDecision, now time.Time) error {
	raw, err := json.Marshal(decision)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO user_risk_actions(user_id,policy_key,generation,action,operation_id,source_event_id,actor_id,reason,decision,created_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9::jsonb,$10)",
		decision.UserID, decision.PolicyKey, decision.Generation, action, operationID, sourceEventID, actorID, reason, string(raw), now)
	return err
}

// Called only after locking the user, cycle and restriction in that order.
func expireUserRiskState(ctx context.Context, tx userRiskTransaction, userID int64, policyKey string, state *userRiskRestrictionState, now time.Time) error {
	if !state.exists || state.cleared.Valid || state.manual || !state.expires.Valid || state.expires.Time.After(now) {
		return nil
	}
	changed, err := tx.ExecContext(ctx,
		"UPDATE user_risk_restrictions SET cleared_at=$3,version=version+1 WHERE user_id=$1 AND policy_key=$2 AND version=$4",
		userID, policyKey, now, state.version)
	if err != nil {
		return err
	}
	affected, err := changed.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("restriction changed during expiry")
	}
	state.version++
	state.cleared = sql.NullTime{Time: now, Valid: true}
	decision := userRiskStateDecision(userID, policyKey, *state, now)
	return insertUserRiskAction(ctx, tx, "expired", fmt.Sprintf("expire/%d/%d", userID, state.version),
		state.sourceEventID, nil, "automatic_expiry", decision, now)
}

func (r *contentModerationRepository) RecordUserRiskEvent(ctx context.Context, input service.UserRiskEventInput) (*service.UserRiskDecision, error) {
	return r.userRiskStore().recordEvent(ctx, input)
}

func (r *userRiskStore) recordEvent(ctx context.Context, input service.UserRiskEventInput) (*service.UserRiskDecision, error) {
	admission := input.Admission
	if admission.UserID <= 0 || admission.Generation <= 0 || admission.PolicyKey == "" ||
		admission.EventID == "" || len(admission.EventID) > 120 || strings.TrimSpace(admission.EventID) != admission.EventID || admission.AdmittedAt.IsZero() {
		return nil, infraerrors.BadRequest("INVALID_USER_RISK_EVENT", "缺少有效的服务端请求准入凭证")
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var role string
	if err := scanSingleRow(ctx, tx, "SELECT role FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE",
		[]any{admission.UserID}, &role); err != nil {
		return nil, err
	}

	// Deduplication precedes policy validation and cycle evaluation. A retry
	// receives its stored original outcome even after policy edits or release.
	var originalUserID int64
	var storedResult []byte
	err = scanSingleRow(ctx, tx, "SELECT user_id,result FROM user_risk_events WHERE policy_key=$1 AND event_id=$2",
		[]any{admission.PolicyKey, admission.EventID}, &originalUserID, &storedResult)
	if err == nil {
		if originalUserID != admission.UserID {
			return nil, fmt.Errorf("risk event identity collision")
		}
		var result service.UserRiskDecision
		if err := json.Unmarshal(storedResult, &result); err != nil {
			return nil, err
		}
		result.Duplicate = true
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return &result, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	input.RequestID = userRiskAuditText(input.RequestID, 128)
	input.Model = userRiskAuditText(input.Model, 255)
	input.Endpoint = userRiskAuditText(input.Endpoint, 255)
	if err := service.ValidateUserRiskPolicySnapshot(input.Policy); err != nil {
		return nil, err
	}
	if input.Policy.Scope != admission.Scope {
		return nil, infraerrors.BadRequest("USER_RISK_SCOPE_MISMATCH", "请求范围与用户限制规则不一致")
	}
	policyRaw, err := json.Marshal(input.Policy)
	if err != nil {
		return nil, err
	}
	var generation int64
	var now time.Time
	if err := scanSingleRow(ctx, tx,
		"SELECT generation,clock_timestamp() FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2 FOR UPDATE",
		[]any{admission.UserID, admission.PolicyKey}, &generation, &now); err != nil {
		return nil, err
	}
	result := service.UserRiskDecision{
		EventID: admission.EventID, UserID: admission.UserID, PolicyKey: admission.PolicyKey,
		Scope: input.Policy.Scope, PolicyRevision: input.Policy.Revision, Generation: admission.Generation, RecordedAt: now,
	}
	counted := true
	switch {
	case admission.Generation != generation:
		counted = false
		result.IgnoredReason = "cycle_changed"
	case role == service.RoleAdmin:
		counted = false
		result.IgnoredReason = "administrator_exempt"
	case input.ExemptReason != "":
		counted = false
		result.IgnoredReason = input.ExemptReason
	}
	action := "ignored"
	if counted {
		var prior int
		if err := scanSingleRow(ctx, tx,
			"SELECT COUNT(*) FROM user_risk_events WHERE user_id=$1 AND policy_key=$2 AND generation=$3 AND counted AND recorded_at>$4::timestamptz-($5::int*INTERVAL '1 day') AND recorded_at <= $4",
			[]any{admission.UserID, admission.PolicyKey, generation, now, input.Policy.WindowDays}, &prior); err != nil {
			return nil, err
		}
		result.HitCount = prior + 1
		state, err := loadUserRiskRestriction(ctx, tx, admission.UserID, admission.PolicyKey)
		if err != nil {
			return nil, err
		}
		if err := expireUserRiskState(ctx, tx, admission.UserID, admission.PolicyKey, &state, now); err != nil {
			return nil, err
		}
		active := state.active(now)
		if active && state.scope != input.Policy.Scope {
			return nil, infraerrors.Conflict("USER_RISK_SCOPE_CONFLICT", "现有限制需解除后才能改变该规则的适用范围")
		}
		selected := service.UserRiskActionRule{}
		for _, rule := range input.Policy.Actions {
			if rule.Threshold <= result.HitCount {
				selected = rule
			}
		}
		action = "hit"
		result.RestrictionVersion = state.version
		if active && (state.manual || selected.Threshold <= state.threshold) {
			preserved := userRiskStateDecision(admission.UserID, admission.PolicyKey, state, now)
			preserved.EventID, preserved.HitCount = admission.EventID, result.HitCount
			result = preserved
		} else if selected.Threshold > 0 {
			var expiry any
			// Escalation strengthens one continuous restriction interval;
			// the action ledger records the later decision time separately.
			restrictedAt := now
			if active {
				restrictedAt = state.restrictedAt
			}
			result.Restricted, result.NewlyApplied = true, true
			result.Threshold, result.AdminRelease = selected.Threshold, selected.AdminRelease
			result.RestrictionVersion = state.version + 1
			if !selected.AdminRelease {
				until := now.Add(time.Duration(selected.DurationHours) * time.Hour)
				if active && state.expires.Valid && state.expires.Time.After(until) {
					until = state.expires.Time
				}
				expiry, result.ExpiresAt = until, &until
			}
			_, err := tx.ExecContext(ctx,
				"INSERT INTO user_risk_restrictions(user_id,policy_key,scope,generation,version,decision_hit_count,threshold,admin_release,restricted_at,expires_at,policy_revision,policy_snapshot,source_event_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12::jsonb,$13) ON CONFLICT(user_id,policy_key) DO UPDATE SET scope=EXCLUDED.scope,generation=EXCLUDED.generation,version=EXCLUDED.version,decision_hit_count=EXCLUDED.decision_hit_count,threshold=EXCLUDED.threshold,admin_release=EXCLUDED.admin_release,restricted_at=EXCLUDED.restricted_at,expires_at=EXCLUDED.expires_at,cleared_at=NULL,policy_revision=EXCLUDED.policy_revision,policy_snapshot=EXCLUDED.policy_snapshot,source_event_id=EXCLUDED.source_event_id",
				admission.UserID, admission.PolicyKey, input.Policy.Scope, generation, result.RestrictionVersion,
				result.HitCount, selected.Threshold, selected.AdminRelease, restrictedAt, expiry, input.Policy.Revision, string(policyRaw), admission.EventID)
			if err != nil {
				return nil, err
			}
			action = "restricted"
			if active {
				action = "escalated"
			}
		}
	}
	resultRaw, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	_, err = tx.ExecContext(ctx,
		"INSERT INTO user_risk_events(policy_key,event_id,user_id,generation,request_id,group_id,model,endpoint,counted,policy_revision,policy_snapshot,result,admitted_at,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11::jsonb,$12::jsonb,$13,$14)",
		admission.PolicyKey, admission.EventID, admission.UserID, admission.Generation, input.RequestID, input.GroupID,
		input.Model, input.Endpoint, counted, input.Policy.Revision, string(policyRaw), string(resultRaw), admission.AdmittedAt, now)
	if err != nil {
		return nil, err
	}
	if err := insertUserRiskAction(ctx, tx, action, "event/"+admission.EventID, admission.EventID, nil, "", result, now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &result, nil
}
