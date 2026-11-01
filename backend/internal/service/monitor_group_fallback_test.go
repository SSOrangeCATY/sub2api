//go:build unit

package service

import (
	"context"
	"errors"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type monitorHealthStub struct {
	statuses map[int64]GroupHealthStatus
	err      error
}

func (s *monitorHealthStub) GetGroupHealth(_ context.Context, id int64, _ string) (GroupHealthSnapshot, error) {
	return GroupHealthSnapshot{Status: s.statuses[id]}, s.err
}
func TestMonitorGroupOrderedRouting(t *testing.T) {
	for _, tc := range []struct {
		name     string
		statuses map[int64]GroupHealthStatus
		target   int64
	}{
		{"primary", map[int64]GroupHealthStatus{1: GroupHealthOperational}, 1},
		{"secondary", map[int64]GroupHealthStatus{1: GroupHealthFailed, 2: GroupHealthOperational}, 2},
		{"third", map[int64]GroupHealthStatus{1: GroupHealthFailed, 2: GroupHealthError, 3: GroupHealthOperational}, 3},
		{"exhausted", map[int64]GroupHealthStatus{1: GroupHealthFailed, 2: GroupHealthError, 3: GroupHealthFailed}, 0},
		{"unknown", map[int64]GroupHealthStatus{1: GroupHealthUnknown}, 1},
		{"degraded", map[int64]GroupHealthStatus{1: GroupHealthDegraded}, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mockGroupRepoForGateway{groups: map[int64]*Group{}}
			for _, id := range []int64{1, 2, 3} {
				repo.groups[id] = &Group{ID: id, Name: fmtGroupHealthKey(id, ""), Platform: PlatformOpenAI, Status: StatusActive, GroupType: GroupTypeBusiness}
			}
			entry := &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{1, 2, 3}}
			route, err := NewMonitorGroupRouter(repo, &monitorHealthStub{statuses: tc.statuses}).Resolve(context.Background(), entry, "model")
			require.NoError(t, err)
			if tc.target == 0 {
				require.Nil(t, route.Target)
				require.Equal(t, "unavailable", route.Status)
			} else {
				require.Equal(t, tc.target, route.Target.ID)
			}
		})
	}
}
func TestMonitorGroupRecoveryAndInfrastructureFailure(t *testing.T) {
	repo := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive}, 2: {ID: 2, Platform: PlatformOpenAI, Status: StatusActive}}}
	health := &monitorHealthStub{statuses: map[int64]GroupHealthStatus{1: GroupHealthFailed, 2: GroupHealthOperational}}
	entry := &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{1, 2}}
	router := NewMonitorGroupRouter(repo, health)
	route, err := router.Resolve(context.Background(), entry, "")
	require.NoError(t, err)
	require.Equal(t, int64(2), route.Target.ID)
	health.statuses[1] = GroupHealthOperational
	route, err = router.Resolve(context.Background(), entry, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), route.Target.ID)
	health.err = errors.New("database unavailable")
	route, err = router.Resolve(context.Background(), entry, "")
	require.NoError(t, err)
	require.Equal(t, int64(1), route.Target.ID)
	require.Equal(t, "monitor_unavailable", route.Candidates[0].Reason)
	require.Equal(t, GroupTypeFallback, entry.GroupType)
}
func TestMonitorGroupRuntimeSkipsInvalidTargets(t *testing.T) {
	repo := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Platform: PlatformGemini, Status: StatusActive}, 2: {ID: 2, Platform: PlatformOpenAI, Status: "inactive"}, 3: {ID: 3, Platform: PlatformOpenAI, Status: StatusActive, GroupType: GroupTypeFallback}, 4: {ID: 4, Platform: PlatformOpenAI, Status: StatusActive}}}
	entry := &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{10, 1, 1, 2, 3, 999, 4}}
	route, err := NewMonitorGroupRouter(repo, &monitorHealthStub{}).Resolve(context.Background(), entry, "")
	require.NoError(t, err)
	require.Equal(t, int64(4), route.Target.ID)
}
func TestMonitorGroupChainValidation(t *testing.T) {
	repo := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive, GroupType: GroupTypeBusiness}, 2: {ID: 2, Platform: PlatformGemini, Status: StatusActive}, 3: {ID: 3, Platform: PlatformOpenAI, Status: "inactive"}, 4: {ID: 4, Platform: PlatformOpenAI, Status: StatusActive, GroupType: GroupTypeFallback}, 5: {ID: 5, Platform: PlatformOpenAI, Status: StatusActive, SubscriptionType: SubscriptionTypeSubscription}}}
	svc := &adminServiceImpl{groupRepo: repo}
	for _, tc := range []struct {
		name, kind string
		ids        []int64
		invalid    bool
	}{
		{"valid", GroupTypeFallback, []int64{1}, false}, {"business", GroupTypeBusiness, nil, false}, {"business chain", GroupTypeBusiness, []int64{1}, true}, {"empty", GroupTypeFallback, nil, true}, {"self", GroupTypeFallback, []int64{10}, true}, {"duplicate", GroupTypeFallback, []int64{1, 1}, true}, {"platform", GroupTypeFallback, []int64{2}, true}, {"inactive", GroupTypeFallback, []int64{3}, true}, {"recursive", GroupTypeFallback, []int64{4}, true}, {"subscription", GroupTypeFallback, []int64{5}, true}, {"missing", GroupTypeFallback, []int64{999}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.validateMonitorFallbackChain(context.Background(), 10, PlatformOpenAI, tc.kind, tc.ids)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestMonitorGroupModelAndStaleness(t *testing.T) {
	now := time.Now()
	old := now.Add(-10 * time.Minute)
	require.True(t, monitorLatestExpired(old, 30))
	require.False(t, monitorLatestExpired(now, 30))
	require.True(t, monitorLatestExpired(time.Time{}, 30))
	entries := []*ChannelMonitorLatest{{Model: "a", Status: "failed", CheckedAt: now}, {Model: "b", Status: "operational", CheckedAt: now}}
	require.Equal(t, "b", latestForMonitorModel(entries, "a", nil, "b").Model)
	require.Nil(t, latestForMonitorModel(entries, "a", nil, "unmonitored"))
	require.Equal(t, GroupHealthUnknown, aggregateGroupHealth([]GroupHealthSnapshot{{Status: GroupHealthFailed}, {Status: GroupHealthUnknown}}).Status)
	require.Equal(t, GroupHealthOperational, aggregateGroupHealth([]GroupHealthSnapshot{{Status: GroupHealthFailed}, {Status: GroupHealthOperational}}).Status)
	require.Equal(t, GroupHealthFailed, aggregateGroupHealth([]GroupHealthSnapshot{{Status: GroupHealthFailed}, {Status: GroupHealthError}}).Status)
}
func TestMonitorGroupKeyCloneAndBillingTarget(t *testing.T) {
	target := &Group{ID: 1, Name: "A", Platform: PlatformOpenAI, Status: StatusActive, RateMultiplier: 0.35}
	entry := &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{1}, RateMultiplier: 0.075}
	repo := &mockGroupRepoForGateway{groups: map[int64]*Group{1: target, 10: entry}}
	svc := &APIKeyService{groupRepo: repo, monitorGroupRouter: NewMonitorGroupRouter(repo, &monitorHealthStub{})}
	key := &APIKey{ID: 20, GroupID: &entry.ID, Group: entry, User: &User{ID: 21}}
	routed, route, err := svc.ResolveMonitorFallbackKey(context.Background(), key, "model")
	require.NoError(t, err)
	require.NotSame(t, key, routed)
	require.NotSame(t, key.User, routed.User)
	require.Equal(t, int64(10), *key.GroupID)
	require.Equal(t, int64(1), *routed.GroupID)
	require.Equal(t, 0.35, routed.Group.RateMultiplier)
	require.Equal(t, int64(10), route.EntryGroupID)
}

func TestMonitorGroupUnchangedBrokenChainDoesNotPreventDisablingEntry(t *testing.T) {
	entry := &Group{ID: 10, Platform: PlatformOpenAI, Status: StatusActive, GroupType: GroupTypeFallback, FallbackGroupIDs: []int64{999}, RateMultiplier: 1, SubscriptionType: SubscriptionTypeStandard, Hydrated: true}
	repo := &mockGroupRepoForGateway{groups: map[int64]*Group{10: entry}}
	svc := &adminServiceImpl{groupRepo: repo}
	same := []int64{999}
	kind := GroupTypeFallback
	updated, err := svc.UpdateGroup(context.Background(), 10, &UpdateGroupInput{Status: "inactive", GroupType: &kind, FallbackGroupIDs: &same})
	require.NoError(t, err)
	require.Equal(t, "inactive", updated.Status)
	changed := []int64{998}
	_, err = svc.UpdateGroup(context.Background(), 10, &UpdateGroupInput{FallbackGroupIDs: &changed})
	require.Error(t, err)
}
