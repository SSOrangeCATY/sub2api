package admin

import (
	"encoding/json"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFallbackRateJSONOmissionNullAndValidation(t *testing.T) {
	for _, tc := range []struct {
		raw     string
		want    *float64
		invalid bool
	}{
		{`{}`, nil, false},
		{`{"fallback_rate_multiplier":null}`, fallbackTestRate(-1), false},
		{`{"fallback_rate_multiplier":0.35}`, fallbackTestRate(0.35), false},
		{`{"fallback_rate_multiplier":""}`, fallbackTestRate(-1), false},
		{`{"fallback_rate_multiplier":0}`, nil, true},
		{`{"fallback_rate_multiplier":-1}`, nil, true},
		{`{"fallback_rate_multiplier":"NaN"}`, nil, true},
		{`{"fallback_rate_multiplier":"+Inf"}`, nil, true},
		{`{"fallback_rate_multiplier":1000000}`, nil, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			var req UpdateGroupRequest
			err := json.Unmarshal([]byte(tc.raw), &req)
			if tc.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, req.FallbackRateMultiplier.ToServiceInput())
		})
	}
}
func fallbackTestRate(v float64) *float64 { return &v }
