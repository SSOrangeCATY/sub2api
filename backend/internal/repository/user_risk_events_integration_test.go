//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func userRiskTestPolicy() service.UserRiskPolicySnapshot {
	return service.UserRiskPolicySnapshot{Revision: 1, Scope: service.UserRiskScopeAPI, WindowDays: 7,
		Actions: []service.UserRiskActionRule{{Threshold: 2, DurationHours: 24}, {Threshold: 3, AdminRelease: true}}}
}

func TestUserRiskEventConcurrentEscalationAndOriginalDeduplication(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admissions := make([]service.UserRiskAdmission, 3)
	for index := range admissions {
		value, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
		admissions[index] = value
	}
	type outcome struct {
		decision *service.UserRiskDecision
		err      error
	}
	start, results := make(chan struct{}), make(chan outcome, 3)
	for _, admission := range admissions {
		go func(value service.UserRiskAdmission) {
			<-start
			decision, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: value, Policy: userRiskTestPolicy()})
			results <- outcome{decision, err}
		}(admission)
	}
	close(start)
	counts := map[int]bool{}
	firstEvent := ""
	for range 3 {
		result := <-results
		require.NoError(t, result.err)
		counts[result.decision.HitCount] = true
		if result.decision.HitCount == 1 {
			firstEvent = result.decision.EventID
			require.False(t, result.decision.Restricted)
		}
		if result.decision.HitCount == 3 {
			require.True(t, result.decision.AdminRelease)
			require.True(t, result.decision.NewlyApplied)
		}
	}
	require.Equal(t, map[int]bool{1: true, 2: true, 3: true}, counts)
	for _, admission := range admissions {
		if admission.EventID != firstEvent {
			continue
		}
		// An empty new policy must not cause a stored event to be reevaluated.
		duplicate, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission})
		require.NoError(t, err)
		require.True(t, duplicate.Duplicate)
		require.Equal(t, 1, duplicate.HitCount)
		require.False(t, duplicate.Restricted)
	}
	var events, actions int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_events WHERE user_id=$1", userID).Scan(&events))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_actions WHERE user_id=$1", userID).Scan(&actions))
	require.Equal(t, 3, events)
	require.Equal(t, 3, actions)
	require.Equal(t, service.StatusActive, ownershipBaseStatus(t, userID))
	_, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.Error(t, err)
}

func TestUserRiskEventLateOldGenerationIsRetainedWithoutRebanning(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	old, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	_, err = integrationDB.ExecContext(ctx, "UPDATE user_risk_cycles SET generation=generation+1,reset_at=clock_timestamp() WHERE user_id=$1 AND policy_key=$2", userID, service.UserRiskPolicyCyber)
	require.NoError(t, err)
	ignored, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: old, Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	require.Equal(t, "cycle_changed", ignored.IgnoredReason)
	require.False(t, ignored.Restricted)
	next, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	decision, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: next, Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	require.Equal(t, int64(2), decision.Generation)
	require.Equal(t, 1, decision.HitCount)
	var retained, counted int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*),COUNT(*) FILTER(WHERE counted) FROM user_risk_events WHERE user_id=$1", userID).Scan(&retained, &counted))
	require.Equal(t, 2, retained)
	require.Equal(t, 1, counted)
}

func TestUserRiskEventUsesRollingSevenDayWindow(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	admission, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	for _, days := range []int{8, 6} {
		_, err := integrationDB.ExecContext(ctx,
			"INSERT INTO user_risk_events(policy_key,event_id,user_id,generation,counted,policy_revision,policy_snapshot,result,admitted_at,recorded_at) VALUES($1,$2,$3,1,TRUE,1,'{}'::jsonb,'{}'::jsonb,clock_timestamp()-($4::int*INTERVAL '1 day'),clock_timestamp()-($4::int*INTERVAL '1 day'))",
			admission.PolicyKey, admission.EventID+string(rune('a'+days)), userID, days)
		require.NoError(t, err)
	}
	decision, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	require.Equal(t, 2, decision.HitCount)
	require.True(t, decision.Restricted)
	require.False(t, decision.AdminRelease)
	require.NotNil(t, decision.ExpiresAt)
	require.Equal(t, 24*time.Hour, decision.ExpiresAt.Sub(decision.RecordedAt))
}
