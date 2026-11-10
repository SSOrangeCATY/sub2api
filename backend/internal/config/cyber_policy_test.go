package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCyberPolicyDisableThresholdDefaultAndEnv(t *testing.T) {
	for _, tc := range []struct {
		name, value string
		want        int
		invalid     bool
	}{
		{"default", "", 10, false},
		{"override", "3", 3, false},
		{"count only", "0", 0, false},
		{"negative", "-1", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetViperWithJWTSecret(t)
			t.Setenv("GATEWAY_CYBER_POLICY_DISABLE_THRESHOLD", tc.value)
			cfg, err := Load()
			if tc.invalid {
				require.ErrorContains(t, err, "gateway.cyber_policy_disable_threshold")
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Gateway.CyberPolicyDisableThreshold)
		})
	}
}
