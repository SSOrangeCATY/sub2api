package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestMonitorGroupAccountBindingRejectsFallbackBeforeMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`(?s)account_group_live_group_lock.*SELECT id, group_type.*FOR SHARE`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "group_type"}).AddRow(10, service.GroupTypeFallback))
	require.ErrorContains(t, lockBusinessGroups(context.Background(), db, []int64{10}), "business groups")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonitorGroupAccountBindingAllowsBusinessAndDeduplicatesIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`(?s)account_group_live_group_lock.*SELECT id, group_type.*FOR SHARE`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "group_type"}).AddRow(1, service.GroupTypeBusiness).AddRow(2, service.GroupTypeBusiness))
	require.NoError(t, lockBusinessGroups(context.Background(), db, []int64{1, 1, 2}))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonitorGroupAccountBindingPreservesMissingGroupError(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()
	mock.ExpectQuery(`(?s)account_group_live_group_lock.*SELECT id, group_type.*FOR SHARE`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id", "group_type"}))
	require.ErrorIs(t, lockBusinessGroups(context.Background(), db, []int64{10}), service.ErrGroupNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}
