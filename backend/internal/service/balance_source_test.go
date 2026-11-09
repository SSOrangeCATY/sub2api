package service

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBalanceSourceValidation(t *testing.T) {
	for _, typ := range []string{RedeemTypeBalance, AdjustmentTypeAdminBalance} {
		source, err := NormalizeBalanceSource(BalanceSourcePaid, typ, 10)
		require.NoError(t, err)
		require.Equal(t, BalanceSourcePaid, source)
	}
	source, err := NormalizeBalanceSource("", RedeemTypeBalance, 10)
	require.NoError(t, err)
	require.Equal(t, BalanceSourceGift, source)
	for _, tc := range []struct {
		source, typ string
		value       float64
	}{
		{"paid", RedeemTypeBalance, 0}, {"paid", RedeemTypeBalance, -1}, {"paid", RedeemTypeConcurrency, 10}, {"paid", RedeemTypeSubscription, 10}, {"paid", RedeemTypeBalance, math.Inf(1)}, {"payment", RedeemTypeBalance, 10},
	} {
		_, err := NormalizeBalanceSource(tc.source, tc.typ, tc.value)
		require.Error(t, err)
	}
}

func TestQQWelfarePaidThresholdValidation(t *testing.T) {
	repo := &welfareRepoStub{}
	svc := NewQQWelfareService(repo, nil, nil)
	for _, v := range []float64{-1, math.NaN(), math.Inf(1), 100000001, 0.000000001} {
		repo.saved = false
		err := svc.SaveCoupon(t.Context(), &QQWelfareCoupon{Code: "PAID", Title: "Coupon", BalanceValue: 1, MinPaidBalance: v})
		require.Error(t, err)
		require.False(t, repo.saved)
	}
	for _, v := range []float64{0, 10, 0.00000001} {
		require.NoError(t, svc.SaveCoupon(t.Context(), &QQWelfareCoupon{Code: "PAID", Title: "Coupon", BalanceValue: 1, MinPaidBalance: v}))
	}
}
