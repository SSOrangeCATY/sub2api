//go:build unit

package repository

import (
	"context"
	"errors"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestBillingIdentityMigrationTransaction(t *testing.T) {
	for _, tt := range []struct {
		name       string
		fail, noop bool
	}{{name: "archives migrates and preserves RPM"}, {name: "rollback if rate cleanup fails", fail: true}, {name: "repeat is noop", noop: true}} {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
			repo := &userRepository{client: client, sql: db}
			mock.ExpectBegin()
			mock.ExpectExec("LOCK TABLE user_group_rate_multipliers IN SHARE ROW EXCLUSIVE MODE").WillReturnResult(sqlmock.NewResult(0, 0))
			users := sqlmock.NewRows([]string{"id"})
			backup := sqlmock.NewRows([]string{"id"})
			count := int64(0)
			if !tt.noop {
				users.AddRow(1).AddRow(3)
				backup.AddRow(42)
				count = 5
			}
			mock.ExpectQuery("(?s)SELECT u.id FROM users.*FOR UPDATE OF u").WillReturnRows(users)
			mock.ExpectQuery("(?s)INSERT INTO billing_identity_migrations.*jsonb_agg.*WHERE EXISTS.*RETURNING id").WithArgs(int64(9), sqlmock.AnyArg()).WillReturnRows(backup)
			if !tt.noop {
				mock.ExpectExec("(?s)UPDATE users SET billing_identity='purchase'.*deleted_at IS NULL").WithArgs(sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 2))
			}
			update := mock.ExpectExec("(?s)UPDATE user_group_rate_multipliers SET rate_multiplier=NULL.*WHERE rate_multiplier IS NOT NULL")
			failure := errors.New("cleanup failed")
			if tt.fail {
				update.WillReturnError(failure)
				mock.ExpectRollback()
			} else {
				update.WillReturnResult(sqlmock.NewResult(0, count))
				mock.ExpectExec("DELETE FROM user_group_rate_multipliers WHERE rate_multiplier IS NULL AND rpm_override IS NULL").WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectCommit()
			}
			got, err := repo.MigratePersonalRatesToPurchase(context.Background(), 9)
			if tt.fail {
				require.ErrorIs(t, err, failure)
				require.Nil(t, got)
			} else {
				require.NoError(t, err)
				require.Equal(t, count, got.ClearedRates)
				if tt.noop {
					require.Zero(t, got.BackupID)
					require.Zero(t, got.MigratedUsers)
				} else {
					require.Equal(t, int64(42), got.BackupID)
					require.Equal(t, 2, got.MigratedUsers)
					require.Equal(t, []int64{1, 3}, got.UserIDs)
				}
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestUserRepositoryBillingIdentityPersistedAndUpdatedSurgically(t *testing.T) {
	repo, _ := newUserEntRepo(t)
	ctx := context.Background()
	user := &service.User{Email: "identity@example.test", Username: "member", Role: service.RoleUser, PasswordHash: "hash", Status: service.StatusActive, BillingIdentity: service.BillingIdentityPurchase, Balance: 12}
	require.NoError(t, repo.Create(ctx, user))
	got, err := repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.BillingIdentityPurchase, got.BillingIdentity)
	got.BillingIdentity = service.BillingIdentityStandard
	got.Balance = 0
	require.NoError(t, repo.Update(ctx, got, service.UserUpdateFields{BillingIdentity: true}))
	got, err = repo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.Equal(t, service.BillingIdentityStandard, got.BillingIdentity)
	require.Equal(t, 12.0, got.Balance)
	require.Equal(t, service.RoleUser, got.Role)
}

func TestAPIKeyAuthLoadsBillingIdentity(t *testing.T) {
	repo, client := newAPIKeyRepoSQLite(t)
	ctx := context.Background()
	user := mustCreateAPIKeyRepoUser(t, ctx, client, "identity-auth@example.test")
	require.NoError(t, client.User.UpdateOneID(user.ID).SetBillingIdentity(service.BillingIdentityPurchase).Exec(ctx))
	key := &service.APIKey{UserID: user.ID, Key: "billing-identity-auth", Name: "identity test", Status: service.StatusActive}
	require.NoError(t, repo.Create(ctx, key))
	got, err := repo.GetByKeyForAuth(ctx, key.Key)
	require.NoError(t, err)
	require.True(t, got.User.UsesPurchasePricing())
}
