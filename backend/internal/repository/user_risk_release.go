package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func userRiskReleaseResult(decision service.UserRiskDecision) *service.UserRiskReleaseResult {
	return &service.UserRiskReleaseResult{
		Released: decision.Released, UserID: decision.UserID, PolicyKey: decision.PolicyKey,
		Generation: decision.Generation, Version: decision.RestrictionVersion, BaseStatus: decision.BaseStatus,
	}
}

func (r *contentModerationRepository) ReleaseUserRiskRestriction(ctx context.Context, input service.UserRiskReleaseInput) (*service.UserRiskReleaseResult, error) {
	return r.userRiskStore().release(ctx, input)
}

func (r *userRiskStore) release(ctx context.Context, input service.UserRiskReleaseInput) (*service.UserRiskReleaseResult, error) {
	input.Reason = strings.TrimSpace(input.Reason)
	if input.UserID <= 0 || input.ActorID <= 0 || input.ExpectedVersion <= 0 ||
		input.PolicyKey == "" || len(input.PolicyKey) > 96 || strings.TrimSpace(input.PolicyKey) != input.PolicyKey ||
		input.OperationID == "" || len(input.OperationID) > 120 || strings.TrimSpace(input.OperationID) != input.OperationID ||
		input.Reason == "" || utf8.RuneCountInString(input.Reason) > 1000 ||
		!utf8.ValidString(input.Reason) || strings.ContainsRune(input.Reason, 0) || strings.ContainsRune(input.OperationID, 0) {
		return nil, infraerrors.BadRequest("INVALID_USER_RISK_RELEASE", "解封需要有效的管理员、限制版本、操作编号和原因")
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	var baseStatus string
	if err := scanSingleRow(ctx, tx, "SELECT status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE",
		[]any{input.UserID}, &baseStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, service.ErrUserNotFound
		}
		return nil, err
	}
	// This is an authoritative permission read without a second user-row lock.
	// Locking the actor after the target could deadlock cross-admin actions.
	var actorRole, actorStatus, actorName, actorEmail string
	err = scanSingleRow(ctx, tx, "SELECT role,status,COALESCE(username,''),email FROM users WHERE id=$1 AND deleted_at IS NULL",
		[]any{input.ActorID}, &actorRole, &actorStatus, &actorName, &actorEmail)
	if errors.Is(err, sql.ErrNoRows) || (err == nil && (actorRole != service.RoleAdmin || actorStatus != service.StatusActive)) {
		return nil, infraerrors.Forbidden("USER_RISK_RELEASE_NOT_ALLOWED", "只有有效的管理员账号可以解除用户限制")
	}
	if err != nil {
		return nil, err
	}
	operationID := "release/" + input.OperationID
	var previousUserID int64
	var previousReason string
	var previousRaw []byte
	err = scanSingleRow(ctx, tx,
		"SELECT user_id,reason,decision FROM user_risk_actions WHERE policy_key=$1 AND operation_id=$2 AND action='released'",
		[]any{input.PolicyKey, operationID}, &previousUserID, &previousReason, &previousRaw)
	if err == nil {
		var previous service.UserRiskDecision
		if err := json.Unmarshal(previousRaw, &previous); err != nil {
			return nil, err
		}
		if previousUserID != input.UserID || previous.ActorID != input.ActorID || previousReason != input.Reason ||
			previous.RestrictionVersion != input.ExpectedVersion+1 {
			return nil, infraerrors.Conflict("USER_RISK_RELEASE_IDEMPOTENCY_CONFLICT", "该操作编号已用于不同的解封请求")
		}
		if err := tx.Commit(); err != nil {
			return nil, err
		}
		return userRiskReleaseResult(previous), nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	var generation int64
	var now time.Time
	if err := scanSingleRow(ctx, tx,
		"SELECT generation,clock_timestamp() FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2 FOR UPDATE",
		[]any{input.UserID, input.PolicyKey}, &generation, &now); err != nil {
		return nil, err
	}
	state, err := loadUserRiskRestriction(ctx, tx, input.UserID, input.PolicyKey)
	if err != nil {
		return nil, err
	}
	if !state.active(now) || state.version != input.ExpectedVersion || state.generation != generation {
		return nil, infraerrors.Conflict("USER_RISK_RESTRICTION_CONFLICT", "用户限制已变化或已到期，请重新读取后再操作")
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE user_risk_cycles SET generation=generation+1,reset_at=$3 WHERE user_id=$1 AND policy_key=$2",
		input.UserID, input.PolicyKey, now); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE user_risk_restrictions SET cleared_at=$3,version=version+1 WHERE user_id=$1 AND policy_key=$2",
		input.UserID, input.PolicyKey, now); err != nil {
		return nil, err
	}
	decision := userRiskStateDecision(input.UserID, input.PolicyKey, state, now)
	decision.Generation = generation + 1
	decision.RestrictionVersion = state.version + 1
	decision.HitCount, decision.Restricted, decision.Released = 0, false, true
	decision.BaseStatus, decision.ActorID = baseStatus, input.ActorID
	decision.ActorLabel = actorName
	if decision.ActorLabel == "" {
		decision.ActorLabel = actorEmail
	}
	if err := insertUserRiskAction(ctx, tx, "released", operationID, state.sourceEventID,
		input.ActorID, input.Reason, decision, now); err != nil {
		_ = tx.Rollback()
		return nil, userRiskReleasePersistenceError(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return userRiskReleaseResult(decision), nil
}
