//go:build integration

package repository

import (
	"context"
	"strconv"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRiskQueriesSeparateWindowCountsAndDecisionHistory(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	admissions := make([]service.UserRiskAdmission, 2)
	for index := range admissions {
		value, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
		admissions[index] = value
	}
	_, err := integrationDB.ExecContext(ctx,
		"INSERT INTO user_risk_events(policy_key,event_id,user_id,generation,counted,policy_revision,policy_snapshot,result,admitted_at,recorded_at) VALUES($1,$2,$3,1,TRUE,4,'{}'::jsonb,'{}'::jsonb,clock_timestamp()-INTERVAL '2 days',clock_timestamp()-INTERVAL '2 days')",
		service.UserRiskPolicyCyber, "historical/"+admissions[0].EventID, userID)
	require.NoError(t, err)
	policy := userRiskTestPolicy()
	policy.Revision = 5
	for _, admission := range admissions {
		_, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: policy})
		require.NoError(t, err)
	}
	filter := service.UserRiskListFilter{PolicyKey: service.UserRiskPolicyCyber, UserID: userID, WindowDays: 1, ReleaseMode: "manual", PageSize: 1000}
	current, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 100, current.PageSize)
	require.Equal(t, 1, current.Page)
	require.Equal(t, int64(1), current.Total)
	require.Len(t, current.Items, 1)
	require.Equal(t, 2, current.Items[0].WindowHitCount)
	require.Equal(t, 3, current.Items[0].DecisionHitCount)
	require.Equal(t, int64(5), current.Items[0].PolicyRevision)
	filter.ReleaseMode = "temporary"
	empty, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Zero(t, empty.Total)
	require.NotNil(t, empty.Items)
	filter.ReleaseMode, filter.Search = "", "%"
	empty, err = repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Zero(t, empty.Total, "search treats SQL wildcard characters literally")
	historyFilter := service.UserRiskListFilter{PolicyKey: service.UserRiskPolicyCyber, UserID: userID, Page: 1, PageSize: 1, Search: strconv.FormatInt(userID, 10)}
	first, err := repo.ListUserRiskActions(ctx, historyFilter)
	require.NoError(t, err)
	require.Equal(t, int64(2), first.Total)
	require.Equal(t, 2, first.Pages)
	historyFilter.Page = 2
	historyFilter.Cursor = first.NextCursor
	second, err := repo.ListUserRiskActions(ctx, historyFilter)
	require.NoError(t, err)
	require.Len(t, first.Items, 1)
	require.Len(t, second.Items, 1)
	require.Greater(t, first.Items[0].ID, second.Items[0].ID)
	historyFilter.Page, historyFilter.Action = 1, "restricted"
	historyFilter.Cursor = ""
	restricted, err := repo.ListUserRiskActions(ctx, historyFilter)
	require.NoError(t, err)
	require.Equal(t, int64(1), restricted.Total)
}

func TestUserRiskExpiryRetainsCountsAndNeverReenablesManualStatus(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	admissions := make([]service.UserRiskAdmission, 2)
	for index := range admissions {
		value, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
		require.NoError(t, err)
		admissions[index] = value
	}
	for _, admission := range admissions {
		_, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	require.NoError(t, users.Update(ctx, &service.User{ID: userID, Status: service.StatusDisabled}, service.UserUpdateFields{Status: true}))
	_, err := integrationDB.ExecContext(ctx, "UPDATE user_risk_restrictions SET expires_at=clock_timestamp()-INTERVAL '1 second' WHERE user_id=$1", userID)
	require.NoError(t, err)
	changed, err := repo.ExpireUserRiskRestrictions(ctx, 1000)
	require.NoError(t, err)
	require.Contains(t, changed, userID)
	require.Equal(t, service.StatusDisabled, ownershipBaseStatus(t, userID))
	again, err := repo.ExpireUserRiskRestrictions(ctx, 100)
	require.NoError(t, err)
	require.Empty(t, again)
	expired, err := repo.ListUserRiskActions(ctx, service.UserRiskListFilter{UserID: userID, Action: "expired"})
	require.NoError(t, err)
	require.Equal(t, int64(1), expired.Total)
	require.Equal(t, int64(2), expired.Items[0].Decision.RestrictionVersion)
	require.False(t, expired.Items[0].Decision.Restricted)
	require.NoError(t, users.Update(ctx, &service.User{ID: userID, Status: service.StatusActive}, service.UserUpdateFields{Status: true}))
	admission, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	decision, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	require.Equal(t, int64(1), decision.Generation)
	require.Equal(t, 3, decision.HitCount)
	require.True(t, decision.AdminRelease)
	require.Equal(t, int64(3), decision.RestrictionVersion)
}
