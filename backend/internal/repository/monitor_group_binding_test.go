package repository

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestMonitorGroupAccountBindingRejectsFallbackBeforeMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`(?s)account_group_live_group_lock.*AND group_type = 'business'.*FOR SHARE`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}))
	require.ErrorContains(t, lockBusinessGroups(context.Background(), db, []int64{10}), "business groups")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMonitorGroupAccountBindingAllowsBusinessAndDeduplicatesIDs(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery(`(?s)account_group_live_group_lock.*AND group_type = 'business'.*FOR SHARE`).WithArgs(sqlmock.AnyArg()).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	require.NoError(t, lockBusinessGroups(context.Background(), db, []int64{1, 1, 2}))
	require.NoError(t, mock.ExpectationsWereMet())
}
