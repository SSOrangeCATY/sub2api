//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRiskReleaseRetainsHistoryAndProtectsNewCycles(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	_, _, actorID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET role=$1 WHERE id=$2", service.RoleAdmin, actorID)
	require.NoError(t, err)
	admissions := make([]service.UserRiskAdmission, 4)
	for index := range admissions {
		admissions[index], err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
	}
	var last *service.UserRiskDecision
	for index := range 3 {
		last, err = repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admissions[index], Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	require.True(t, last.AdminRelease)
	require.NoError(t, users.Update(ctx, &service.User{ID: userID, Status: service.StatusDisabled}, service.UserUpdateFields{Status: true}))
	input := service.UserRiskReleaseInput{
		UserID: userID, PolicyKey: service.UserRiskPolicyCyber, ActorID: actorID,
		OperationID: admissions[0].EventID, Reason: "  人工复核后解除  ", ExpectedVersion: last.RestrictionVersion,
	}
	released, err := repo.ReleaseUserRiskRestriction(ctx, input)
	require.NoError(t, err)
	require.True(t, released.Released)
	require.Equal(t, int64(2), released.Generation)
	require.Equal(t, service.StatusDisabled, released.BaseStatus)
	require.Equal(t, service.StatusDisabled, ownershipBaseStatus(t, userID))
	replay, err := repo.ReleaseUserRiskRestriction(ctx, input)
	require.NoError(t, err)
	require.Equal(t, released, replay)
	changed := input
	changed.Reason = "different operation"
	_, err = repo.ReleaseUserRiskRestriction(ctx, changed)
	require.Error(t, err)
	late, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admissions[3], Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	require.Equal(t, "cycle_changed", late.IgnoredReason)
	require.False(t, late.Restricted)
	require.NoError(t, users.Update(ctx, &service.User{ID: userID, Status: service.StatusActive}, service.UserUpdateFields{Status: true}))
	for count := range 2 {
		admission, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
		decision, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
		require.Equal(t, count+1, decision.HitCount)
		require.Equal(t, int64(2), decision.Generation)
	}
	// Replaying an old release must not clear a new penalty.
	replay, err = repo.ReleaseUserRiskRestriction(ctx, input)
	require.NoError(t, err)
	require.Equal(t, released, replay)
	_, err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.Error(t, err)
	original, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admissions[0]})
	require.NoError(t, err)
	require.True(t, original.Duplicate)
	require.Equal(t, 1, original.HitCount)
	var events, actions int
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_events WHERE user_id=$1", userID).Scan(&events))
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT COUNT(*) FROM user_risk_actions WHERE user_id=$1", userID).Scan(&actions))
	require.Equal(t, 6, events)
	require.Equal(t, 7, actions)
	var raw []byte
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT decision FROM user_risk_actions WHERE user_id=$1 AND action='released'", userID).Scan(&raw))
	var audit service.UserRiskDecision
	require.NoError(t, json.Unmarshal(raw, &audit))
	require.Equal(t, actorID, audit.ActorID)
	require.NotEmpty(t, audit.ActorLabel)
}

func TestUserRiskReleaseRequiresCurrentVersionAndAdministrator(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	admissions := make([]service.UserRiskAdmission, 3)
	for index := range admissions {
		value, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
		admissions[index] = value
	}
	var last *service.UserRiskDecision
	for _, admission := range admissions {
		var err error
		last, err = repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	input := service.UserRiskReleaseInput{UserID: userID, PolicyKey: service.UserRiskPolicyCyber, ActorID: userID,
		OperationID: admissions[0].EventID, Reason: "review", ExpectedVersion: last.RestrictionVersion}
	_, err := repo.ReleaseUserRiskRestriction(ctx, input)
	require.Error(t, err, "ordinary users cannot release their own restrictions")
	_, _, actorID := restrictionOwnershipFixture(t)
	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET role=$1 WHERE id=$2", service.RoleAdmin, actorID)
	require.NoError(t, err)
	input.ActorID, input.ExpectedVersion = actorID, last.RestrictionVersion-1
	_, err = repo.ReleaseUserRiskRestriction(ctx, input)
	require.Error(t, err, "a stale version cannot release a restriction")
	var generation int64
	require.NoError(t, integrationDB.QueryRowContext(ctx, "SELECT generation FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2", userID, service.UserRiskPolicyCyber).Scan(&generation))
	require.Equal(t, int64(1), generation)
}
