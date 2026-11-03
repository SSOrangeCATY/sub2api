package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type welfareRepoStub struct {
	QQWelfareRepository
	cfg         QQWelfareConfig
	saved       bool
	hash        string
	event       QQWelfareEvent
	identityErr error
}

func (r *welfareRepoStub) UserIdentity(_ context.Context, uid int64) (*QQWelfareEvent, error) {
	if r.identityErr != nil {
		return nil, r.identityErr
	}
	return &QQWelfareEvent{ActorID: fmt.Sprintf("qq-%d", uid), GroupID: "trusted-group"}, nil
}
func (r *welfareRepoStub) TouchAndLimit(_ context.Context, e QQWelfareEvent) error {
	r.event = e
	return nil
}
func (r *welfareRepoStub) Snapshot(context.Context, QQWelfareEvent) (*QQWelfareSnapshot, error) {
	return &QQWelfareSnapshot{Points: 3, Products: []QQWelfareProduct{{Title: "private reward"}}, Orders: []QQWelfareOrder{{ID: "private receipt"}}}, nil
}
func TestQQWelfareWebsiteAndBotPermissions(t *testing.T) {
	ctx := context.Background()
	r := &welfareRepoStub{}
	s := NewQQWelfareService(r, nil, nil)
	for _, action := range []string{"exchange", "claim", "overview", "records"} {
		var e QQWelfareEvent
		require.NoError(t, json.Unmarshal([]byte(`{"action":"`+action+`","is_website":true}`), &e))
		e.IsWebsite = true
		_, err := s.Event(ctx, e)
		require.Error(t, err)
	}
	_, err := s.UserAction(ctx, 7, "verify", "", "", 0, 0)
	require.Error(t, err)
	_, err = s.UserAction(ctx, 7, "exchange", "bad", "", 1, 0)
	require.Error(t, err)
	r.identityErr = ErrWelfareUnverified
	_, err = s.UserAction(ctx, 7, "checkin", "", "", 0, 0)
	require.ErrorIs(t, err, ErrWelfareUnverified)
	r.identityErr = nil
	result, err := s.UserAction(ctx, 7, "overview", "", "", 0, 0)
	require.NoError(t, err)
	require.EqualValues(t, 3, result.Snapshot.Points)
	require.Equal(t, "qq-7", r.event.ActorID)
	require.True(t, r.event.IsWebsite)
	require.Equal(t, "website:", r.event.MessageID)
	result, err = s.Event(ctx, QQWelfareEvent{Action: "points", ActorID: "qq-7", GroupID: "trusted-group", MessageID: "message"})
	require.NoError(t, err)
	require.Nil(t, result.Snapshot)
	require.EqualValues(t, 3, result.Points.Points)
	b, err := json.Marshal(result)
	require.NoError(t, err)
	require.NotContains(t, string(b), "private")
	require.False(t, r.event.IsWebsite)
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
