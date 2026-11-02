package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

func normalizeBillingIdentity(value string) string {
	if value == service.BillingIdentityPurchase {
		return value
	}
	return service.BillingIdentityStandard
}

// MigratePersonalRatesToPurchase archives the exact previous state, enrolls
// live override owners, and clears only rates (including orphan group rows).
// No data changes occur at application startup. A repeat call is a no-op.
func (r *userRepository) MigratePersonalRatesToPurchase(ctx context.Context, actorAdminID int64) (*service.BillingIdentityMigrationResult, error) {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	db := tx.Client()
	if _, err = db.ExecContext(ctx, `LOCK TABLE user_group_rate_multipliers IN SHARE ROW EXCLUSIVE MODE`); err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, `SELECT u.id FROM users u WHERE u.deleted_at IS NULL AND EXISTS (SELECT 1 FROM user_group_rate_multipliers r WHERE r.user_id=u.id AND r.rate_multiplier IS NOT NULL) ORDER BY u.id FOR UPDATE OF u`)
	if err != nil {
		return nil, err
	}
	result := &service.BillingIdentityMigrationResult{}
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			_ = rows.Close()
			return nil, err
		}
		result.UserIDs = append(result.UserIDs, id)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = db.QueryContext(ctx, `INSERT INTO billing_identity_migrations(actor_admin_id, backup)
 SELECT $1, jsonb_build_object('rates',(SELECT jsonb_agg(to_jsonb(r)) FROM user_group_rate_multipliers r WHERE rate_multiplier IS NOT NULL),
 'users',(SELECT jsonb_agg(jsonb_build_object('id',id,'billing_identity',billing_identity)) FROM users WHERE id=ANY($2)))
 WHERE EXISTS(SELECT 1 FROM user_group_rate_multipliers WHERE rate_multiplier IS NOT NULL) RETURNING id`, actorAdminID, pq.Array(result.UserIDs))
	if err != nil {
		return nil, err
	}
	if rows.Next() {
		err = rows.Scan(&result.BackupID)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(result.UserIDs) > 0 {
		if _, err = db.ExecContext(ctx, `UPDATE users SET billing_identity='purchase', updated_at=NOW() WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(result.UserIDs)); err != nil {
			return nil, err
		}
	}
	changed, err := db.ExecContext(ctx, `UPDATE user_group_rate_multipliers SET rate_multiplier=NULL, updated_at=NOW() WHERE rate_multiplier IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	result.ClearedRates, err = changed.RowsAffected()
	if err != nil {
		return nil, err
	}
	if _, err = db.ExecContext(ctx, `DELETE FROM user_group_rate_multipliers WHERE rate_multiplier IS NULL AND rpm_override IS NULL`); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	result.MigratedUsers = len(result.UserIDs)
	return result, nil
}
