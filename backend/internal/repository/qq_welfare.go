package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

type qqWelfareRepository struct{ db *sql.DB }

func NewQQWelfareRepository(db *sql.DB) service.QQWelfareRepository {
	return &qqWelfareRepository{db: db}
}

func (r *qqWelfareRepository) Config(ctx context.Context) (*service.QQWelfareConfig, error) {
	c := &service.QQWelfareConfig{}
	err := r.db.QueryRowContext(ctx, `SELECT require_qq_for_api,enabled,group_number,group_open_id,bot_key,last_bot_seen_at FROM qq_welfare_config WHERE id=1`).Scan(&c.RequireQQForAPI, &c.Enabled, &c.GroupNumber, &c.GroupOpenID, &c.BotKey, &c.LastBotSeenAt)
	if errors.Is(err, sql.ErrNoRows) {
		b := make([]byte, 32)
		if _, err = rand.Read(b); err != nil {
			return nil, err
		}
		if _, err = r.db.ExecContext(ctx, `INSERT INTO qq_welfare_config(id,bot_key) VALUES(1,$1) ON CONFLICT DO NOTHING`, hex.EncodeToString(b)); err != nil {
			return nil, err
		}
		return r.Config(ctx)
	}
	return c, err
}
func (r *qqWelfareRepository) UpdateConfig(ctx context.Context, enabled bool, number string, requireQQ ...bool) error {
	if _, err := r.Config(ctx); err != nil {
		return err
	}
	required := false
	if len(requireQQ) > 0 {
		required = requireQQ[0]
	}
	_, err := r.db.ExecContext(ctx, `UPDATE qq_welfare_config SET require_qq_for_api=$3,enabled=$1,group_open_id=CASE WHEN group_number=$2 THEN group_open_id ELSE '' END,group_number=$2,group_code_hash=NULL,group_code_expires=NULL,updated_at=now() WHERE id=1`, enabled, number, required)
	return err
}
func (r *qqWelfareRepository) GroupCode(ctx context.Context, hash string, expiry time.Time) error {
	result, err := r.db.ExecContext(ctx, `UPDATE qq_welfare_config SET group_code_hash=$1,group_code_expires=$2 WHERE id=1 AND group_number<>''`, hash, expiry)
	return welfareAffected(result, err, service.ErrWelfareUnavailable)
}
func (r *qqWelfareRepository) BindGroup(ctx context.Context, hash, group string) error {
	result, err := r.db.ExecContext(ctx, `UPDATE qq_welfare_config SET group_open_id=$1,group_code_hash=NULL,group_code_expires=NULL,updated_at=now() WHERE id=1 AND group_code_hash=$2 AND group_code_expires>now()`, group, hash)
	return welfareAffected(result, err, service.ErrWelfareBinding)
}
func (r *qqWelfareRepository) Binding(ctx context.Context, uid int64) (*service.QQWelfareBinding, error) {
	cfg, err := r.Config(ctx)
	if err != nil {
		return nil, err
	}
	out := &service.QQWelfareBinding{RequireQQForAPI: cfg.RequireQQForAPI, GroupNumber: cfg.GroupNumber, Ready: (cfg.Enabled || cfg.RequireQQForAPI) && cfg.GroupOpenID != ""}
	err = r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qq_welfare_accounts WHERE user_id=$1 AND group_id=$2)`, uid, cfg.GroupOpenID).Scan(&out.Verified)
	out.Verified = out.Verified && out.Ready
	return out, err
}
func (r *qqWelfareRepository) UserIdentity(ctx context.Context, uid int64) (*service.QQWelfareEvent, error) {
	e := &service.QQWelfareEvent{}
	err := r.db.QueryRowContext(ctx, `SELECT a.qq_id,a.group_id FROM qq_welfare_accounts a JOIN users u ON u.id=a.user_id JOIN qq_welfare_config c ON c.id=1 WHERE a.user_id=$1 AND u.status='active' AND u.deleted_at IS NULL AND c.enabled AND c.group_open_id<>'' AND a.group_id=c.group_open_id`, uid).Scan(&e.ActorID, &e.GroupID)
	return e, welfareMissing(err, service.ErrWelfareUnverified)
}
func (r *qqWelfareRepository) CreateBinding(ctx context.Context, uid int64, hash string, expiry time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	var group string
	if err = tx.QueryRowContext(ctx, `SELECT group_open_id FROM qq_welfare_config WHERE id=1 AND (enabled OR require_qq_for_api) AND group_open_id<>'' FOR SHARE`).Scan(&group); err != nil {
		return welfareMissing(err, service.ErrWelfareUnavailable)
	}
	var active int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR UPDATE`, uid).Scan(&active); err != nil {
		return welfareMissing(err, service.ErrUserNotFound)
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qq_welfare_accounts WHERE user_id=$1 AND group_id=$2)`, uid, group).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return service.ErrWelfareBinding
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO qq_welfare_bind_codes(user_id,code_hash,expires_at) VALUES($1,$2,$3) ON CONFLICT(user_id) DO UPDATE SET code_hash=EXCLUDED.code_hash,expires_at=EXCLUDED.expires_at,created_at=now() WHERE qq_welfare_bind_codes.created_at<now()-interval '60 seconds'`, uid, hash, expiry)
	if err = welfareAffected(result, err, service.ErrWelfareRate); err != nil {
		return err
	}
	return tx.Commit()
}
func welfareGroupTx(ctx context.Context, tx *sql.Tx, group string) error {
	var configured string
	err := tx.QueryRowContext(ctx, `SELECT group_open_id FROM qq_welfare_config WHERE id=1 AND enabled AND group_open_id<>'' FOR SHARE`).Scan(&configured)
	if err != nil {
		return welfareMissing(err, service.ErrWelfareUnavailable)
	}
	if configured != group {
		return service.ErrWelfareGroup
	}
	return nil
}
func (r *qqWelfareRepository) Verify(ctx context.Context, e service.QQWelfareEvent, hash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err = welfareBindingGroupTx(ctx, tx, e.GroupID); err != nil {
		return err
	}
	var uid int64
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM qq_welfare_bind_codes WHERE code_hash=$1 AND expires_at>now()`, hash).Scan(&uid); err != nil {
		return welfareMissing(err, service.ErrWelfareBinding)
	}
	var active int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND status='active' AND deleted_at IS NULL FOR UPDATE`, uid).Scan(&active); err != nil {
		return welfareMissing(err, service.ErrWelfareBinding)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM qq_welfare_bind_codes WHERE user_id=$1 AND code_hash=$2 AND expires_at>now()`, uid, hash)
	if err = welfareAffected(result, err, service.ErrWelfareBinding); err != nil {
		return err
	}
	result, err = tx.ExecContext(ctx, `INSERT INTO qq_welfare_accounts(user_id,qq_id,group_id) VALUES($1,$2,$3) ON CONFLICT(user_id) DO UPDATE SET group_id=EXCLUDED.group_id,verified_at=now() WHERE qq_welfare_accounts.qq_id=EXCLUDED.qq_id`, uid, e.ActorID, e.GroupID)
	if err = welfareAffected(result, err, service.ErrWelfareBinding); err != nil {
		return welfareUnique(err, service.ErrWelfareBinding)
	}
	return tx.Commit()
}
func welfareAccountTx(ctx context.Context, tx *sql.Tx, e service.QQWelfareEvent) (int64, int64, error) {
	if err := welfareGroupTx(ctx, tx, e.GroupID); err != nil {
		return 0, 0, err
	}
	var uid, points int64
	err := tx.QueryRowContext(ctx, `SELECT a.user_id,a.points FROM qq_welfare_accounts a JOIN users u ON u.id=a.user_id WHERE a.qq_id=$1 AND a.group_id=$2 AND u.status='active' AND u.deleted_at IS NULL FOR UPDATE OF a,u`, e.ActorID, e.GroupID).Scan(&uid, &points)
	return uid, points, welfareMissing(err, service.ErrWelfareUnverified)
}
func (r *qqWelfareRepository) CheckIn(ctx context.Context, e service.QQWelfareEvent) (bool, int64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return false, 0, err
	}
	defer func() { _ = tx.Rollback() }()
	uid, points, err := welfareAccountTx(ctx, tx, e)
	if err != nil {
		return false, 0, err
	}
	var day string
	err = tx.QueryRowContext(ctx, `INSERT INTO qq_welfare_checkins(user_id,day) VALUES($1,(now() AT TIME ZONE 'Asia/Shanghai')::date) ON CONFLICT DO NOTHING RETURNING day::text`, uid).Scan(&day)
	if errors.Is(err, sql.ErrNoRows) {
		return false, points, nil
	}
	if err != nil {
		return false, 0, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE qq_welfare_accounts SET points=points+1 WHERE user_id=$1`, uid); err != nil {
		return false, 0, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO qq_welfare_ledger(user_id,delta,reason,reference) VALUES($1,1,'checkin',$2)`, uid, fmt.Sprintf("checkin:%d:%s", uid, day)); err != nil {
		return false, 0, err
	}
	err = tx.Commit()
	return true, points + 1, err
}
func (r *qqWelfareRepository) Snapshot(ctx context.Context, e service.QQWelfareEvent) (*service.QQWelfareSnapshot, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	uid, points, err := welfareAccountTx(ctx, tx, e)
	if err != nil {
		return nil, err
	}
	snap := &service.QQWelfareSnapshot{Points: points, GeneratedAt: time.Now(), Products: []service.QQWelfareProduct{}, Orders: []service.QQWelfareOrder{}}
	err = tx.QueryRowContext(ctx, `SELECT c.group_number,EXISTS(SELECT 1 FROM qq_welfare_checkins WHERE user_id=$1 AND day=(now() AT TIME ZONE 'Asia/Shanghai')::date) FROM qq_welfare_config c WHERE c.id=1`, uid).Scan(&snap.GroupNumber, &snap.CheckedIn)
	if err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,version,title,points_cost,balance_value,stock,enabled,expires_at FROM qq_welfare_rewards WHERE enabled AND stock>0 AND (expires_at IS NULL OR expires_at>now()) ORDER BY id LIMIT 50`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p service.QQWelfareProduct
		if err = rows.Scan(&p.ID, &p.Version, &p.Title, &p.PointsCost, &p.BalanceValue, &p.Stock, &p.Enabled, &p.ExpiresAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		snap.Products = append(snap.Products, p)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount),0) FROM paid_balance_credits WHERE user_id=$1`, uid).Scan(&snap.PaidBalanceTotal); err != nil {
		return nil, err
	}
	snap.Orders, err = welfareOrders(ctx, tx, uid, 100)
	if err != nil {
		return nil, err
	}
	return snap, tx.Commit()
}
func (r *qqWelfareRepository) Fulfill(ctx context.Context, e service.QQWelfareEvent, id, code string) (*service.QQWelfareOrder, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	uid, points, err := welfareAccountTx(ctx, tx, e)
	if err != nil {
		return nil, err
	}
	key := service.WelfareHash(e.GroupID + "\x00" + e.ActorID + "\x00" + e.MessageID)
	existing := &service.QQWelfareOrder{}
	err = tx.QueryRowContext(ctx, `SELECT id,user_id,title,points_cost,balance_value,created_at FROM qq_welfare_orders WHERE request_key=$1 AND user_id=$2`, key, uid).Scan(&existing.ID, &existing.UserID, &existing.Title, &existing.PointsCost, &existing.BalanceValue, &existing.CreatedAt)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	order := &service.QQWelfareOrder{ID: id, UserID: uid}
	var rewardID, couponID *int64
	if e.Action == "exchange" {
		rewardID = &e.ProductID
		var version int64
		err = tx.QueryRowContext(ctx, `SELECT title,points_cost,balance_value,version FROM qq_welfare_rewards WHERE id=$1 AND enabled AND stock>0 AND (expires_at IS NULL OR expires_at>now()) FOR UPDATE`, e.ProductID).Scan(&order.Title, &order.PointsCost, &order.BalanceValue, &version)
		if err != nil {
			return nil, welfareMissing(err, service.ErrWelfareSoldOut)
		}
		if e.IsWebsite && e.ProductVersion != version {
			return nil, infraerrors.Conflict("WELFARE_STALE_PRODUCT", "商品已发生变化，请刷新后重新确认兑换。")
		}
		if points < int64(order.PointsCost) {
			return nil, service.ErrWelfarePoints
		}
		if _, err = tx.ExecContext(ctx, `UPDATE qq_welfare_rewards SET stock=stock-1,version=version+1,updated_at=now() WHERE id=$1`, e.ProductID); err != nil {
			return nil, err
		}
	} else {
		var cid int64
		var maxClaims, claimed int
		var minPaid float64
		err = tx.QueryRowContext(ctx, `SELECT id,title,balance_value,max_claims,claimed,min_paid_balance FROM qq_welfare_coupons WHERE code=$1 AND enabled AND (expires_at IS NULL OR expires_at>now()) FOR UPDATE`, e.Code).Scan(&cid, &order.Title, &order.BalanceValue, &maxClaims, &claimed, &minPaid)
		if err != nil {
			return nil, welfareMissing(err, service.ErrWelfareSoldOut)
		}
		var qualified bool
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(sum(amount),0) >= $2::numeric FROM paid_balance_credits WHERE user_id=$1`, uid, minPaid).Scan(&qualified); err != nil {
			return nil, err
		}
		if !qualified {
			return nil, infraerrors.Forbidden("WELFARE_PAID_BALANCE_REQUIRED", fmt.Sprintf("累计付费充值需达到 $%s 才能领取此福利码。", strconv.FormatFloat(minPaid, 'f', -1, 64)))
		}
		couponID = &cid
		var used bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM qq_welfare_claims WHERE coupon_id=$1 AND user_id=$2)`, cid, uid).Scan(&used); err != nil {
			return nil, err
		}
		if used {
			return nil, service.ErrWelfareAlreadyClaimed
		}
		if maxClaims > 0 && claimed >= maxClaims {
			return nil, service.ErrWelfareSoldOut
		}
		if _, err = tx.ExecContext(ctx, `UPDATE qq_welfare_coupons SET claimed=claimed+1,updated_at=now() WHERE id=$1`, cid); err != nil {
			return nil, err
		}
	}
	// The individual code is consumed in this same transaction: a group message never exposes a usable bearer code.
	var redeemID int64
	err = tx.QueryRowContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,used_by,used_at,notes,created_at,validity_days,balance_source) VALUES($1,'admin_balance',$2,'used',$3,now(),$4,now(),0,'gift') RETURNING id`, code, order.BalanceValue, uid, "QQ 福利："+order.Title).Scan(&redeemID)
	if err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1,updated_at=now() WHERE id=$2`, order.BalanceValue, uid); err != nil {
		return nil, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE qq_welfare_accounts SET points=points-$1 WHERE user_id=$2`, order.PointsCost, uid); err != nil {
		return nil, err
	}
	err = tx.QueryRowContext(ctx, `INSERT INTO qq_welfare_orders(id,request_key,user_id,reward_id,coupon_id,title,points_cost,balance_value,redeem_code_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING created_at`, id, key, uid, rewardID, couponID, order.Title, order.PointsCost, order.BalanceValue, redeemID).Scan(&order.CreatedAt)
	if err != nil {
		return nil, err
	}
	if couponID != nil {
		if _, err = tx.ExecContext(ctx, `INSERT INTO qq_welfare_claims(coupon_id,user_id,order_id) VALUES($1,$2,$3)`, *couponID, uid, id); err != nil {
			return nil, err
		}
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO qq_welfare_ledger(user_id,delta,reason,reference) VALUES($1,$2,'redeem',$3)`, uid, -order.PointsCost, "order:"+id); err != nil {
		return nil, err
	}
	return order, tx.Commit()
}

type welfareQueryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func welfareOrders(ctx context.Context, q welfareQueryer, uid int64, limit int) ([]service.QQWelfareOrder, error) {
	rows, err := q.QueryContext(ctx, `SELECT id,user_id,title,points_cost,balance_value,created_at FROM qq_welfare_orders WHERE ($1::bigint=0 OR user_id=$1) ORDER BY created_at DESC,id DESC LIMIT $2`, uid, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []service.QQWelfareOrder{}
	for rows.Next() {
		var o service.QQWelfareOrder
		if err = rows.Scan(&o.ID, &o.UserID, &o.Title, &o.PointsCost, &o.BalanceValue, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}
func (r *qqWelfareRepository) Admin(ctx context.Context) (*service.QQWelfareAdmin, error) {
	cfg, err := r.Config(ctx)
	if err != nil {
		return nil, err
	}
	out := &service.QQWelfareAdmin{Config: cfg, Products: []service.QQWelfareProduct{}, Coupons: []service.QQWelfareCoupon{}}
	if err = r.db.QueryRowContext(ctx, `SELECT count(*) FROM qq_welfare_accounts`).Scan(&out.VerifiedAccounts); err != nil {
		return nil, err
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id,version,title,points_cost,balance_value,stock,enabled,expires_at FROM qq_welfare_rewards ORDER BY id DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p service.QQWelfareProduct
		if err = rows.Scan(&p.ID, &p.Version, &p.Title, &p.PointsCost, &p.BalanceValue, &p.Stock, &p.Enabled, &p.ExpiresAt); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Products = append(out.Products, p)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	rows, err = r.db.QueryContext(ctx, `SELECT id,code,title,balance_value,max_claims,claimed,enabled,expires_at,min_paid_balance FROM qq_welfare_coupons ORDER BY id DESC LIMIT 1000`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var p service.QQWelfareCoupon
		if err = rows.Scan(&p.ID, &p.Code, &p.Title, &p.BalanceValue, &p.MaxClaims, &p.Claimed, &p.Enabled, &p.ExpiresAt, &p.MinPaidBalance); err != nil {
			_ = rows.Close()
			return nil, err
		}
		out.Coupons = append(out.Coupons, p)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	out.Orders, err = welfareAdminOrders(ctx, r.db, 0, 100, 0)
	return out, err
}
func (r *qqWelfareRepository) SaveProduct(ctx context.Context, p *service.QQWelfareProduct) error {
	if p.ID == 0 {
		return r.db.QueryRowContext(ctx, `INSERT INTO qq_welfare_rewards(title,points_cost,balance_value,stock,enabled,expires_at) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,version`, p.Title, p.PointsCost, p.BalanceValue, p.Stock, p.Enabled, p.ExpiresAt).Scan(&p.ID, &p.Version)
	}
	err := r.db.QueryRowContext(ctx, `UPDATE qq_welfare_rewards SET title=$1,points_cost=$2,balance_value=$3,stock=$4,enabled=$5,expires_at=$6,version=version+1,updated_at=now() WHERE id=$7 AND version=$8 RETURNING version`, p.Title, p.PointsCost, p.BalanceValue, p.Stock, p.Enabled, p.ExpiresAt, p.ID, p.Version).Scan(&p.Version)
	return welfareMissing(err, infraerrors.Conflict("WELFARE_STALE_PRODUCT", "商品或库存已发生变化，请刷新后重新编辑。"))
}
func (r *qqWelfareRepository) SaveCoupon(ctx context.Context, p *service.QQWelfareCoupon) error {
	if p.ID == 0 {
		err := r.db.QueryRowContext(ctx, `INSERT INTO qq_welfare_coupons(code,title,balance_value,max_claims,enabled,expires_at,min_paid_balance) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, p.Code, p.Title, p.BalanceValue, p.MaxClaims, p.Enabled, p.ExpiresAt, p.MinPaidBalance).Scan(&p.ID)
		return welfareUnique(err, infraerrors.Conflict("WELFARE_CODE_EXISTS", "此兑换码已存在，请编辑原兑换码。"))
	}
	// A campaign's public code is immutable, so renaming cannot reset per-account claim identity.
	result, err := r.db.ExecContext(ctx, `UPDATE qq_welfare_coupons SET title=$1,balance_value=$2,max_claims=$3,enabled=$4,expires_at=$5,min_paid_balance=$8,updated_at=now() WHERE id=$6 AND code=$7 AND ($3=0 OR $3>=claimed)`, p.Title, p.BalanceValue, p.MaxClaims, p.Enabled, p.ExpiresAt, p.ID, p.Code, p.MinPaidBalance)
	return welfareAffected(result, err, infraerrors.Conflict("WELFARE_INVALID_UPDATE", "兑换码不可改名，领取上限不可小于已领取数量。"))
}
func (r *qqWelfareRepository) TouchAndLimit(ctx context.Context, e service.QQWelfareEvent) error {
	if _, err := r.Config(ctx); err != nil {
		return err
	}
	var err error
	if !e.IsWebsite {
		_, err = r.db.ExecContext(ctx, `UPDATE qq_welfare_config SET last_bot_seen_at=now() WHERE id=1 AND (last_bot_seen_at IS NULL OR last_bot_seen_at<now()-interval '15 seconds')`)
		if err != nil {
			return err
		}
	}
	var count int
	key := service.WelfareHash(e.GroupID + "\x00" + e.ActorID)
	err = r.db.QueryRowContext(ctx, `INSERT INTO qq_welfare_limits(key,started_at,count) VALUES($1,now(),1) ON CONFLICT(key) DO UPDATE SET started_at=CASE WHEN qq_welfare_limits.started_at<now()-interval '1 minute' THEN now() ELSE qq_welfare_limits.started_at END,count=CASE WHEN qq_welfare_limits.started_at<now()-interval '1 minute' THEN 1 ELSE qq_welfare_limits.count+1 END RETURNING count`, key).Scan(&count)
	if err != nil {
		return err
	}
	if count > 20 {
		return service.ErrWelfareRate
	}
	_, err = r.db.ExecContext(ctx, `DELETE FROM qq_welfare_limits WHERE key IN (SELECT key FROM qq_welfare_limits WHERE started_at<now()-interval '1 day' LIMIT 50)`)
	return err
}
func welfareAffected(result sql.Result, err, missing error) error {
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return missing
	}
	return nil
}
func welfareMissing(err, missing error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return missing
	}
	return err
}
func welfareUnique(err, conflict error) error {
	var pg *pq.Error
	if errors.As(err, &pg) && pg.Code == "23505" {
		return conflict
	}
	return err
}
