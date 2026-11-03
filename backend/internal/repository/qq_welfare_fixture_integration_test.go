//go:build integration

package repository

import (
	"database/sql"
	"testing"
)

func welfareTestDB(t *testing.T) *sql.DB { t.Helper(); return integrationDB }
