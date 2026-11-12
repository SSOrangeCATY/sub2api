package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCyberPolicyNoticeRetainsRestrictionInstructionsWithMaximalUpstreamError(t *testing.T) {
	expires := time.Date(2026, 11, 13, 9, 30, 0, 0, time.UTC)
	for _, test := range []struct {
		name     string
		decision UserRiskDecision
		want     string
	}{
		{"temporary", UserRiskDecision{HitCount: 2, Restricted: true, NewlyApplied: true, ExpiresAt: &expires}, expires.Format(time.RFC3339)},
		{"manual", UserRiskDecision{HitCount: 3, Restricted: true, NewlyApplied: true, AdminRelease: true}, "需联系管理员解除"},
	} {
		t.Run(test.name, func(t *testing.T) {
			repo := &contentModerationTestRepo{}
			svc := NewContentModerationService(
				&contentModerationTestSettingRepo{values: map[string]string{SettingKeyRiskControlEnabled: "true"}},
				repo, nil, nil, nil, nil, nil, nil,
			)
			svc.RecordCyberPolicyEvent(context.Background(), CyberPolicyRecordInput{
				UserID: 1, UserEmail: "synthetic@example.invalid", Model: "gpt-5",
				UpstreamMessage: strings.Repeat("上", maxModerationExcerptRunes*4),
				Decision:        &test.decision,
			})
			logs := repo.snapshotLogs()
			require.Len(t, logs, 1)
			require.Equal(t, test.decision.HitCount, logs[0].ViolationCount)
			require.LessOrEqual(t, len([]rune(logs[0].Error)), maxModerationExcerptRunes*4)
			require.Contains(t, buildCyberPolicyNoticeEmailBody("Synthetic site", &logs[0]), test.want)
		})
	}
}
