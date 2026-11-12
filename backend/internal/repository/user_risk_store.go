package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type userRiskExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type userRiskTransaction interface {
	userRiskExecutor
	Commit() error
	Rollback() error
}

// Both repositories use one state machine and the same user-first lock order.
type userRiskStore struct {
	read  userRiskExecutor
	begin func(context.Context) (userRiskTransaction, error)
}

func (r *contentModerationRepository) userRiskStore() *userRiskStore {
	return &userRiskStore{read: r.db, begin: func(ctx context.Context) (userRiskTransaction, error) {
		return r.db.BeginTx(ctx, nil)
	}}
}

func (r *userRepository) userRiskStore() *userRiskStore {
	return &userRiskStore{read: r.client, begin: func(ctx context.Context) (userRiskTransaction, error) {
		return r.client.Tx(ctx)
	}}
}

func (r *contentModerationRepository) BeginUserRiskRequest(ctx context.Context, userID int64, policyKey, scope string) (service.UserRiskAdmission, error) {
	return r.userRiskStore().beginRequest(ctx, userID, policyKey, scope)
}

func (r *userRepository) BeginUserRiskRequest(ctx context.Context, userID int64, policyKey, scope string) (service.UserRiskAdmission, error) {
	return r.userRiskStore().beginRequest(ctx, userID, policyKey, scope)
}

func (r *userRiskStore) beginRequest(ctx context.Context, userID int64, policyKey, scope string) (service.UserRiskAdmission, error) {
	empty := service.UserRiskAdmission{}
	if userID <= 0 || policyKey == "" || len(policyKey) > 96 || strings.TrimSpace(policyKey) != policyKey ||
		scope == "" || len(scope) > 64 || strings.TrimSpace(scope) != scope {
		return empty, infraerrors.BadRequest("INVALID_USER_RISK_ADMISSION", "用户限制请求参数无效")
	}
	tx, err := r.begin(ctx)
	if err != nil {
		return empty, fmt.Errorf("begin user risk admission: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	var role, status string
	if err := scanSingleRow(ctx, tx, "SELECT role,status FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE",
		[]any{userID}, &role, &status); err != nil {
		return empty, fmt.Errorf("read user risk admission identity: %w", err)
	}
	if status != service.StatusActive {
		return empty, infraerrors.Forbidden("USER_INACTIVE", "账号已停用，请联系管理员。")
	}
	if role != service.RoleAdmin {
		var blocked bool
		if err := scanSingleRow(ctx, tx,
			"SELECT EXISTS(SELECT 1 FROM user_risk_restrictions WHERE user_id=$1 AND scope=$2 AND cleared_at IS NULL AND (admin_release OR expires_at IS NULL OR expires_at>clock_timestamp()))",
			[]any{userID, scope}, &blocked); err != nil {
			return empty, fmt.Errorf("check effective user access restrictions: %w", err)
		}
		if blocked {
			return empty, infraerrors.Forbidden("USER_ACCESS_RESTRICTED", "当前操作因用户限制暂停，请查看账户限制详情或联系管理员。")
		}
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO user_risk_cycles(user_id,policy_key) VALUES($1,$2) ON CONFLICT(user_id,policy_key) DO NOTHING",
		userID, policyKey); err != nil {
		return empty, fmt.Errorf("initialize user risk cycle: %w", err)
	}
	var generation int64
	var admittedAt time.Time
	if err := scanSingleRow(ctx, tx,
		"SELECT generation,clock_timestamp() FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2 FOR UPDATE",
		[]any{userID, policyKey}, &generation, &admittedAt); err != nil {
		return empty, fmt.Errorf("capture user risk generation: %w", err)
	}
	admission := service.UserRiskAdmission{
		EventID: rand.Text(), UserID: userID, PolicyKey: policyKey,
		Scope: scope, Generation: generation, AdmittedAt: admittedAt,
	}
	if err := tx.Commit(); err != nil {
		return empty, err
	}
	return admission, nil
}
