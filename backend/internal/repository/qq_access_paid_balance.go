package repository

import (
	"context"
	"database/sql"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// CheckQQAPIAccess deliberately bypasses the API key cache. Changing the policy or
// authentication group must take effect across instances and established sessions.
func (r *userRepository) CheckQQAPIAccess(ctx context.Context, uid int64) error {
	rows, err := clientFromContext(ctx, r.client).QueryContext(ctx, `SELECT NOT c.require_qq_for_api OR
 (c.group_open_id<>'' AND EXISTS(SELECT 1 FROM qq_welfare_accounts a WHERE a.user_id=$1 AND a.group_id=c.group_open_id))
 FROM (SELECT 1) seed LEFT JOIN qq_welfare_config c ON c.id=1`, uid)
	if err != nil {
		return infraerrors.ServiceUnavailable("QQ_VERIFICATION_UNAVAILABLE", "QQ 认证状态暂时无法查询，请稍后重试。")
	}
	defer func() { _ = rows.Close() }()
	var allowed sql.NullBool
	if !rows.Next() {
		return infraerrors.ServiceUnavailable("QQ_VERIFICATION_UNAVAILABLE", "QQ 认证状态暂时无法查询，请稍后重试。")
	}
	if err := rows.Scan(&allowed); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Missing singleton is the initial default (gate off), never a database error.
	if allowed.Valid && !allowed.Bool {
		return infraerrors.Forbidden("QQ_VERIFICATION_REQUIRED", "请先在网站个人资料完成 QQ 群认证后再使用 API。")
	}
	return nil
}

func welfareBindingGroupTx(ctx context.Context, tx *sql.Tx, group string) error {
	var configured string
	err := tx.QueryRowContext(ctx, `SELECT group_open_id FROM qq_welfare_config WHERE id=1 AND (enabled OR require_qq_for_api) AND group_open_id<>'' FOR SHARE`).Scan(&configured)
	if err != nil {
		return welfareMissing(err, service.ErrWelfareUnavailable)
	}
	if configured != group {
		return service.ErrWelfareGroup
	}
	return nil
}

func welfareAdminOrders(ctx context.Context, q welfareQueryer, couponID int64, limit, offset int) ([]service.QQWelfareAdminOrder, error) {
	rows, err := q.QueryContext(ctx, `SELECT o.id,o.user_id,o.title,o.points_cost,o.balance_value,o.created_at,
 COALESCE(u.username,''),COALESCE(u.email,''),o.coupon_id,COALESCE(c.code,'')
 FROM qq_welfare_orders o LEFT JOIN users u ON u.id=o.user_id LEFT JOIN qq_welfare_coupons c ON c.id=o.coupon_id
 WHERE ($1::bigint=0 OR o.coupon_id=$1) ORDER BY o.created_at DESC,o.id DESC LIMIT $2 OFFSET $3`, couponID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.QQWelfareAdminOrder{}
	for rows.Next() {
		var o service.QQWelfareAdminOrder
		if err := rows.Scan(&o.ID, &o.UserID, &o.Title, &o.PointsCost, &o.BalanceValue, &o.CreatedAt, &o.Username, &o.Email, &o.CouponID, &o.CouponCode); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *qqWelfareRepository) CouponClaims(ctx context.Context, id int64, page, size int) (*service.QQWelfareCouponClaims, error) {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	out := &service.QQWelfareCouponClaims{}
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM qq_welfare_orders WHERE coupon_id=$1`, id).Scan(&out.Total); err != nil {
		return nil, err
	}
	out.Orders, err = welfareAdminOrders(ctx, tx, id, size, (page-1)*size)
	if err != nil {
		return nil, err
	}
	return out, tx.Commit()
}
