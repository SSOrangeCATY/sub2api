//go:build integration

package repository

import (
	"context"
	"crypto/rand"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
	"testing"
)

func restrictionOwnershipFixture(t *testing.T) (*contentModerationRepository, *userRepository, int64) {
	t.Helper()
	ctx := context.Background()
	var userID int64
	require.NoError(t, integrationDB.QueryRowContext(ctx,
		"INSERT INTO users(email,password_hash,role,status) VALUES($1,$2,$3,$4) RETURNING id",
		"restriction-ownership-"+rand.Text()+"@example.test",
		"synthetic-test-password-hash", service.RoleUser, service.StatusActive).Scan(&userID))
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(context.Background(), "DELETE FROM users WHERE id=$1", userID)
		require.NoError(t, err)
	})
	return &contentModerationRepository{db: integrationDB}, &userRepository{client: testEntClient(t)}, userID
}

func ownershipBaseStatus(t *testing.T, userID int64) string {
	t.Helper()
	var status string
	require.NoError(t, integrationDB.QueryRowContext(context.Background(), "SELECT status FROM users WHERE id=$1", userID).Scan(&status))
	return status
}
