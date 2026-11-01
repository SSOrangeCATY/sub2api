package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type monitorTurnResolverStub struct {
	key   *service.APIKey
	err   error
	model string
}

func (s *monitorTurnResolverStub) ResolveMonitorFallbackKey(_ context.Context, _ *service.APIKey, model string) (*service.APIKey, *service.MonitorGroupRoute, error) {
	s.model = model
	return s.key, nil, s.err
}

func TestMonitorGroupWSTurnRefreshesTargetPricesAndRequiresReconnectOnSwitch(t *testing.T) {
	entry := &service.APIKey{ID: 1, Group: &service.Group{ID: 10, GroupType: service.GroupTypeFallback}}
	conn := &service.APIKey{ID: 1, Group: &service.Group{ID: 2, RateMultiplier: 0.075}}
	latest := &service.APIKey{ID: 1, Group: &service.Group{ID: 2, RateMultiplier: 0.35}}
	resolver := &monitorTurnResolverStub{key: latest}
	ctx := context.WithValue(context.Background(), ctxkey.Group, conn.Group)
	key, err := resolveOpenAIWSMonitorTurnKey(ctx, resolver, entry, conn, "model-a")
	require.NoError(t, err)
	require.Same(t, latest, key)
	require.Equal(t, "model-a", resolver.model)
	var turns openAIWSTurnBillingAPIKeys
	turns.set(1, conn)
	turns.set(2, key)
	billingCtx := withOpenAIWSTurnBillingGroup(ctx, turns.forTurn(2, conn))
	billingGroup, ok := billingCtx.Value(ctxkey.Group).(*service.Group)
	require.True(t, ok)
	require.Equal(t, 0.35, billingGroup.RateMultiplier)
	require.Equal(t, 0.075, turns.forTurn(1, conn).Group.RateMultiplier)
	require.Equal(t, 0.075, conn.Group.RateMultiplier)
	resolver.key = &service.APIKey{ID: 1, Group: &service.Group{ID: 3}}
	_, err = resolveOpenAIWSMonitorTurnKey(ctx, resolver, entry, conn, "model-a")
	require.ErrorContains(t, err, "route changed")
	resolver.err = errors.New("all unavailable")
	_, err = resolveOpenAIWSMonitorTurnKey(ctx, resolver, entry, conn, "model-a")
	require.ErrorContains(t, err, "unavailable")
}
