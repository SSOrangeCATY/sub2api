package service

import "testing"

func TestUserRiskActionForCountUsesConfiguredThresholds(t *testing.T) {
	rules := []UserRiskActionRule{
		{Threshold: 1, DurationHours: 1},
		{Threshold: 5, DurationHours: 720},
		{Threshold: 1000, AdminRelease: true},
	}
	for _, test := range []struct {
		count, threshold, hours int
		manual, applies         bool
	}{
		{0, 0, 0, false, false},
		{1, 1, 1, false, true},
		{4, 1, 1, false, true},
		{5, 5, 720, false, true},
		{999, 5, 720, false, true},
		{1000, 1000, 0, true, true},
		{1001, 1000, 0, true, true},
	} {
		action, applies := UserRiskActionForCount(rules, test.count)
		if action.Threshold != test.threshold || action.DurationHours != test.hours ||
			action.AdminRelease != test.manual || applies != test.applies {
			t.Fatalf("count=%d: action=%+v applies=%t", test.count, action, applies)
		}
	}
}
