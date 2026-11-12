//go:build unit

package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"testing"
	"time"
)

type runtimeRevisionSettings struct {
	SettingRepository
	values map[string]string
	read   chan struct{}
}

func (r *runtimeRevisionSettings) GetMultiple(context.Context, []string) (map[string]string, error) {
	if r.read != nil {
		select {
		case r.read <- struct{}{}:
		default:
		}
	}
	return r.values, nil
}

func runtimeRevisionFixture(t *testing.T, revision int64, keyword string) (*ContentModerationConfig, []byte, *contentModerationRuntimeSnapshot) {
	t.Helper()
	cfg := defaultContentModerationConfig()
	cfg.Revision = revision
	cfg.BlockedKeywords = []string{keyword}
	cfg.normalize()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &contentModerationRuntimeSnapshot{
		config: cfg.effectiveEngine(cfg.Engine), keywordMatcher: newContentModerationKeywordMatcher(cfg.BlockedKeywords),
		configDigest: sha256.Sum256(raw), riskControlEnabled: true, loadedAt: time.Now(),
	}
	return cfg, raw, snapshot
}

func TestContentModerationRuntimeRevisionRejectsLatePublication(t *testing.T) {
	svc := &ContentModerationService{}
	_, _, initial := runtimeRevisionFixture(t, 1, "initial")
	older, olderRaw, _ := runtimeRevisionFixture(t, 2, "older")
	newer, newerRaw, _ := runtimeRevisionFixture(t, 3, "newer")
	svc.runtimeSnapshot.Store(initial)
	publishOlder := make(chan struct{})
	done := make(chan struct{})
	go func() {
		<-publishOlder
		svc.replaceRuntimeConfig(older, olderRaw)
		close(done)
	}()
	svc.replaceRuntimeConfig(newer, newerRaw)
	close(publishOlder)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("late publication did not finish")
	}
	got := svc.runtimeSnapshot.Load()
	if got.config.Revision != 3 || got.configDigest != sha256.Sum256(newerRaw) {
		t.Fatalf("runtime regressed to revision %d", got.config.Revision)
	}
}

func TestContentModerationRuntimeRevisionRejectsOldRefresh(t *testing.T) {
	_, oldRaw, _ := runtimeRevisionFixture(t, 2, "older")
	_, _, latest := runtimeRevisionFixture(t, 3, "newer")
	svc := &ContentModerationService{settingRepo: &runtimeRevisionSettings{
		values: map[string]string{SettingKeyContentModerationConfig: string(oldRaw), SettingKeyRiskControlEnabled: "true"},
	}}
	svc.runtimeSnapshot.Store(latest)
	got, err := svc.refreshRuntimeSnapshot(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.config.Revision != 3 || svc.runtimeSnapshot.Load() != latest {
		t.Fatal("database refresh replaced a newer published revision")
	}
}

func TestContentModerationRuntimeRevisionRereadsSameVersionMismatch(t *testing.T) {
	_, authoritativeRaw, _ := runtimeRevisionFixture(t, 3, "authoritative")
	_, _, stale := runtimeRevisionFixture(t, 3, "stale runtime")
	wrong, wrongRaw, _ := runtimeRevisionFixture(t, 3, "untrusted")
	read := make(chan struct{}, 1)
	svc := &ContentModerationService{settingRepo: &runtimeRevisionSettings{
		values: map[string]string{SettingKeyContentModerationConfig: string(authoritativeRaw), SettingKeyRiskControlEnabled: "true"},
		read:   read,
	}}
	svc.runtimeSnapshot.Store(stale)
	svc.replaceRuntimeConfig(wrong, wrongRaw)
	select {
	case <-read:
	case <-time.After(time.Second):
		t.Fatal("same-version conflict did not reread authoritative settings")
	}
	svc.runtimeRefreshMu.Lock()
	got := svc.runtimeSnapshot.Load()
	svc.runtimeRefreshMu.Unlock()
	if got.configDigest != sha256.Sum256(authoritativeRaw) {
		t.Fatal("authoritative reread did not repair the stale same-version runtime snapshot")
	}
}
