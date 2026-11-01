package service

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"golang.org/x/sync/singleflight"
)

type GroupHealthStatus string

const (
	GroupHealthOperational GroupHealthStatus = "operational"
	GroupHealthDegraded    GroupHealthStatus = "degraded"
	GroupHealthFailed      GroupHealthStatus = "failed"
	GroupHealthError       GroupHealthStatus = "error"
	GroupHealthUnknown     GroupHealthStatus = "unknown"
)

type GroupHealthSnapshot struct {
	Status    GroupHealthStatus `json:"status"`
	CheckedAt *time.Time        `json:"checked_at,omitempty"`
}
type GroupHealthReader interface {
	GetGroupHealth(context.Context, int64, string) (GroupHealthSnapshot, error)
}
type monitorGroupHealthReader struct {
	groups   GroupRepository
	monitors ChannelMonitorRepository
	passive  ChannelMonitorV2Repository
	settings channelMonitorRuntimeReader
}

func NewMonitorGroupHealthReader(groups GroupRepository, monitors ChannelMonitorRepository) GroupHealthReader {
	return &monitorGroupHealthReader{groups: groups, monitors: monitors}
}
func (r *monitorGroupHealthReader) GetGroupHealth(ctx context.Context, groupID int64, model string) (GroupHealthSnapshot, error) {
	if r.settings != nil {
		runtime := r.settings.GetChannelMonitorRuntime(ctx)
		if !runtime.Enabled {
			return GroupHealthSnapshot{Status: GroupHealthUnknown}, nil
		}
		if runtime.Mode == ChannelMonitorModeV2 {
			return r.passiveHealth(ctx, groupID, model)
		}
	}
	group, err := r.groups.GetByIDLite(ctx, groupID)
	if err != nil {
		return GroupHealthSnapshot{}, err
	}
	monitors, err := r.monitors.ListEnabled(ctx)
	if err != nil {
		return GroupHealthSnapshot{}, err
	}
	matched := make([]*ChannelMonitor, 0)
	ids := make([]int64, 0)
	for _, m := range monitors {
		// Exact matching preserves the database's case-sensitive unique group names.
		// Health checks must probe business targets directly, never the fallback entry.
		if m != nil && m.Enabled && strings.TrimSpace(m.GroupName) == group.Name && monitorProviderMatchesPlatform(m.Provider, group.Platform) && monitorTracksRequestedModel(m, model) {
			matched = append(matched, m)
			ids = append(ids, m.ID)
		}
	}
	if len(ids) == 0 {
		return GroupHealthSnapshot{Status: GroupHealthUnknown}, nil
	}
	latest, err := r.monitors.ListLatestForMonitorIDs(ctx, ids)
	if err != nil {
		return GroupHealthSnapshot{}, err
	}
	snapshots := make([]GroupHealthSnapshot, 0, len(matched))
	for _, m := range matched {
		entry := latestForMonitorModel(latest[m.ID], m.PrimaryModel, m.ExtraModels, model)
		snapshot := GroupHealthSnapshot{Status: GroupHealthUnknown}
		if entry != nil && !monitorLatestExpired(entry.CheckedAt, m.IntervalSeconds+m.JitterSeconds) {
			checked := entry.CheckedAt
			snapshot = GroupHealthSnapshot{Status: GroupHealthStatus(strings.ToLower(strings.TrimSpace(entry.Status))), CheckedAt: &checked}
		}
		snapshots = append(snapshots, snapshot)
	}
	return aggregateGroupHealth(snapshots), nil
}

// A matching name/model alone does not establish protocol compatibility. Providers
// using the same gateway are compatible (including OpenAI-compatible vendors).
// Antigravity serves both Messages and Gemini; its own monitor is quota-only.
func monitorProviderMatchesPlatform(provider, platform string) bool {
	if _, supported := monitorProviders[provider]; !supported {
		return false
	}
	if platform == PlatformAntigravity {
		return provider == MonitorProviderAntigravity || provider == MonitorProviderAnthropic || provider == MonitorProviderGemini
	}
	groupSpec, groupOK := domain.LookupPlatform(platform)
	providerSpec, providerOK := domain.LookupPlatform(provider)
	return groupOK && providerOK && groupSpec.Gateway == providerSpec.Gateway
}

func monitorTracksRequestedModel(m *ChannelMonitor, model string) bool {
	model = strings.TrimSpace(model)
	if model == "" || strings.TrimSpace(m.PrimaryModel) == model {
		return true
	}
	for _, extra := range m.ExtraModels {
		if strings.TrimSpace(extra) == model {
			return true
		}
	}
	return false
}

func aggregateGroupHealth(snapshots []GroupHealthSnapshot) GroupHealthSnapshot {
	result := GroupHealthSnapshot{Status: GroupHealthUnknown}
	failed, operational, degraded := 0, false, false
	for _, s := range snapshots {
		if s.CheckedAt != nil && (result.CheckedAt == nil || s.CheckedAt.After(*result.CheckedAt)) {
			result.CheckedAt = s.CheckedAt
		}
		switch s.Status {
		case GroupHealthOperational:
			operational = true
		case GroupHealthDegraded:
			degraded = true
		case GroupHealthFailed, GroupHealthError:
			failed++
		}
	}
	switch {
	case operational:
		result.Status = GroupHealthOperational
	case degraded:
		result.Status = GroupHealthDegraded
	case len(snapshots) > 0 && failed == len(snapshots):
		result.Status = GroupHealthFailed
	}
	return result
}
func latestForMonitorModel(entries []*ChannelMonitorLatest, primary string, extras []string, requested string) *ChannelMonitorLatest {
	// A failure for another model must not disable the requested model.
	model := strings.TrimSpace(requested)
	if model == "" {
		model = strings.TrimSpace(primary)
	}
	var newest *ChannelMonitorLatest
	for _, entry := range entries {
		if entry != nil && strings.TrimSpace(entry.Model) == model && (newest == nil || entry.CheckedAt.After(newest.CheckedAt)) {
			newest = entry
		}
	}
	return newest
}
func monitorLatestExpired(checkedAt time.Time, intervalSeconds int) bool {
	maxAge := 2 * time.Minute
	if age := time.Duration(intervalSeconds) * 2 * time.Second; age > maxAge {
		maxAge = age
	}
	return checkedAt.IsZero() || checkedAt.After(time.Now().Add(time.Minute)) || time.Since(checkedAt) > maxAge
}
func (r *monitorGroupHealthReader) passiveHealth(ctx context.Context, groupID int64, model string) (GroupHealthSnapshot, error) {
	unknown := GroupHealthSnapshot{Status: GroupHealthUnknown}
	if r.passive == nil {
		return unknown, nil
	}
	cfg, err := r.passive.GetConfig(ctx)
	if err != nil {
		return unknown, err
	}
	if cfg == nil || !cfg.Enabled {
		return unknown, nil
	}
	now := time.Now().UTC()
	filter := ChannelMonitorV2Filter{Range: "90m", GroupIDs: []int64{groupID}, Start: now.Add(-5 * time.Minute), End: now, Bucket: time.Minute}
	if model != "" {
		filter.Models = []string{model}
	}
	snapshot, err := r.passive.GetSnapshot(ctx, filter, *cfg, true)
	if err != nil {
		return unknown, err
	}
	// V2 defaults to five-minute aggregation. Allow one refresh plus a bucket
	// of aggregation lag, but never accept a future or indefinitely stale watermark.
	maxAge := time.Duration(cfg.RefreshIntervalSeconds)*time.Second + time.Minute
	if maxAge < 2*time.Minute {
		maxAge = 2 * time.Minute
	}
	if snapshot == nil || snapshot.Coverage.DataThrough.IsZero() || snapshot.Coverage.DataThrough.After(now.Add(time.Minute)) || now.Sub(snapshot.Coverage.DataThrough) > maxAge {
		return unknown, nil
	}
	// Only error-rate health triggers failover, never cache/TTFT scores alone.
	// A bounded window ages out old evidence so the primary is retried.
	health := ChannelMonitorV2HealthForWithThresholds(snapshot.Metrics, cfg.HealthThresholds)
	checked := snapshot.Coverage.DataThrough
	result := GroupHealthSnapshot{Status: GroupHealthUnknown, CheckedAt: &checked}
	switch health.ErrorRate {
	case "critical":
		result.Status = GroupHealthFailed
	case "warning":
		result.Status = GroupHealthDegraded
	case "healthy":
		result.Status = GroupHealthOperational
	}
	return result, nil
}

type groupHealthCacheEntry struct {
	snapshot GroupHealthSnapshot
	expires  time.Time
}
type cachedGroupHealthReader struct {
	reader  GroupHealthReader
	mu      sync.Mutex
	entries map[string]groupHealthCacheEntry
	flight  singleflight.Group
}

func (r *cachedGroupHealthReader) GetGroupHealth(ctx context.Context, id int64, model string) (GroupHealthSnapshot, error) {
	key := fmtGroupHealthKey(id, model)
	r.mu.Lock()
	cached, ok := r.entries[key]
	r.mu.Unlock()
	if ok && time.Now().Before(cached.expires) {
		return cached.snapshot, nil
	}
	value, err, _ := r.flight.Do(key, func() (any, error) {
		snapshot, err := r.reader.GetGroupHealth(ctx, id, model)
		if err != nil {
			return snapshot, err
		}
		r.mu.Lock()
		if len(r.entries) >= 4096 {
			clear(r.entries)
		}
		r.entries[key] = groupHealthCacheEntry{snapshot, time.Now().Add(5 * time.Second)}
		r.mu.Unlock()
		return snapshot, nil
	})
	if err != nil {
		return GroupHealthSnapshot{}, err
	}
	return value.(GroupHealthSnapshot), nil
}
