//go:build unit

package service

import (
	"encoding/json"
	"testing"
	"time"
)

func TestContentModerationPolicyRevisionDefaultsAndProfiles(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]any
		want   int64
	}{
		{"legacy document", map[string]any{"enabled": true}, 1},
		{"legacy zero revision", map[string]any{"revision": 0}, 1},
		{"saved revision", map[string]any{"revision": 9}, 9},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := json.Marshal(tc.fields)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := parseContentModerationConfig(string(raw))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Revision != tc.want {
				t.Fatalf("revision=%d, want %d", cfg.Revision, tc.want)
			}
			if got := cfg.effectiveEngine("typesafe").Revision; got != tc.want {
				t.Fatalf("engine profile lost revision: got %d, want %d", got, tc.want)
			}
		})
	}
	cfg, err := parseContentModerationConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Revision != 1 {
		t.Fatalf("new configuration revision=%d", cfg.Revision)
	}
}

func confirmedUserRestrictionPolicy() CyberPolicyUserRestrictionPolicy {
	return CyberPolicyUserRestrictionPolicy{Enabled: true, WindowDays: 7, Actions: []CyberPolicyUserRestrictionRule{
		{Threshold: 2, DurationHours: 24},
		{Threshold: 3, AdminRelease: true},
	}}
}

func TestCyberPolicyUserRestrictionPolicyConfirmedEscalation(t *testing.T) {
	policy := confirmedUserRestrictionPolicy()
	if err := ValidateCyberPolicyUserRestriction(policy); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                   string
		hits, threshold, hours int
		manual, applies        bool
	}{
		{"first hit records only", 1, 0, 0, false, false},
		{"second hit suspends one day", 2, 2, 24, false, true},
		{"third hit requires administrator", 3, 3, 0, true, true},
		{"later hits retain administrator release", 9, 3, 0, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action, applies := UserRiskActionForCount(policy.Actions, tc.hits)
			if applies != tc.applies || action.Threshold != tc.threshold || action.DurationHours != tc.hours || action.AdminRelease != tc.manual {
				t.Fatalf("hits=%d: action=%+v applies=%t", tc.hits, action, applies)
			}
		})
	}
	now := time.Date(2026, 11, 12, 0, 0, 0, 0, time.UTC)
	if got := CyberPolicyWindowStart(policy, now); !got.Equal(now.Add(-168 * time.Hour)) {
		t.Fatalf("unexpected rolling window: %s", got)
	}
}

func TestCyberPolicyUserRestrictionPolicyRejectsInvalidConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		change func(*CyberPolicyUserRestrictionPolicy)
	}{
		{"empty window", func(p *CyberPolicyUserRestrictionPolicy) { p.WindowDays = 0 }},
		{"oversized window", func(p *CyberPolicyUserRestrictionPolicy) { p.WindowDays = 31 }},
		{"no actions", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions = nil }},
		{"too many actions", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions = make([]CyberPolicyUserRestrictionRule, 9) }},
		{"zero threshold", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[0].Threshold = 0 }},
		{"duplicate thresholds", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[1].Threshold = 2 }},
		{"descending thresholds", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[0].Threshold = 4 }},
		{"oversized threshold", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[1].Threshold = 1001 }},
		{"zero temporary duration", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[0].DurationHours = 0 }},
		{"oversized duration", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[0].DurationHours = 721 }},
		{"manual action with timer", func(p *CyberPolicyUserRestrictionPolicy) { p.Actions[1].DurationHours = 24 }},
		{"manual action before another rule", func(p *CyberPolicyUserRestrictionPolicy) {
			p.Actions[0].AdminRelease = true
			p.Actions[0].DurationHours = 0
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			policy := confirmedUserRestrictionPolicy()
			tc.change(&policy)
			if err := ValidateCyberPolicyUserRestriction(policy); err == nil {
				t.Fatal("invalid policy was accepted")
			}
		})
	}
}

func TestCyberPolicyUserRestrictionPolicyAllowsReusableBoundaries(t *testing.T) {
	for _, days := range []int{1, 30} {
		for _, hours := range []int{1, 720} {
			policy := CyberPolicyUserRestrictionPolicy{WindowDays: days, Actions: []CyberPolicyUserRestrictionRule{{Threshold: 1, DurationHours: hours}, {Threshold: 1000, AdminRelease: true}}}
			if err := ValidateCyberPolicyUserRestriction(policy); err != nil {
				t.Fatalf("days=%d hours=%d: %v", days, hours, err)
			}
		}
	}
}
