package service

import (
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"strings"
)

func ValidateUserRiskPolicySnapshot(policy UserRiskPolicySnapshot) error {
	if policy.Revision <= 0 || policy.Scope == "" || len(policy.Scope) > 64 || strings.TrimSpace(policy.Scope) != policy.Scope {
		return infraerrors.BadRequest("INVALID_USER_RISK_POLICY", "用户限制规则版本或适用范围无效")
	}
	return validateUserRiskRules(policy.WindowDays, policy.Actions)
}
