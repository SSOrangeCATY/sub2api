//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRiskAdmissionCapturesServerIdentityAndGeneration(t *testing.T) {
	_, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	first, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	second, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	require.NotEmpty(t, first.EventID)
	require.NotEqual(t, first.EventID, second.EventID)
	require.Equal(t, int64(1), first.Generation)
	require.Equal(t, first.Generation, second.Generation)
	require.Equal(t, userID, first.UserID)
	require.Equal(t, service.UserRiskPolicyCyber, first.PolicyKey)
	require.Equal(t, service.UserRiskScopeAPI, first.Scope)
	require.False(t, first.AdmittedAt.IsZero())
	_, err = integrationDB.ExecContext(ctx,
		"UPDATE user_risk_cycles SET generation=generation+1,reset_at=clock_timestamp() WHERE user_id=$1 AND policy_key=$2",
		userID, service.UserRiskPolicyCyber)
	require.NoError(t, err)
	next, err := users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	require.Equal(t, int64(2), next.Generation)
	require.Equal(t, int64(1), first.Generation, "an admitted request keeps its original cycle")
}

func TestUserRiskAdmissionChecksAllApplicablePolicies(t *testing.T) {
	_, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx,
		"INSERT INTO user_risk_restrictions(user_id,policy_key,scope,generation,version,decision_hit_count,threshold,admin_release,restricted_at,policy_revision,policy_snapshot,source_event_id) VALUES($1,'other_policy','api',1,1,2,2,TRUE,clock_timestamp(),1,'{}'::jsonb,'synthetic-block')",
		userID)
	require.NoError(t, err)
	_, err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.Error(t, err, "a different policy cannot bypass an active restriction in the same scope")
	_, err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, "welfare")
	require.NoError(t, err, "a restriction does not silently change unrelated business scopes")
	require.Equal(t, service.StatusActive, ownershipBaseStatus(t, userID))
}

func TestUserRiskAdmissionExpiryDoesNotRequireCleanup(t *testing.T) {
	_, users, userID := restrictionOwnershipFixture(t)
	ctx := context.Background()
	_, err := integrationDB.ExecContext(ctx,
		"INSERT INTO user_risk_restrictions(user_id,policy_key,scope,generation,version,decision_hit_count,threshold,admin_release,restricted_at,expires_at,policy_revision,policy_snapshot,source_event_id) VALUES($1,'other_policy','api',1,1,2,2,FALSE,clock_timestamp()-INTERVAL '2 days',clock_timestamp()-INTERVAL '1 second',1,'{}'::jsonb,'synthetic-expired')",
		userID)
	require.NoError(t, err)
	_, err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.NoError(t, err)
	var retained int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_risk_restrictions WHERE user_id=$1", userID).Scan(&retained))
	require.Equal(t, 1, retained, "access checks must not delete audit state")
	require.NoError(t, users.Update(ctx, &service.User{ID: userID, Status: service.StatusDisabled}, service.UserUpdateFields{Status: true}))
	_, err = users.BeginUserRiskRequest(ctx, userID, service.UserRiskPolicyCyber, service.UserRiskScopeAPI)
	require.Error(t, err, "automatic expiry cannot undo administrator disablement")
}
