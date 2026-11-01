//go:build unit

package service

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type independentMonitorRepo struct {
	ChannelMonitorRepository
	monitors []*ChannelMonitor
	latest   map[int64][]*ChannelMonitorLatest
	err      error
}

type monitorPoolRepo struct {
	AccountRepository
	accounts []Account
}

func (r *monitorPoolRepo) ListSchedulableByGroupID(context.Context, int64) ([]Account, error) {
	return r.accounts, nil
}

func (r *independentMonitorRepo) ListEnabled(context.Context) ([]*ChannelMonitor, error) {
	return r.monitors, r.err
}
func (r *independentMonitorRepo) ListLatestForMonitorIDs(context.Context, []int64) (map[int64][]*ChannelMonitorLatest, error) {
	return r.latest, r.err
}

type independentProbeKeyRepo struct {
	APIKeyRepository
	key *APIKey
}

func (r *independentProbeKeyRepo) GetByKey(context.Context, string) (*APIKey, error) {
	if r.key == nil {
		return nil, ErrAPIKeyNotFound
	}
	return r.key, nil
}
func TestMonitorGroupIndependentProbeRejectsEntryAndMismatchedGroup(t *testing.T) {
	repo := &independentProbeKeyRepo{key: &APIKey{Group: &Group{Name: "A", GroupType: GroupTypeFallback}}}
	svc := &ChannelMonitorService{localKeys: repo}
	require.Error(t, svc.validateIndependentProbe(context.Background(), "test-placeholder", "A"))
	repo.key.Group.GroupType = GroupTypeBusiness
	require.NoError(t, svc.validateIndependentProbe(context.Background(), "test-placeholder", "A"))
	require.Error(t, svc.validateIndependentProbe(context.Background(), "test-placeholder", "B"))
	repo.key = nil
	require.NoError(t, svc.validateIndependentProbe(context.Background(), "external-placeholder", "A"))
}
func TestMonitorGroupV1HealthIndependentExactMatch(t *testing.T) {
	groups := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Name: "A", Platform: PlatformOpenAI}, 2: {ID: 2, Name: "a", Platform: PlatformOpenAI}}}
	monitors := &independentMonitorRepo{monitors: []*ChannelMonitor{{ID: 11, GroupName: "A", Provider: MonitorProviderOpenAI, Enabled: true, PrimaryModel: "model", IntervalSeconds: 30}, {ID: 12, GroupName: "a", Provider: MonitorProviderOpenAI, Enabled: true, PrimaryModel: "model", IntervalSeconds: 30}}, latest: map[int64][]*ChannelMonitorLatest{11: {{Model: "model", Status: "failed", CheckedAt: time.Now()}}, 12: {{Model: "model", Status: "operational", CheckedAt: time.Now()}}}}
	reader := &monitorGroupHealthReader{groups: groups, monitors: monitors}
	a, err := reader.GetGroupHealth(context.Background(), 1, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthFailed, a.Status)
	// An unrelated model monitor on the same group must not mask this failure.
	monitors.monitors = append(monitors.monitors, &ChannelMonitor{ID: 13, GroupName: "A", Provider: MonitorProviderOpenAI, Enabled: true, PrimaryModel: "other", IntervalSeconds: 30})
	monitors.latest[13] = []*ChannelMonitorLatest{{Model: "other", Status: "operational", CheckedAt: time.Now()}}
	a, err = reader.GetGroupHealth(context.Background(), 1, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthFailed, a.Status)
	// A different protocol with the same name and model cannot mask a failure,
	// or turn a healthy business group into a failed one.
	monitors.monitors = append(monitors.monitors, &ChannelMonitor{ID: 14, GroupName: "A", Provider: MonitorProviderGemini, Enabled: true, PrimaryModel: "model", IntervalSeconds: 30})
	monitors.latest[14] = []*ChannelMonitorLatest{{Model: "model", Status: "operational", CheckedAt: time.Now()}}
	a, err = reader.GetGroupHealth(context.Background(), 1, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthFailed, a.Status)
	monitors.latest[11][0].Status = "operational"
	monitors.latest[14][0].Status = "failed"
	a, err = reader.GetGroupHealth(context.Background(), 1, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthOperational, a.Status)
	b, err := reader.GetGroupHealth(context.Background(), 2, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthOperational, b.Status)
	a, err = reader.GetGroupHealth(context.Background(), 1, "unmonitored")
	require.NoError(t, err)
	require.Equal(t, GroupHealthUnknown, a.Status)
	monitors.latest[11][0].CheckedAt = time.Now().Add(-10 * time.Minute)
	a, err = reader.GetGroupHealth(context.Background(), 1, "model")
	require.NoError(t, err)
	require.Equal(t, GroupHealthUnknown, a.Status)
}
func TestMonitorGroupSkipsEmptyPool(t *testing.T) {
	groups := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive}}}
	router := NewMonitorGroupRouter(groups, &monitorHealthStub{})
	router.accounts = &monitorPoolRepo{}
	route, err := router.Resolve(context.Background(), &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{1}}, "")
	require.NoError(t, err)
	require.Nil(t, route.Target)
	require.Equal(t, "no_schedulable_accounts", route.Candidates[0].Reason)
}

func TestMonitorGroupPoolPrecheckDoesNotRejectChannelModelAlias(t *testing.T) {
	groups := &mockGroupRepoForGateway{groups: map[int64]*Group{1: {ID: 1, Platform: PlatformOpenAI, Status: StatusActive}}}
	router := NewMonitorGroupRouter(groups, &monitorHealthStub{})
	router.accounts = &monitorPoolRepo{accounts: []Account{{ID: 1, Platform: PlatformOpenAI, Status: StatusActive, Schedulable: true, Type: AccountTypeAPIKey, Credentials: map[string]any{"model_mapping": map[string]any{"real-model": "real-model"}}}}}
	route, err := router.Resolve(context.Background(), &Group{ID: 10, GroupType: GroupTypeFallback, Platform: PlatformOpenAI, Status: StatusActive, FallbackGroupIDs: []int64{1}}, "public-alias")
	require.NoError(t, err)
	require.NotNil(t, route.Target)
	require.Equal(t, int64(1), route.Target.ID)
}
