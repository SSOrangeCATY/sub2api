//go:build integration || welfareintegration

package repository

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type welfareFixture struct {
	db     *sql.DB
	repo   *qqWelfareRepository
	uid    int64
	event  service.QQWelfareEvent
	prefix string
}

func newWelfareFixture(t *testing.T) *welfareFixture {
	t.Helper()
	ctx := context.Background()
	db := welfareTestDB(t)
	prefix := fmt.Sprintf("welfare-%d", time.Now().UnixNano())
	r := &qqWelfareRepository{db: db}
	_, err := r.Config(ctx)
	require.NoError(t, err)
	require.NoError(t, r.UpdateConfig(ctx, true, "487375322"))
	require.NoError(t, r.GroupCode(ctx, service.WelfareHash("group-token"), time.Now().Add(time.Minute)))
	require.NoError(t, r.BindGroup(ctx, service.WelfareHash("group-token"), "group"))
	var uid int64
	err = db.QueryRowContext(ctx, `INSERT INTO users(email,password_hash,role,status,balance,created_at,updated_at) VALUES($1,'test','user','active',0,now(),now()) RETURNING id`, prefix+"@example.invalid").Scan(&uid)
	require.NoError(t, err)
	e := service.QQWelfareEvent{GroupID: "group", ActorID: prefix, MessageID: "first"}
	require.NoError(t, r.CreateBinding(ctx, uid, service.WelfareHash("binding-token"), time.Now().Add(time.Minute)))
	require.NoError(t, r.Verify(ctx, e, service.WelfareHash("binding-token")))
	f := &welfareFixture{db: db, repo: r, uid: uid, event: e, prefix: prefix}
	t.Cleanup(func() {
		_, err := db.ExecContext(ctx, `DELETE FROM qq_welfare_claims WHERE user_id=$1`, uid)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM qq_welfare_orders WHERE user_id=$1`, uid)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM qq_welfare_ledger WHERE user_id=$1`, uid)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM redeem_codes WHERE used_by=$1`, uid)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM qq_welfare_rewards WHERE title=$1`, prefix)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM qq_welfare_coupons WHERE title=$1`, prefix)
		require.NoError(t, err)
		_, err = db.ExecContext(ctx, `DELETE FROM users WHERE id=$1`, uid)
		require.NoError(t, err)
	})
	return f
}
func TestQQWelfareWebsiteSharesIdentityPointsAndReplay(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := context.Background()
	svc := service.NewQQWelfareService(f.repo, nil, nil)
	_, err := f.repo.UserIdentity(ctx, f.uid+1000000)
	require.ErrorIs(t, err, service.ErrWelfareUnverified)
	web, err := svc.UserAction(ctx, f.uid, "checkin", "", "", 0, 0)
	require.NoError(t, err)
	require.Contains(t, web.Message, "获得 1 积分")
	e := f.event
	e.Action = "checkin"
	bot, err := svc.Event(ctx, e)
	require.NoError(t, err)
	require.Contains(t, bot.Message, "今天已签到")
	e.Action = "points"
	bot, err = svc.Event(ctx, e)
	require.NoError(t, err)
	require.EqualValues(t, 1, bot.Points.Points)
	p := &service.QQWelfareProduct{Title: f.prefix, PointsCost: 1, BalanceValue: 0.25, Stock: 2, Enabled: true}
	require.NoError(t, f.repo.SaveProduct(ctx, p))
	_, err = svc.UserAction(ctx, f.uid, "exchange", "website-stale-1234567", "", p.ID, p.Version+1)
	require.Error(t, err)
	first, err := svc.UserAction(ctx, f.uid, "exchange", "website-request-123456", "", p.ID, p.Version)
	require.NoError(t, err)
	retry, err := svc.UserAction(ctx, f.uid, "exchange", "website-request-123456", "", p.ID, p.Version)
	require.NoError(t, err)
	require.Equal(t, first.Message, retry.Message)
	c := &service.QQWelfareCoupon{Title: f.prefix, Code: fmt.Sprintf("WEB_%d", f.uid), BalanceValue: 0.5, Enabled: true}
	require.NoError(t, f.repo.SaveCoupon(ctx, c))
	_, err = svc.UserAction(ctx, f.uid, "claim", "website-request-234567", c.Code, 0, 0)
	require.NoError(t, err)
	_, err = svc.UserAction(ctx, f.uid, "claim", "website-request-345678", c.Code, 0, 0)
	require.ErrorIs(t, err, service.ErrWelfareAlreadyClaimed)
	var balance float64
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.uid).Scan(&balance))
	require.Equal(t, 0.75, balance)
	_, err = f.db.ExecContext(ctx, `UPDATE qq_welfare_config SET last_bot_seen_at=NULL WHERE id=1`)
	require.NoError(t, err)
	_, err = svc.UserAction(ctx, f.uid, "overview", "", "", 0, 0)
	require.NoError(t, err)
	cfg, err := f.repo.Config(ctx)
	require.NoError(t, err)
	require.Nil(t, cfg.LastBotSeenAt)
	_, err = f.db.ExecContext(ctx, `UPDATE users SET status='disabled' WHERE id=$1`, f.uid)
	require.NoError(t, err)
	_, err = svc.UserAction(ctx, f.uid, "checkin", "", "", 0, 0)
	require.ErrorIs(t, err, service.ErrWelfareUnverified)
}
func TestQQWelfareConcurrentCheckInAndGate(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := context.Background()
	e := f.event
	e.GroupID = "other"
	_, err := f.repo.Snapshot(ctx, e)
	require.ErrorIs(t, err, service.ErrWelfareGroup)
	e = f.event
	e.ActorID = "unbound"
	_, err = f.repo.Snapshot(ctx, e)
	require.ErrorIs(t, err, service.ErrWelfareUnverified)
	var successes atomic.Int32
	var wg sync.WaitGroup
	errs := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fresh, _, err := f.repo.CheckIn(ctx, f.event)
			errs <- err
			if fresh {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, successes.Load())
	snap, err := f.repo.Snapshot(ctx, f.event)
	require.NoError(t, err)
	require.EqualValues(t, 1, snap.Points)
	require.True(t, snap.CheckedIn)
	var day string
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT day::text FROM qq_welfare_checkins WHERE user_id=$1`, f.uid).Scan(&day))
	require.Equal(t, time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02"), day)
}
func TestQQWelfareRedemptionAtomicReplayAndSharedOnce(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := context.Background()
	_, _, err := f.repo.CheckIn(ctx, f.event)
	require.NoError(t, err)
	p := &service.QQWelfareProduct{Title: f.prefix, PointsCost: 1, BalanceValue: 0.25, Stock: 1, Enabled: true}
	require.NoError(t, f.repo.SaveProduct(ctx, p))
	e := f.event
	e.Action = "exchange"
	e.ProductID = p.ID
	var wg sync.WaitGroup
	orders := make(chan *service.QQWelfareOrder, 8)
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			o, err := f.repo.Fulfill(ctx, e, fmt.Sprintf("%032d", n+1), fmt.Sprintf("%032d", n+101))
			orders <- o
			errs <- err
		}(i)
	}
	wg.Wait()
	close(orders)
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var receipt string
	for o := range orders {
		if receipt == "" {
			receipt = o.ID
		}
		require.Equal(t, receipt, o.ID)
	}
	var balance float64
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.uid).Scan(&balance))
	require.InDelta(t, 0.25, balance, 1e-8)
	var points, stock int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT points FROM qq_welfare_accounts WHERE user_id=$1`, f.uid).Scan(&points))
	require.Zero(t, points)
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT stock FROM qq_welfare_rewards WHERE id=$1`, p.ID).Scan(&stock))
	require.Zero(t, stock)
	// A descriptive edit from a form opened before the sale must not restock it.
	p.Title = f.prefix + " edited"
	require.Error(t, f.repo.SaveProduct(ctx, p))
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT stock FROM qq_welfare_rewards WHERE id=$1`, p.ID).Scan(&stock))
	require.Zero(t, stock)
	c := &service.QQWelfareCoupon{Title: f.prefix, Code: "ONCE_" + fmt.Sprint(f.uid), BalanceValue: 0.5, Enabled: true}
	require.NoError(t, f.repo.SaveCoupon(ctx, c))
	e.Action = "claim"
	e.Code = c.Code
	e.MessageID = "coupon-1"
	_, err = f.repo.Fulfill(ctx, e, "coupon-order-"+f.prefix[len(f.prefix)-12:], "coupon-code-"+f.prefix[len(f.prefix)-12:])
	require.NoError(t, err)
	e.MessageID = "coupon-2"
	_, err = f.repo.Fulfill(ctx, e, "unused-order", "unused-code")
	require.ErrorIs(t, err, service.ErrWelfareAlreadyClaimed)
	c.Enabled = false
	require.NoError(t, f.repo.SaveCoupon(ctx, c))
	c.Enabled = true
	require.NoError(t, f.repo.SaveCoupon(ctx, c))
	_, err = f.repo.Fulfill(ctx, e, "unused-order", "unused-code")
	require.ErrorIs(t, err, service.ErrWelfareAlreadyClaimed)
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.uid).Scan(&balance))
	require.InDelta(t, 0.75, balance, 1e-8)
	// The same public code remains usable by a different website account.
	other := newWelfareFixture(t)
	otherEvent := other.event
	otherEvent.Action, otherEvent.Code = "claim", c.Code
	_, err = other.repo.Fulfill(ctx, otherEvent, "second-account-order", "second-account-redeem")
	require.NoError(t, err)
	var claimed int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT claimed FROM qq_welfare_coupons WHERE id=$1`, c.ID).Scan(&claimed))
	require.Equal(t, 2, claimed)
}
func TestQQWelfareRollbackAndExpiredBinding(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := context.Background()
	_, _, err := f.repo.CheckIn(ctx, f.event)
	require.NoError(t, err)
	p := &service.QQWelfareProduct{Title: f.prefix, PointsCost: 2, BalanceValue: 1, Stock: 1, Enabled: true}
	require.NoError(t, f.repo.SaveProduct(ctx, p))
	e := f.event
	e.Action = "exchange"
	e.ProductID = p.ID
	_, err = f.repo.Fulfill(ctx, e, "no-points", "no-points-code")
	require.ErrorIs(t, err, service.ErrWelfarePoints)
	p.PointsCost = 1
	require.NoError(t, f.repo.SaveProduct(ctx, p))
	_, err = f.db.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,created_at) VALUES($1,'balance',1,'unused',now())`, f.prefix)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := f.db.ExecContext(ctx, `DELETE FROM redeem_codes WHERE code=$1`, f.prefix)
		require.NoError(t, err)
	})
	_, err = f.repo.Fulfill(ctx, e, "conflict-order", f.prefix)
	require.Error(t, err)
	var points, stock int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT points FROM qq_welfare_accounts WHERE user_id=$1`, f.uid).Scan(&points))
	require.Equal(t, 1, points)
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT stock FROM qq_welfare_rewards WHERE id=$1`, p.ID).Scan(&stock))
	require.Equal(t, 1, stock)
	require.ErrorIs(t, f.repo.Verify(ctx, e, service.WelfareHash("binding-token")), service.ErrWelfareBinding)
	require.ErrorIs(t, f.repo.BindGroup(ctx, service.WelfareHash("group-token"), "stolen"), service.ErrWelfareBinding)
	_, err = f.db.ExecContext(ctx, `INSERT INTO qq_welfare_bind_codes(user_id,code_hash,expires_at) VALUES($1,$2,now()-interval '1 minute')`, f.uid, service.WelfareHash("expired-token"))
	require.NoError(t, err)
	require.ErrorIs(t, f.repo.Verify(ctx, e, service.WelfareHash("expired-token")), service.ErrWelfareBinding)
	require.NoError(t, f.repo.GroupCode(ctx, service.WelfareHash("expired-group"), time.Now().Add(-time.Minute)))
	require.ErrorIs(t, f.repo.BindGroup(ctx, service.WelfareHash("expired-group"), "wrong-group"), service.ErrWelfareBinding)
	require.NoError(t, f.repo.UpdateConfig(ctx, true, "123456789"))
	_, _, err = f.repo.CheckIn(ctx, e)
	require.ErrorIs(t, err, service.ErrWelfareUnavailable)
}

func TestQQWelfareDifferentAccountsCompeteForLastRewardAndCoupon(t *testing.T) {
	first, second := newWelfareFixture(t), newWelfareFixture(t)
	ctx := context.Background()
	for _, f := range []*welfareFixture{first, second} {
		_, _, err := f.repo.CheckIn(ctx, f.event)
		require.NoError(t, err)
	}
	p := &service.QQWelfareProduct{Title: first.prefix, PointsCost: 1, BalanceValue: 0.25, Stock: 1, Enabled: true}
	require.NoError(t, first.repo.SaveProduct(ctx, p))
	c := &service.QQWelfareCoupon{Title: first.prefix, Code: "LAST_" + fmt.Sprint(first.uid), BalanceValue: 0.5, MaxClaims: 1, Enabled: true}
	require.NoError(t, first.repo.SaveCoupon(ctx, c))
	for _, action := range []string{"exchange", "claim"} {
		start := make(chan struct{})
		errs := make(chan error, 2)
		for i, f := range []*welfareFixture{first, second} {
			go func(i int, f *welfareFixture) {
				<-start
				e := f.event
				e.Action, e.MessageID, e.ProductID, e.Code = action, action, p.ID, c.Code
				_, err := f.repo.Fulfill(ctx, e, fmt.Sprintf("race-%s-%d", action, i), fmt.Sprintf("race-code-%s-%d", action, i))
				errs <- err
			}(i, f)
		}
		close(start)
		successes, soldOut := 0, 0
		for range 2 {
			err := <-errs
			if err == nil {
				successes++
			} else {
				require.ErrorIs(t, err, service.ErrWelfareSoldOut)
				soldOut++
			}
		}
		require.Equal(t, 1, successes)
		require.Equal(t, 1, soldOut)
	}
	var total float64
	require.NoError(t, first.db.QueryRowContext(ctx, `SELECT sum(balance) FROM users WHERE id IN ($1,$2)`, first.uid, second.uid).Scan(&total))
	require.InDelta(t, 0.75, total, 1e-8)
}

func TestQQWelfareConcurrentDistinctMessagesClaimOnce(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := context.Background()
	c := &service.QQWelfareCoupon{Title: f.prefix, Code: "PARALLEL_" + fmt.Sprint(f.uid), BalanceValue: 0.5, Enabled: true}
	require.NoError(t, f.repo.SaveCoupon(ctx, c))
	start := make(chan struct{})
	errs := make(chan error, 6)
	for i := range 6 {
		go func(i int) {
			<-start
			e := f.event
			e.Action = "claim"
			e.Code = c.Code
			e.MessageID = fmt.Sprint(i)
			_, err := f.repo.Fulfill(ctx, e, fmt.Sprintf("parallel-order-%d", i), fmt.Sprintf("parallel-code-%d", i))
			errs <- err
		}(i)
	}
	close(start)
	successes := 0
	for range 6 {
		err := <-errs
		if err == nil {
			successes++
		} else {
			require.ErrorIs(t, err, service.ErrWelfareAlreadyClaimed)
		}
	}
	require.Equal(t, 1, successes)
}
