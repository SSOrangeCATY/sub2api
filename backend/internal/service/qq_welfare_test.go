package service

import (
	"context"
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type welfareRepoStub struct {
	QQWelfareRepository
	cfg   QQWelfareConfig
	saved bool
	hash  string
}

func (r *welfareRepoStub) Config(context.Context) (*QQWelfareConfig, error) { return &r.cfg, nil }
func (r *welfareRepoStub) SaveCoupon(_ context.Context, p *QQWelfareCoupon) error {
	r.saved = true
	return nil
}
func (r *welfareRepoStub) SaveProduct(_ context.Context, p *QQWelfareProduct) error {
	r.saved = true
	return nil
}
func (r *welfareRepoStub) CreateBinding(_ context.Context, _ int64, hash string, _ time.Time) error {
	r.hash = hash
	return nil
}
func TestQQWelfareValidationAndSecretBoundary(t *testing.T) {
	ctx := context.Background()
	repo := &welfareRepoStub{cfg: QQWelfareConfig{BotKey: "private-key"}}
	svc := NewQQWelfareService(repo, nil, nil)
	require.Error(t, svc.Authenticate(ctx, ""))
	require.Error(t, svc.Authenticate(ctx, "wrong"))
	require.NoError(t, svc.Authenticate(ctx, "private-key"))
	encoded, err := json.Marshal(repo.cfg)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), "private-key")
	code, err := svc.BindingCode(ctx, 1)
	require.NoError(t, err)
	require.Len(t, code, 32)
	require.Len(t, repo.hash, 64)
	require.NotEqual(t, code, repo.hash)
	for _, value := range []float64{0, -1, math.NaN(), math.Inf(1), 10001, 0.000000001, 1.00000000001} {
		require.Error(t, svc.SaveProduct(ctx, &QQWelfareProduct{Title: "test", BalanceValue: value, PointsCost: 1, Stock: 1}))
	}
	require.False(t, repo.saved)
	require.Error(t, svc.SaveCoupon(ctx, &QQWelfareCoupon{Title: "test", Code: "bad code", BalanceValue: 1}))
	p := &QQWelfareCoupon{Title: " test ", Code: " welcome_1 ", BalanceValue: 1}
	require.NoError(t, svc.SaveCoupon(ctx, p))
	require.Equal(t, "WELCOME_1", p.Code)
	past := time.Now().Add(-time.Hour)
	require.Error(t, svc.SaveProduct(ctx, &QQWelfareProduct{Title: "test", BalanceValue: 1, PointsCost: 1, ExpiresAt: &past, Enabled: true}))
	require.NoError(t, svc.SaveProduct(ctx, &QQWelfareProduct{Title: "test", BalanceValue: 1, PointsCost: 1, ExpiresAt: &past, Enabled: false}))
}
