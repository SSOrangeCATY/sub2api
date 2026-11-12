//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func moderationRevisionDocument(t *testing.T, revision int64, label string) string {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"revision": revision, "blocked_keywords": []string{label}})
	require.NoError(t, err)
	return string(raw)
}

func moderationRevisionTestRepository(t *testing.T) *settingRepository {
	t.Helper()
	repo := &settingRepository{client: testEntClient(t)}
	ctx := context.Background()
	previous, err := repo.GetValue(ctx, service.SettingKeyContentModerationConfig)
	if err != nil {
		require.ErrorIs(t, err, service.ErrSettingNotFound)
	}
	t.Cleanup(func() {
		if err == nil {
			require.NoError(t, repo.Set(ctx, service.SettingKeyContentModerationConfig, previous))
		} else {
			require.NoError(t, repo.Delete(ctx, service.SettingKeyContentModerationConfig))
		}
	})
	return repo
}

func TestModerationRevisionCASLegacyAndConflictingDocuments(t *testing.T) {
	repo := moderationRevisionTestRepository(t)
	ctx := context.Background()
	cases := []struct {
		name, seed     string
		expected       int64
		saved, invalid bool
	}{
		{"first save", "__ABSENT__", 1, true, false},
		{"missing row rejects a later revision", "__ABSENT__", 9, false, false},
		{"empty legacy document", "", 1, true, false},
		{"whitespace legacy document", "   ", 1, true, false},
		{"legacy missing revision", "{}", 1, true, false},
		{"legacy zero revision", moderationRevisionDocument(t, 0, "zero"), 1, true, false},
		{"matching version", moderationRevisionDocument(t, 9, "current"), 9, true, false},
		{"stale version", moderationRevisionDocument(t, 9, "current"), 8, false, false},
		{"malformed document preserved", "{broken", 1, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.seed == "__ABSENT__" {
				require.NoError(t, repo.Delete(ctx, service.SettingKeyContentModerationConfig))
			} else {
				require.NoError(t, repo.Set(ctx, service.SettingKeyContentModerationConfig, tc.seed))
			}
			candidate := moderationRevisionDocument(t, tc.expected+1, "candidate")
			saved, err := repo.CompareAndSetContentModerationConfig(ctx, candidate, nil, tc.expected)
			if tc.invalid {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tc.saved, saved)
			stored, readErr := repo.GetValue(ctx, service.SettingKeyContentModerationConfig)
			if tc.saved {
				require.NoError(t, readErr)
				require.JSONEq(t, candidate, stored)
			} else if tc.seed == "__ABSENT__" {
				require.ErrorIs(t, readErr, service.ErrSettingNotFound)
			} else {
				require.NoError(t, readErr)
				require.Equal(t, tc.seed, stored)
			}
		})
	}
}

func TestModerationRevisionCASConcurrentWriters(t *testing.T) {
	repo := moderationRevisionTestRepository(t)
	for _, existing := range []bool{false, true} {
		t.Run(map[bool]string{false: "first writers", true: "existing document"}[existing], func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if existing {
				require.NoError(t, repo.Set(ctx, service.SettingKeyContentModerationConfig, moderationRevisionDocument(t, 1, "initial")))
			} else {
				require.NoError(t, repo.Delete(ctx, service.SettingKeyContentModerationConfig))
			}
			type outcome struct {
				saved    bool
				err      error
				document string
			}
			start := make(chan struct{})
			results := make(chan outcome, 2)
			for _, label := range []string{"writer A", "writer B"} {
				document := moderationRevisionDocument(t, 2, label)
				go func(raw string) {
					<-start
					saved, err := repo.CompareAndSetContentModerationConfig(ctx, raw, nil, 1)
					results <- outcome{saved, err, raw}
				}(document)
			}
			close(start)
			winner, successful := "", 0
			for range 2 {
				result := <-results
				require.NoError(t, result.err)
				if result.saved {
					winner = result.document
					successful++
				}
			}
			require.Equal(t, 1, successful, "exactly one same-version writer must commit")
			stored, err := repo.GetValue(ctx, service.SettingKeyContentModerationConfig)
			require.NoError(t, err)
			require.JSONEq(t, winner, stored)
		})
	}
}

func TestModerationRevisionCASRejectsNestedTransaction(t *testing.T) {
	repo := moderationRevisionTestRepository(t)
	ctx := context.Background()
	initial := moderationRevisionDocument(t, 1, "initial")
	require.NoError(t, repo.Set(ctx, service.SettingKeyContentModerationConfig, initial))
	tx, err := testEntClient(t).Tx(ctx)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()
	nested := &settingRepository{client: tx.Client()}
	saved, err := nested.CompareAndSetContentModerationConfig(ctx, moderationRevisionDocument(t, 2, "uncommitted"), nil, 1)
	require.Error(t, err)
	require.False(t, saved)
	require.NoError(t, tx.Rollback())
	stored, err := repo.GetValue(ctx, service.SettingKeyContentModerationConfig)
	require.NoError(t, err)
	require.Equal(t, initial, stored)
}
