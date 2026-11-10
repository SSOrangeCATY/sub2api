package repository

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRecordCyberPolicyErrorPersistsThresholdAndReturnsState(t *testing.T) {
	for _, tc := range []struct {
		threshold int
		count     int64
		disabled  bool
	}{{10, 9, false}, {10, 10, true}, {3, 4, true}, {0, 20, false}} {
		t.Run(fmt.Sprintf("threshold_%d_count_%d", tc.threshold, tc.count), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer func() { _ = db.Close() }()
			repo := &accountRepository{sql: db}
			mock.ExpectQuery(regexp.QuoteMeta(recordCyberPolicyErrorSQL)).WithArgs(int64(7), tc.threshold, service.StatusError, sqlmock.AnyArg(), service.SchedulerOutboxEventAccountChanged).
				WillReturnRows(sqlmock.NewRows([]string{"count", "disabled"}).AddRow(tc.count, tc.disabled))
			count, disabled, err := repo.RecordCyberPolicyError(t.Context(), 7, tc.threshold)
			require.NoError(t, err)
			require.Equal(t, tc.count, count)
			require.Equal(t, tc.disabled, disabled)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestRecordCyberPolicyErrorPropagatesStorageFailureAndMissingAccount(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	repo := &accountRepository{sql: db}
	storageErr := errors.New("database down")
	mock.ExpectQuery(regexp.QuoteMeta(recordCyberPolicyErrorSQL)).WillReturnError(storageErr)
	_, _, err = repo.RecordCyberPolicyError(t.Context(), 7, 10)
	require.ErrorIs(t, err, storageErr)
	mock.ExpectQuery(regexp.QuoteMeta(recordCyberPolicyErrorSQL)).WillReturnRows(sqlmock.NewRows([]string{"count", "disabled"}))
	_, _, err = repo.RecordCyberPolicyError(t.Context(), 7, 10)
	require.ErrorIs(t, err, service.ErrAccountNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCyberPolicyCountSurvivesStaleAccountEdit(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	client := dbent.NewClient(dbent.Driver(entsql.OpenDB(dialect.Postgres, db)))
	defer func() { _ = client.Close() }()
	mock.ExpectQuery(`(?s)SELECT.*FOR NO KEY UPDATE`).WillReturnRows(sqlmock.NewRows(openCodeGoMergeMockColumns()).
		AddRow(false, false, false, nil, nil, nil, nil, nil, nil, false, nil, nil, nil, nil, []byte(`{"count":9}`)))
	account := &service.Account{ID: 7, Extra: map[string]any{service.CyberPolicyErrorCountExtraKey: 0, "note": "edited"}}
	extra, err := lockAndMergeAccountProbeExtra(context.Background(), client, account, nil, nil)
	require.NoError(t, err)
	require.EqualValues(t, 9, extra[service.CyberPolicyErrorCountExtraKey])
	require.Equal(t, "edited", extra["note"])
	require.NoError(t, mock.ExpectationsWereMet())
	updates := map[string]any{service.CyberPolicyErrorCountExtraKey: 0, "codex_fingerprint_seed": "injected", "note": "edited"}
	require.Equal(t, map[string]any{"note": "edited"}, stripAccountReadOnlyExtraUpdate(updates))
	require.Len(t, updates, 3, "do not mutate caller input")
}

func TestCyberPolicyStaleEditPreservesDisableAndFreshEditCanRestore(t *testing.T) {
	for _, tc := range []struct {
		name        string
		loadedCount int64
		wantStatus  string
	}{
		{"stale edit", 9, service.StatusError},
		{"fresh explicit recovery", 10, service.StatusActive},
	} {
		t.Run(tc.name, func(t *testing.T) {
			account := &service.Account{Status: service.StatusActive, Schedulable: true,
				Extra: map[string]any{service.CyberPolicyErrorCountExtraKey: tc.loadedCount}}
			extra := map[string]any{"note": "edited"}
			err := mergeCyberPolicyAccountState(account, extra, []byte(`{"count":10,"status":"error","schedulable":false,"error_message":"Cyber Policy: accumulated upstream errors reached disable threshold (10)"}`))
			require.NoError(t, err)
			require.Equal(t, tc.wantStatus, account.Status)
			require.Equal(t, tc.wantStatus == service.StatusActive, account.Schedulable)
			require.EqualValues(t, 10, extra[service.CyberPolicyErrorCountExtraKey])
			require.Equal(t, "edited", extra["note"])
		})
	}
}
