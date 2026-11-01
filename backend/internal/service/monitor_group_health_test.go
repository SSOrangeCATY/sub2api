package service

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestMonitorGroupAuthSnapshotRoundTrip(t *testing.T) {
	svc := &APIKeyService{}
	id := int64(10)
	key := &APIKey{ID: 20, UserID: 21, GroupID: &id, Status: StatusActive, User: &User{ID: 21, Status: StatusActive}, Group: &Group{ID: id, GroupType: GroupTypeFallback, FallbackGroupIDs: []int64{2, 1}, Platform: PlatformOpenAI, Status: StatusActive}}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: svc.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var entry APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &entry))
	restored, used, err := svc.applyAuthCacheEntry("test-placeholder", &entry)
	require.NoError(t, err)
	require.True(t, used)
	require.Equal(t, GroupTypeFallback, restored.Group.GroupType)
	require.Equal(t, []int64{2, 1}, restored.Group.FallbackGroupIDs)
	entry.Snapshot.Version = 25
	_, used, err = svc.applyAuthCacheEntry("test-placeholder", &entry)
	require.NoError(t, err)
	require.False(t, used)
}

func TestMonitorGroupProviderPlatformCompatibility(t *testing.T) {
	for _, tc := range []struct {
		provider, platform string
		want               bool
	}{
		{MonitorProviderOpenAI, PlatformOpenAI, true},
		{MonitorProviderOpenAI, PlatformKimi, true},
		{MonitorProviderKimi, PlatformOpenAI, true},
		{MonitorProviderOpenAI, PlatformCommandCode, true},
		{MonitorProviderAnthropic, PlatformTypeSafe, true},
		{MonitorProviderAnthropic, PlatformAntigravity, true},
		{MonitorProviderGemini, PlatformAntigravity, true},
		{MonitorProviderAntigravity, PlatformAntigravity, true},
		{MonitorProviderAntigravity, PlatformAnthropic, false},
		{MonitorProviderGemini, PlatformOpenAI, false},
		{MonitorProviderOpenAI, PlatformGemini, false},
		{MonitorProviderOpenAI, PlatformComposite, false},
		{"", PlatformOpenAI, false},
	} {
		t.Run(tc.provider+"/"+tc.platform, func(t *testing.T) {
			require.Equal(t, tc.want, monitorProviderMatchesPlatform(tc.provider, tc.platform))
		})
	}
}

type monitorPassiveHealthStub struct {
	ChannelMonitorV2Repository
	cfg      ChannelMonitorV2Config
	snapshot *ChannelMonitorV2Snapshot
	filter   ChannelMonitorV2Filter
}

func (r *monitorPassiveHealthStub) GetConfig(context.Context) (*ChannelMonitorV2Config, error) {
	return &r.cfg, nil
}
func (r *monitorPassiveHealthStub) GetSnapshot(_ context.Context, f ChannelMonitorV2Filter, _ ChannelMonitorV2Config, _ bool) (*ChannelMonitorV2Snapshot, error) {
	r.filter = f
	return r.snapshot, nil
}
func TestMonitorGroupPassiveHealthUsesErrorSignal(t *testing.T) {
	now := time.Now()
	cfg := ChannelMonitorV2Config{Enabled: true, RefreshIntervalSeconds: 300, HealthThresholds: DefaultChannelMonitorV2HealthThresholds()}
	for _, tc := range []struct {
		name    string
		metrics ChannelMonitorV2Metric
		through time.Time
		want    GroupHealthStatus
	}{
		{"failure", ChannelMonitorV2Metric{RequestCount: 100, ErrorRate: 0.9}, now, GroupHealthFailed},
		{"healthy slow", ChannelMonitorV2Metric{RequestCount: 100, ErrorRate: 0}, now, GroupHealthOperational},
		{"insufficient", ChannelMonitorV2Metric{RequestCount: 2, ErrorRate: 1}, now, GroupHealthUnknown},
		{"stale", ChannelMonitorV2Metric{RequestCount: 100, ErrorRate: 1}, now.Add(-10 * time.Minute), GroupHealthUnknown},
		{"within default refresh", ChannelMonitorV2Metric{RequestCount: 100, ErrorRate: 1}, now.Add(-4 * time.Minute), GroupHealthFailed},
		{"future watermark", ChannelMonitorV2Metric{RequestCount: 100, ErrorRate: 1}, now.Add(10 * time.Minute), GroupHealthUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := &monitorPassiveHealthStub{cfg: cfg, snapshot: &ChannelMonitorV2Snapshot{Metrics: tc.metrics, Coverage: ChannelMonitorV2Coverage{DataThrough: tc.through}}}
			reader := &monitorGroupHealthReader{passive: repo}
			got, err := reader.passiveHealth(context.Background(), 7, "model-a")
			require.NoError(t, err)
			require.Equal(t, tc.want, got.Status)
			require.Equal(t, []int64{7}, repo.filter.GroupIDs)
			require.Equal(t, []string{"model-a"}, repo.filter.Models)
			require.InDelta(t, 300, repo.filter.End.Sub(repo.filter.Start).Seconds(), 1)
		})
	}
}
