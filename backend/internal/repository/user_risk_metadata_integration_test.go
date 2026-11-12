//go:build integration

package repository

import (
	"context"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRiskEventAuditMetadataCannotDiscardEnforcement(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	first, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	second, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	_, err = repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: first, Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	input := service.UserRiskEventInput{
		Admission: second, Policy: userRiskTestPolicy(),
		RequestID: string(rune(0)) + string([]byte{255}) + strings.Repeat("标", 300),
		Model:     strings.Repeat("模", 256),
		Endpoint:  string([]byte{254}) + string(rune(0)) + strings.Repeat("路", 300),
	}
	decision, err := repo.RecordUserRiskEvent(ctx, input)
	require.NoError(t, err)
	require.Equal(t, 2, decision.HitCount)
	require.True(t, decision.Restricted)
	require.True(t, decision.NewlyApplied)
	var requestID, model, endpoint, eventID string
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT request_id,model,endpoint,event_id FROM user_risk_events WHERE policy_key=$1 AND event_id=$2",
		second.PolicyKey, second.EventID).Scan(&requestID, &model, &endpoint, &eventID))
	require.Equal(t, 128, utf8.RuneCountInString(requestID))
	require.Equal(t, 255, utf8.RuneCountInString(model))
	require.Equal(t, 255, utf8.RuneCountInString(endpoint))
	for _, value := range []string{requestID, model, endpoint} {
		require.True(t, utf8.ValidString(value))
		require.False(t, strings.ContainsRune(value, 0))
	}
	require.Equal(t, second.EventID, eventID)
	duplicate, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{
		Admission: second, Model: strings.Repeat("变", 2000),
	})
	require.NoError(t, err)
	require.True(t, duplicate.Duplicate)
	require.Equal(t, 2, duplicate.HitCount)
	var events, actions int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_events WHERE user_id=$1", userID).Scan(&events))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_actions WHERE user_id=$1", userID).Scan(&actions))
	require.Equal(t, 2, events)
	require.Equal(t, 2, actions)
}
