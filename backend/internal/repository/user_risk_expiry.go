package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

var _ service.UserRiskPolicyRepository = (*contentModerationRepository)(nil)

func (r *contentModerationRepository) ExpireUserRiskRestrictions(ctx context.Context, limit int) ([]int64, error) {
	return r.userRiskStore().expire(ctx, limit)
}

func (r *userRiskStore) expire(ctx context.Context, limit int) ([]int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 100
	}
	rows, err := r.read.QueryContext(ctx,
		"SELECT user_id,policy_key FROM user_risk_restrictions WHERE cleared_at IS NULL AND NOT admin_release AND expires_at <= clock_timestamp() ORDER BY user_id,policy_key LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	type candidate struct {
		userID    int64
		policyKey string
	}
	candidates := []candidate{}
	for rows.Next() {
		var item candidate
		if err := rows.Scan(&item.userID, &item.policyKey); err != nil {
			_ = rows.Close()
			return nil, err
		}
		candidates = append(candidates, item)
	}
	readErr, closeErr := rows.Err(), rows.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	changed := []int64{}
	seen := map[int64]bool{}
	for _, item := range candidates {
		expired, err := r.expireOne(ctx, item.userID, item.policyKey)
		if err != nil {
			return changed, err
		}
		if expired && !seen[item.userID] {
			changed = append(changed, item.userID)
			seen[item.userID] = true
		}
	}
	return changed, nil
}

func (r *userRiskStore) expireOne(ctx context.Context, userID int64, policyKey string) (bool, error) {
	tx, err := r.begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	var lockedUserID int64
	err = scanSingleRow(ctx, tx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", []any{userID}, &lockedUserID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	var generation int64
	var now time.Time
	if err := scanSingleRow(ctx, tx,
		"SELECT generation,clock_timestamp() FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2 FOR UPDATE",
		[]any{userID, policyKey}, &generation, &now); err != nil {
		return false, err
	}
	state, err := loadUserRiskRestriction(ctx, tx, userID, policyKey)
	if err != nil {
		return false, err
	}
	if state.exists && !state.cleared.Valid && state.generation != generation {
		return false, fmt.Errorf("restriction generation differs from its policy cycle")
	}
	version := state.version
	if err := expireUserRiskState(ctx, tx, userID, policyKey, &state, now); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return state.version != version, nil
}
