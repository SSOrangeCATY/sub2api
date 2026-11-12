package service

import (
	"crypto/rand"
	"sort"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func ValidateCyberPolicyUserRestriction(policy CyberPolicyUserRestrictionPolicy) error {
	return validateUserRiskRules(policy.WindowDays, policy.Actions)
}

func validateUserRiskRules(windowDays int, actions []UserRiskActionRule) error {
	if windowDays < 1 || windowDays > 30 {
		return infraerrors.BadRequest("INVALID_CYBER_POLICY_WINDOW", "Cyber Policy 用户限制窗口须为 1 到 30 天")
	}
	if len(actions) < 1 || len(actions) > 8 {
		return infraerrors.BadRequest("INVALID_CYBER_POLICY_ACTIONS", "Cyber Policy 至少配置 1 条、最多配置 8 条处罚规则")
	}
	previous := 0
	manualReleaseSeen := false
	for i, action := range actions {
		if action.Threshold <= previous || action.Threshold > 1000 {
			return infraerrors.BadRequest("INVALID_CYBER_POLICY_THRESHOLD", "处罚次数须递增且在 1 到 1000 之间")
		}
		if manualReleaseSeen {
			return infraerrors.BadRequest("INVALID_CYBER_POLICY_ACTION_ORDER", "管理员解封规则须放在最后")
		}
		if action.AdminRelease {
			if action.DurationHours != 0 || i != len(actions)-1 {
				return infraerrors.BadRequest("INVALID_CYBER_POLICY_MANUAL_RELEASE", "管理员解封规则不能设置自动解除期限，且必须放在最后")
			}
			manualReleaseSeen = true
		} else if action.DurationHours < 1 || action.DurationHours > 720 {
			return infraerrors.BadRequest("INVALID_CYBER_POLICY_DURATION", "自动暂停时长须为 1 到 720 小时")
		}
		previous = action.Threshold
	}
	return nil
}

func cyberPolicyEventID() string { return rand.Text() }

func cyberPolicyActionForCount(policy CyberPolicyUserRestrictionPolicy, count int) (CyberPolicyUserRestrictionRule, bool) {
	actions := append([]CyberPolicyUserRestrictionRule(nil), policy.Actions...)
	sort.Slice(actions, func(i, j int) bool { return actions[i].Threshold < actions[j].Threshold })
	selected := CyberPolicyUserRestrictionRule{}
	found := false
	for _, action := range actions {
		if count < action.Threshold {
			break
		}
		selected, found = action, true
	}
	return selected, found
}

func NewCyberPolicyUserRestrictionEventID() string {
	return cyberPolicyEventID()
}

func CyberPolicyWindowStart(policy CyberPolicyUserRestrictionPolicy, now time.Time) time.Time {
	if policy.WindowDays <= 0 {
		policy.WindowDays = 7
	}
	return now.Add(-time.Duration(policy.WindowDays) * 24 * time.Hour)
}
