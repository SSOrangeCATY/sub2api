//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"testing"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestUserRiskReleaseConcurrentCrossUserOperationCollision(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	type target struct{ userID, version int64 }
	targets := []target{}
	var repo *contentModerationRepository
	for range 2 {
		current, users, userID := restrictionOwnershipFixture(t)
		repo = current
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
		targets = append(targets, target{userID, last.RestrictionVersion})
	}
	_, _, actorID := restrictionOwnershipFixture(t)
	_, err := integrationDB.ExecContext(ctx, "UPDATE users SET role=$1 WHERE id=$2", service.RoleAdmin, actorID)
	require.NoError(t, err)
	guard, err := integrationDB.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = guard.Rollback() }()
	_, err = guard.ExecContext(ctx, "LOCK TABLE user_risk_actions IN SHARE MODE")
	require.NoError(t, err)
	operationID := rand.Text()
	type outcome struct {
		userID int64
		result *service.UserRiskReleaseResult
		err    error
	}
	results := make(chan outcome, 2)
	for _, item := range targets {
		go func(value target) {
			result, err := repo.ReleaseUserRiskRestriction(ctx, service.UserRiskReleaseInput{
				UserID: value.userID, PolicyKey: service.UserRiskPolicyCyber, ActorID: actorID,
				OperationID: operationID, Reason: "concurrent review", ExpectedVersion: value.version,
			})
			results <- outcome{value.userID, result, err}
		}(item)
	}
	require.Eventually(t, func() bool {
		var waiting int
		err := integrationDB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM pg_stat_activity WHERE datname=current_database() AND pid!=pg_backend_pid() AND wait_event_type='Lock' AND query LIKE '%INSERT INTO user_risk_actions%'").Scan(&waiting)
		return err == nil && waiting == 2
	}, 3*time.Second, 20*time.Millisecond, "both transactions must reach the real operation-key insert")
	require.NoError(t, guard.Commit())
	successful, conflicted := 0, 0
	for range 2 {
		item := <-results
		var generation int64
		require.NoError(t, integrationDB.QueryRowContext(ctx,
			"SELECT generation FROM user_risk_cycles WHERE user_id=$1 AND policy_key=$2",
			item.userID, service.UserRiskPolicyCyber).Scan(&generation))
		if item.err == nil {
			successful++
			require.True(t, item.result.Released)
			require.Equal(t, int64(2), generation)
		} else {
			conflicted++
			require.Equal(t, 409, infraerrors.Code(item.err))
			require.Equal(t, "USER_RISK_RELEASE_IDEMPOTENCY_CONFLICT", infraerrors.Reason(item.err))
			require.Equal(t, int64(1), generation, "the failed release must roll back its cycle")
		}
	}
	require.Equal(t, 1, successful)
	require.Equal(t, 1, conflicted)
	var released int
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM user_risk_actions WHERE policy_key=$1 AND operation_id=$2 AND action='released'",
		service.UserRiskPolicyCyber, "release/"+operationID).Scan(&released))
	require.Equal(t, 1, released)
}
