//go:build integration

package repository

import (
	"context"
	"sync"
	"testing"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestCyberPolicyAccountPersistenceAndScheduling(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newAccountRepositoryWithSQL(client, tx, nil)
	a := mustCreateAccount(t, client, &service.Account{Name: "cyber-a", Status: service.StatusActive, Schedulable: true, Extra: map[string]any{"keep": "value"}})
	b := mustCreateAccount(t, client, &service.Account{Name: "cyber-b", Status: service.StatusActive, Schedulable: true})
	for i := 1; i <= 9; i++ {
		count, disabled, err := repo.RecordCyberPolicyError(ctx, a.ID, 10)
		require.NoError(t, err)
		require.EqualValues(t, i, count)
		require.False(t, disabled)
	}
	_, disabled, err := repo.RecordCyberPolicyError(ctx, b.ID, 10)
	require.NoError(t, err)
	require.False(t, disabled)
	// A new repository instance must read the persisted count, not start over.
	repo = newAccountRepositoryWithSQL(client, tx, nil)
	count, disabled, err := repo.RecordCyberPolicyError(ctx, a.ID, 10)
	require.NoError(t, err)
	require.EqualValues(t, 10, count)
	require.True(t, disabled)
	stored, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, service.StatusError, stored.Status)
	require.False(t, stored.IsSchedulable())
	require.Contains(t, stored.ErrorMessage, "10")
	require.Equal(t, "value", stored.Extra["keep"])
	stored, err = repo.GetByID(ctx, b.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, stored.Extra[service.CyberPolicyErrorCountExtraKey])
	require.True(t, stored.IsSchedulable())
	eligible, err := client.Account.Query().Where(dbaccount.IDEQ(a.ID), dbaccount.StatusEQ(service.StatusActive), dbaccount.SchedulableEQ(true)).Exist(ctx)
	require.NoError(t, err)
	require.False(t, eligible)
}

func TestCyberPolicyAccountConcurrentPersistence(t *testing.T) {
	ctx := context.Background()
	a := mustCreateAccount(t, integrationEntClient, &service.Account{Name: "cyber-concurrent", Status: service.StatusActive, Schedulable: true})
	t.Cleanup(func() { _, _ = integrationDB.ExecContext(ctx, "DELETE FROM accounts WHERE id = $1", a.ID) })
	repo := newAccountRepositoryWithSQL(integrationEntClient, integrationDB, nil)
	var wg sync.WaitGroup
	errors := make(chan error, 24)
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); _, _, err := repo.RecordCyberPolicyError(ctx, a.ID, 10); errors <- err }()
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	stored, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.EqualValues(t, 24, stored.Extra[service.CyberPolicyErrorCountExtraKey])
	require.False(t, stored.IsSchedulable())
}

func TestCyberPolicyDisableSurvivesStaleEditAndAllowsFreshRecovery(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	client := tx.Client()
	repo := newAccountRepositoryWithSQL(client, tx, nil)
	a := mustCreateAccount(t, client, &service.Account{
		Name: "cyber-stale-edit", Status: service.StatusActive, Schedulable: true,
	})
	stale, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	_, disabled, err := repo.RecordCyberPolicyError(ctx, a.ID, 1)
	require.NoError(t, err)
	require.True(t, disabled)
	stale.Name = "name edited after disable"
	require.NoError(t, repo.Update(ctx, stale))
	stored, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.Equal(t, stale.Name, stored.Name)
	require.Equal(t, service.StatusError, stored.Status)
	require.False(t, stored.IsSchedulable())
	require.Contains(t, stored.ErrorMessage, "Cyber Policy")
	require.EqualValues(t, 1, stored.Extra[service.CyberPolicyErrorCountExtraKey])
	stored.Status, stored.Schedulable, stored.ErrorMessage = service.StatusActive, true, ""
	require.NoError(t, repo.Update(ctx, stored))
	restored, err := repo.GetByID(ctx, a.ID)
	require.NoError(t, err)
	require.True(t, restored.IsSchedulable())
	require.EqualValues(t, 1, restored.Extra[service.CyberPolicyErrorCountExtraKey])
}
