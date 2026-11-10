//go:build integration || welfareintegration

package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func welfareEntClient(f *welfareFixture) *dbent.Client {
	return dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, f.db)))
}
func welfareAdminService(f *welfareFixture, client *dbent.Client, receipts service.RedeemCodeRepository) service.AdminService {
	return service.NewAdminService(nil, NewUserRepository(client, f.db), nil, nil, nil, nil, receipts, nil, nil, nil, nil, nil, nil, client, nil, nil, nil, nil, nil, nil, nil, nil, nil)
}
func welfarePaidTotal(t *testing.T, f *welfareFixture) float64 {
	t.Helper()
	var total float64
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT COALESCE(sum(amount),0) FROM paid_balance_credits WHERE user_id=$1`, f.uid).Scan(&total))
	return total
}
func welfareTestCode(t *testing.T, f *welfareFixture, repo service.RedeemCodeRepository, source string, value float64) *service.RedeemCode {
	t.Helper()
	code, err := service.GenerateRedeemCode()
	require.NoError(t, err)
	rc := &service.RedeemCode{Code: code, Type: service.RedeemTypeBalance, Value: value, Status: service.StatusUnused, BalanceSource: source}
	require.NoError(t, repo.Create(t.Context(), rc))
	t.Cleanup(func() {
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE id=$1`, rc.ID)
		require.NoError(t, err)
	})
	return rc
}

func TestQQPaidCreditSourcesThresholdReplayAndDetails(t *testing.T) {
	f, other := newWelfareFixture(t), newWelfareFixture(t)
	ctx := t.Context()
	client := welfareEntClient(f)
	receipts := NewRedeemCodeRepository(client)
	admin := welfareAdminService(f, client, receipts)
	redeem := service.NewRedeemService(receipts, NewUserRepository(client, f.db), nil, nil, nil, client, nil, nil)
	_, err := admin.UpdateUserBalance(ctx, f.uid, 100, "add", "gift", service.BalanceSourceGift)
	require.NoError(t, err)
	require.Zero(t, welfarePaidTotal(t, f))
	_, err = admin.UpdateUserBalance(ctx, f.uid, 4, "add", "paid", service.BalanceSourcePaid)
	require.NoError(t, err)
	require.Equal(t, 4.0, welfarePaidTotal(t, f))
	code := welfareTestCode(t, f, receipts, service.BalanceSourcePaid, 6)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := redeem.Redeem(ctx, f.uid, code.Code)
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, service.ErrRedeemCodeUsed) {
				t.Errorf("concurrent redemption: %v", err)
			}
		}()
	}
	wg.Wait()
	require.EqualValues(t, 1, wins.Load())
	require.Equal(t, 10.0, welfarePaidTotal(t, f))
	gift := welfareTestCode(t, f, receipts, service.BalanceSourceGift, 20)
	_, err = redeem.Redeem(ctx, f.uid, gift.Code)
	require.NoError(t, err)
	// Unclassified historical receipts never silently establish payment eligibility.
	_, err = f.db.ExecContext(ctx, `INSERT INTO redeem_codes(code,type,value,status,used_by) VALUES($1,'balance',500,'used',$2)`, f.prefix, f.uid)
	require.NoError(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE users SET balance=0,username='Paid User' WHERE id=$1`, f.uid)
	require.NoError(t, err)
	require.Equal(t, 10.0, welfarePaidTotal(t, f), "spending/gifts/history do not change lifetime paid credits")
	_, err = f.db.ExecContext(ctx, `UPDATE redeem_codes SET balance_source='paid' WHERE id=$1`, gift.ID)
	require.Error(t, err)
	_, err = f.db.ExecContext(ctx, `UPDATE redeem_codes SET value=600 WHERE id=$1`, code.ID)
	require.Error(t, err)
	coupon := &service.QQWelfareCoupon{Code: fmt.Sprintf("PAID_%d", f.uid), Title: f.prefix, BalanceValue: 1, Enabled: true, MinPaidBalance: 10.00000001}
	require.NoError(t, f.repo.SaveCoupon(ctx, coupon))
	e := f.event
	e.Action = "claim"
	e.Code = coupon.Code
	_, err = f.repo.Fulfill(ctx, e, "paid-order-"+fmt.Sprint(f.uid), "paid-redeem-"+fmt.Sprint(f.uid))
	require.Equal(t, "WELFARE_PAID_BALANCE_REQUIRED", infraerrors.Reason(err))
	var claimed int
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT claimed FROM qq_welfare_coupons WHERE id=$1`, coupon.ID).Scan(&claimed))
	require.Zero(t, claimed)
	coupon.MinPaidBalance = 10
	require.NoError(t, f.repo.SaveCoupon(ctx, coupon))
	order, err := f.repo.Fulfill(ctx, e, "paid-order-"+fmt.Sprint(f.uid), "paid-redeem-"+fmt.Sprint(f.uid))
	require.NoError(t, err)
	coupon.MinPaidBalance = 100
	require.NoError(t, f.repo.SaveCoupon(ctx, coupon))
	replay, err := f.repo.Fulfill(ctx, e, "ignored-order", "ignored-code")
	require.NoError(t, err)
	require.Equal(t, order.ID, replay.ID)
	// Qualification belongs to a website account, not the campaign or QQ group.
	otherEvent := other.event
	otherEvent.Action = "claim"
	otherEvent.Code = coupon.Code
	_, err = other.repo.Fulfill(ctx, otherEvent, "other-order", "other-code")
	require.Equal(t, "WELFARE_PAID_BALANCE_REQUIRED", infraerrors.Reason(err))
	_, err = f.db.ExecContext(ctx, `DELETE FROM redeem_codes WHERE used_by=$1 AND balance_source='paid'`, f.uid)
	require.NoError(t, err)
	require.Equal(t, 10.0, welfarePaidTotal(t, f), "deleting receipts must not erase paid qualification")
	snap, err := f.repo.Snapshot(ctx, f.event)
	require.NoError(t, err)
	require.Equal(t, 10.0, snap.PaidBalanceTotal)
	payload, err := json.Marshal(snap)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "email")
	require.NotContains(t, string(payload), "username")
	details, err := f.repo.CouponClaims(ctx, coupon.ID, 1, 1)
	require.NoError(t, err)
	require.EqualValues(t, 1, details.Total)
	require.Len(t, details.Orders, 1)
	require.Equal(t, "Paid User", details.Orders[0].Username)
	require.Equal(t, f.prefix+"@example.invalid", details.Orders[0].Email)
	require.Equal(t, coupon.Code, details.Orders[0].CouponCode)
	page2, err := f.repo.CouponClaims(ctx, coupon.ID, 2, 1)
	require.NoError(t, err)
	require.Empty(t, page2.Orders)
	require.Equal(t, details.Total, page2.Total)
}

type failingWelfareReceiptRepo struct{ service.RedeemCodeRepository }

func (f failingWelfareReceiptRepo) Create(context.Context, *service.RedeemCode) error {
	return errors.New("injected receipt storage failure")
}

type failingWelfareBalanceRepo struct{ service.UserRepository }

func (f failingWelfareBalanceRepo) UpdateBalance(context.Context, int64, float64) error {
	return errors.New("injected balance storage failure")
}

func TestQQPaidCreditTransactionRollbackAndConcurrentAdminRecharge(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := t.Context()
	client := welfareEntClient(f)
	receipts := NewRedeemCodeRepository(client)
	admin := welfareAdminService(f, client, failingWelfareReceiptRepo{receipts})
	_, err := admin.UpdateUserBalance(ctx, f.uid, 5, "add", "", service.BalanceSourcePaid)
	require.ErrorContains(t, err, "injected receipt")
	var balance float64
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.uid).Scan(&balance))
	require.Zero(t, balance)
	require.Zero(t, welfarePaidTotal(t, f))
	code := welfareTestCode(t, f, receipts, service.BalanceSourcePaid, 5)
	redeem := service.NewRedeemService(receipts, failingWelfareBalanceRepo{NewUserRepository(client, f.db)}, nil, nil, nil, client, nil, nil)
	_, err = redeem.Redeem(ctx, f.uid, code.Code)
	require.ErrorContains(t, err, "injected balance")
	stored, err := receipts.GetByID(ctx, code.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusUnused, stored.Status)
	require.Zero(t, welfarePaidTotal(t, f))
	admin = welfareAdminService(f, client, receipts)
	for _, op := range []string{"set", "subtract"} {
		_, err = admin.UpdateUserBalance(ctx, f.uid, 5, op, "", service.BalanceSourcePaid)
		require.Error(t, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := admin.UpdateUserBalance(ctx, f.uid, 1, "add", "", service.BalanceSourcePaid)
			if err != nil {
				t.Errorf("admin recharge: %v", err)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, 8.0, welfarePaidTotal(t, f))
	require.NoError(t, f.db.QueryRowContext(ctx, `SELECT balance FROM users WHERE id=$1`, f.uid).Scan(&balance))
	require.Equal(t, 8.0, balance)
}

func TestQQAPIGateIndependentOfWelfareAndCurrentGroup(t *testing.T) {
	f := newWelfareFixture(t)
	ctx := t.Context()
	users := NewUserRepository(welfareEntClient(f), f.db).(*userRepository)
	require.NoError(t, users.CheckQQAPIAccess(ctx, f.uid))
	require.NoError(t, f.repo.UpdateConfig(ctx, false, "487375322", true))
	require.NoError(t, users.CheckQQAPIAccess(ctx, f.uid), "bound users qualify while welfare is off")
	_, _, err := f.repo.CheckIn(ctx, f.event)
	require.ErrorIs(t, err, service.ErrWelfareUnavailable)
	_, err = f.db.ExecContext(ctx, `DELETE FROM qq_welfare_accounts WHERE user_id=$1`, f.uid)
	require.NoError(t, err)
	require.Equal(t, "QQ_VERIFICATION_REQUIRED", infraerrors.Reason(users.CheckQQAPIAccess(ctx, f.uid)))
	binding, err := f.repo.Binding(ctx, f.uid)
	require.NoError(t, err)
	require.True(t, binding.Ready)
	require.False(t, binding.Verified)
	require.NoError(t, f.repo.CreateBinding(ctx, f.uid, service.WelfareHash("gate-binding"), time.Now().Add(time.Minute)))
	require.NoError(t, f.repo.Verify(ctx, f.event, service.WelfareHash("gate-binding")))
	require.NoError(t, users.CheckQQAPIAccess(ctx, f.uid))
	require.NoError(t, f.repo.UpdateConfig(ctx, false, "123456789", true))
	require.Equal(t, "QQ_VERIFICATION_REQUIRED", infraerrors.Reason(users.CheckQQAPIAccess(ctx, f.uid)), "changing the group takes effect immediately")
	require.NoError(t, f.repo.UpdateConfig(ctx, false, "123456789", false))
	require.NoError(t, users.CheckQQAPIAccess(ctx, f.uid))
}

func TestQQPaidReceiptAllowsRecipientDeletion(t *testing.T) {
	f := newWelfareFixture(t)
	client := welfareEntClient(f)
	admin := welfareAdminService(f, client, NewRedeemCodeRepository(client))
	_, err := admin.UpdateUserBalance(t.Context(), f.uid, 2, "add", f.prefix, service.BalanceSourcePaid)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := f.db.ExecContext(context.Background(), `DELETE FROM redeem_codes WHERE notes=$1`, f.prefix)
		require.NoError(t, err)
	})
	_, err = f.db.ExecContext(t.Context(), `DELETE FROM users WHERE id=$1`, f.uid)
	require.NoError(t, err)
	require.Zero(t, welfarePaidTotal(t, f))
	var orphaned int
	require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT count(*) FROM redeem_codes WHERE notes=$1 AND used_by IS NULL`, f.prefix).Scan(&orphaned))
	require.Equal(t, 1, orphaned)
}

func TestQQPaidSmallRechargeAtLargeBalanceIsExact(t *testing.T) {
	for _, tc := range []struct{ balance, deposit float64 }{{100000000, 0.01}, {200000000, 0.00000001}} {
		t.Run(fmt.Sprintf("%g_%g", tc.balance, tc.deposit), func(t *testing.T) {
			f := newWelfareFixture(t)
			client := welfareEntClient(f)
			admin := welfareAdminService(f, client, NewRedeemCodeRepository(client))
			_, err := f.db.ExecContext(t.Context(), `UPDATE users SET balance=$1 WHERE id=$2`, tc.balance, f.uid)
			require.NoError(t, err)
			_, err = admin.UpdateUserBalance(t.Context(), f.uid, tc.deposit, "add", "", service.BalanceSourcePaid)
			require.NoError(t, err)
			require.Equal(t, tc.deposit, welfarePaidTotal(t, f))
			var exact bool
			require.NoError(t, f.db.QueryRowContext(t.Context(), `SELECT balance=$1::numeric+$2::numeric FROM users WHERE id=$3`, tc.balance, tc.deposit, f.uid).Scan(&exact))
			require.True(t, exact)
		})
	}
}

type pausedWelfareReceiptRepo struct {
	service.RedeemCodeRepository
	created chan struct{}
	release chan struct{}
	fail    bool
}

func (r *pausedWelfareReceiptRepo) Create(ctx context.Context, receipt *service.RedeemCode) error {
	if err := r.RedeemCodeRepository.Create(ctx, receipt); err != nil {
		return err
	}
	close(r.created)
	select {
	case <-r.release:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.fail {
		return errors.New("rollback after receipt insert")
	}
	return nil
}

func TestQQPaidQualificationWaitsForRechargeCommit(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(fmt.Sprintf("rollback_%t", fail), func(t *testing.T) {
			f := newWelfareFixture(t)
			client := welfareEntClient(f)
			paused := &pausedWelfareReceiptRepo{RedeemCodeRepository: NewRedeemCodeRepository(client), created: make(chan struct{}), release: make(chan struct{}), fail: fail}
			var once sync.Once
			release := func() { once.Do(func() { close(paused.release) }) }
			defer release()
			admin := welfareAdminService(f, client, paused)
			coupon := &service.QQWelfareCoupon{Code: fmt.Sprintf("WAIT_%d", f.uid), Title: f.prefix, BalanceValue: 1, Enabled: true, MinPaidBalance: 10}
			require.NoError(t, f.repo.SaveCoupon(t.Context(), coupon))
			adminDone := make(chan error, 1)
			go func() {
				_, err := admin.UpdateUserBalance(t.Context(), f.uid, 10, "add", "", service.BalanceSourcePaid)
				adminDone <- err
			}()
			select {
			case <-paused.created:
			case <-time.After(3 * time.Second):
				t.Fatal("recharge did not reach receipt insert")
			}
			require.Zero(t, welfarePaidTotal(t, f), "uncommitted paid receipt is invisible")
			e := f.event
			e.Action = "claim"
			e.Code = coupon.Code
			claimDone := make(chan error, 1)
			go func() {
				_, err := f.repo.Fulfill(t.Context(), e, fmt.Sprintf("wait-%d", f.uid), fmt.Sprintf("wait-code-%d", f.uid))
				claimDone <- err
			}()
			select {
			case err := <-claimDone:
				t.Fatalf("claim passed the held user lock before commit: %v", err)
			case <-time.After(100 * time.Millisecond):
			}
			release()
			if fail {
				require.ErrorContains(t, <-adminDone, "rollback after receipt")
				require.Equal(t, "WELFARE_PAID_BALANCE_REQUIRED", infraerrors.Reason(<-claimDone))
				require.Zero(t, welfarePaidTotal(t, f))
			} else {
				require.NoError(t, <-adminDone)
				require.NoError(t, <-claimDone)
				require.Equal(t, 10.0, welfarePaidTotal(t, f))
			}
		})
	}
}
