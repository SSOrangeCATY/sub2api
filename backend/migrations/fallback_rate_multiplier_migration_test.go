package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFallbackRateMultiplierMigrationDefaultsToTargetRate(t *testing.T) {
	content, err := FS.ReadFile("252_fallback_rate_multiplier.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS fallback_rate_multiplier DECIMAL(10,4)")
	require.Contains(t, sql, "fallback_rate_multiplier IS NULL")
	require.Contains(t, sql, "group_type = 'fallback'")
	require.Contains(t, sql, "fallback_rate_multiplier > 0")
	require.Contains(t, sql, "fallback_rate_multiplier <> 'NaN'::numeric")
	require.NotContains(t, strings.ToLower(sql), "update groups")
}
