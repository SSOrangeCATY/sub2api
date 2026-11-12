//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"database/sql"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

type riskVisibilityInserter interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func insertRiskVisibilityAction(t *testing.T, db riskVisibilityInserter, userID int64, policy string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.QueryRowContext(context.Background(),
		"INSERT INTO user_risk_actions(user_id,policy_key,generation,action,operation_id,decision) VALUES($1,$2,1,'hit',$3,'{}'::jsonb) RETURNING id",
		userID, policy, rand.Text()).Scan(&id))
	return id
}

func TestUserRiskCursorHistoryDocumentsLateLowIDCommit(t *testing.T) {
	for _, size := range []int{1, 2} {
		t.Run(map[int]string{1: "boundary not crossed", 2: "boundary already crossed"}[size], func(t *testing.T) {
			repo, _, firstUserID := restrictionOwnershipFixture(t)
			_, _, secondUserID := restrictionOwnershipFixture(t)
			ctx := context.Background()
			policy := "late_cursor_" + rand.Text()
			oldest := insertRiskVisibilityAction(t, integrationDB, firstUserID, policy)
			earlier := insertRiskVisibilityAction(t, integrationDB, firstUserID, policy)
			pending, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = pending.Rollback() }()
			var locked int64
			require.NoError(t, pending.QueryRowContext(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", firstUserID).Scan(&locked))
			lowID := insertRiskVisibilityAction(t, pending, firstUserID, policy)
			committed, err := integrationDB.BeginTx(ctx, nil)
			require.NoError(t, err)
			defer func() { _ = committed.Rollback() }()
			require.NoError(t, committed.QueryRowContext(ctx, "SELECT id FROM users WHERE id=$1 FOR UPDATE", secondUserID).Scan(&locked))
			highID := insertRiskVisibilityAction(t, committed, secondUserID, policy)
			require.NoError(t, committed.Commit())
			require.Less(t, earlier, lowID)
			require.Less(t, lowID, highID)
			filter := service.UserRiskListFilter{PolicyKey: policy, PageSize: size}
			first, err := repo.ListUserRiskActions(ctx, filter)
			require.NoError(t, err)
			require.Equal(t, int64(3), first.Total)
			require.Equal(t, highID, first.Items[0].ID)
			require.True(t, first.HasMore)
			require.NoError(t, pending.Commit())
			filter.Cursor = first.NextCursor
			second, err := repo.ListUserRiskActions(ctx, filter)
			require.NoError(t, err)
			require.Equal(t, int64(4), second.Total, "Total describes the current page's visible read snapshot")
			if size == 1 {
				require.Equal(t, lowID, second.Items[0].ID)
			} else {
				require.Equal(t, oldest, second.Items[0].ID)
				require.False(t, second.HasMore, "a newly visible record above the cursor requires refresh")
			}
			refreshed, err := repo.ListUserRiskActions(ctx, service.UserRiskListFilter{PolicyKey: policy, PageSize: 100})
			require.NoError(t, err)
			require.Equal(t, int64(4), refreshed.Total)
			require.Len(t, refreshed.Items, 4)
			require.Equal(t, lowID, refreshed.Items[1].ID)
		})
	}
}
