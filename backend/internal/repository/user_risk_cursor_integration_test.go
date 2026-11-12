//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func cursorRiskAdmissions(t *testing.T, users *userRepository, userID int64, policy string, count int) []service.UserRiskAdmission {
	t.Helper()
	admissions := make([]service.UserRiskAdmission, count)
	for index := range admissions {
		value, err := users.BeginUserRiskRequest(context.Background(), userID, policy, service.UserRiskScopeAPI)
		require.NoError(t, err)
		admissions[index] = value
	}
	return admissions
}

func TestUserRiskCursorHistoryExcludesNewActionsAndBindsFilters(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	admissions := cursorRiskAdmissions(t, users, userID, service.UserRiskPolicyCyber, 5)
	for _, admission := range admissions[:4] {
		_, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	filter := service.UserRiskListFilter{PolicyKey: service.UserRiskPolicyCyber, UserID: userID, PageSize: 2}
	first, err := repo.ListUserRiskActions(ctx, filter)
	require.NoError(t, err)
	require.Len(t, first.Items, 2)
	require.Equal(t, int64(4), first.Total)
	require.True(t, first.HasMore)
	require.NotEmpty(t, first.NextCursor)
	require.False(t, first.AsOf.IsZero())
	_, err = repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admissions[4], Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	filter.Cursor = first.NextCursor
	second, err := repo.ListUserRiskActions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, 2, second.Page)
	require.Equal(t, int64(4), second.Total)
	require.Len(t, second.Items, 2)
	require.False(t, second.HasMore)
	require.Greater(t, first.Items[1].ID, second.Items[0].ID)
	changed := filter
	changed.Action = "hit"
	_, err = repo.ListUserRiskActions(ctx, changed)
	require.Error(t, err, "a cursor must not silently change its filters")
	_, err = repo.ListUserRiskRestrictions(ctx, filter)
	require.Error(t, err, "history and restriction cursors cannot be interchanged")
	changed = filter
	changed.Cursor = "not-a-valid-cursor"
	_, err = repo.ListUserRiskActions(ctx, changed)
	require.Error(t, err)
	changed = filter
	changed.Cursor, changed.Page = "", 2
	_, err = repo.ListUserRiskActions(ctx, changed)
	require.Error(t, err, "page numbers must never reintroduce OFFSET traversal")
}

func TestUserRiskCursorRestrictionsSurviveReleaseInsertAndEscalation(t *testing.T) {
	ctx := context.Background()
	policyKey := "cursor_" + rand.Text()
	var repo *contentModerationRepository
	pending := map[int64]service.UserRiskAdmission{}
	for range 3 {
		current, users, userID := restrictionOwnershipFixture(t)
		repo = current
		admissions := cursorRiskAdmissions(t, users, userID, policyKey, 3)
		for _, admission := range admissions[:2] {
			_, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
			require.NoError(t, err)
		}
		pending[userID] = admissions[2]
	}
	all, err := repo.ListUserRiskRestrictions(ctx, service.UserRiskListFilter{PolicyKey: policyKey, PageSize: 100})
	require.NoError(t, err)
	require.Len(t, all.Items, 3)
	filter := service.UserRiskListFilter{PolicyKey: policyKey, PageSize: 1}
	first, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, all.Items[0].UserID, first.Items[0].UserID)
	_, _, actorID := restrictionOwnershipFixture(t)
	_, err = integrationDB.ExecContext(ctx, "UPDATE users SET role=$1 WHERE id=$2", service.RoleAdmin, actorID)
	require.NoError(t, err)
	_, err = repo.ReleaseUserRiskRestriction(ctx, service.UserRiskReleaseInput{
		UserID: first.Items[0].UserID, PolicyKey: policyKey, ActorID: actorID,
		OperationID: rand.Text(), Reason: "cursor regression", ExpectedVersion: first.Items[0].Version,
	})
	require.NoError(t, err)
	_, err = repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: pending[all.Items[1].UserID], Policy: userRiskTestPolicy()})
	require.NoError(t, err)
	current, newUsers, newUserID := restrictionOwnershipFixture(t)
	for _, admission := range cursorRiskAdmissions(t, newUsers, newUserID, policyKey, 2) {
		_, err := current.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	filter.Cursor = first.NextCursor
	second, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Len(t, second.Items, 1)
	require.Equal(t, all.Items[1].UserID, second.Items[0].UserID)
	require.True(t, second.Items[0].AdminRelease)
	require.True(t, all.Items[1].RestrictedAt.Equal(second.Items[0].RestrictedAt), "escalation retains the continuous interval start")
	filter.Cursor = second.NextCursor
	third, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Len(t, third.Items, 1)
	require.Equal(t, all.Items[2].UserID, third.Items[0].UserID)
	require.False(t, third.HasMore)
}

func TestUserRiskCursorRestrictionTieUsesCompletePolicyKey(t *testing.T) {
	repo, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	firstKey, secondKey := "cursor_alpha", "cursor_beta"
	first := cursorRiskAdmissions(t, users, userID, firstKey, 2)
	second := cursorRiskAdmissions(t, users, userID, secondKey, 2)
	for _, admission := range append(first, second...) {
		_, err := repo.RecordUserRiskEvent(ctx, service.UserRiskEventInput{Admission: admission, Policy: userRiskTestPolicy()})
		require.NoError(t, err)
	}
	_, err := integrationDB.ExecContext(ctx, "UPDATE user_risk_restrictions SET restricted_at=transaction_timestamp() WHERE user_id=$1", userID)
	require.NoError(t, err)
	filter := service.UserRiskListFilter{UserID: userID, PageSize: 1}
	page, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, firstKey, page.Items[0].PolicyKey)
	filter.Cursor = page.NextCursor
	next, err := repo.ListUserRiskRestrictions(ctx, filter)
	require.NoError(t, err)
	require.Equal(t, secondKey, next.Items[0].PolicyKey)
	require.False(t, next.HasMore)
}
