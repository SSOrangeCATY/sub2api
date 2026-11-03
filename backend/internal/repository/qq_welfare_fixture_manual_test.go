//go:build welfareintegration && !integration

package repository

import (
	"context"
	"database/sql"
	"github.com/stretchr/testify/require"
	"os"
	"sync"
	"testing"
)

var welfareManualOnce sync.Once
var welfareManualDB *sql.DB
var welfareManualErr error

func welfareTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("WELFARE_TEST_DSN")
	if dsn == "" {
		t.Fatal("WELFARE_TEST_DSN must point to a disposable test database")
	}
	welfareManualOnce.Do(func() {
		welfareManualDB, welfareManualErr = sql.Open("postgres", dsn)
		if welfareManualErr == nil {
			welfareManualErr = ApplyMigrations(context.Background(), welfareManualDB)
		}
	})
	require.NoError(t, welfareManualErr)
	return welfareManualDB
}
