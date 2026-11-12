package service

import (
	"context"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type userRiskAdmissionRepository interface {
	BeginUserRiskRequest(context.Context, int64, string, string) (UserRiskAdmission, error)
}

// Every logical API request receives one server identity before upstream work.
// The policy and scope are chosen here, never supplied by the API client.
func (s *APIKeyService) BeginUserRiskRequest(ctx context.Context, userID int64) (UserRiskAdmission, error) {
	empty := UserRiskAdmission{}
	if userID <= 0 {
		return empty, infraerrors.BadRequest("INVALID_USER_ID", "用户编号无效")
	}
	if s == nil || s.userRepo == nil {
		return empty, infraerrors.InternalServer("USER_RISK_STORE_UNAVAILABLE", "用户访问限制检查暂不可用")
	}
	repo, ok := s.userRepo.(userRiskAdmissionRepository)
	if !ok {
		return empty, infraerrors.InternalServer("USER_RISK_STORE_UNAVAILABLE", "用户访问限制检查暂不可用")
	}
	admission, err := repo.BeginUserRiskRequest(ctx, userID, UserRiskPolicyCyber, UserRiskScopeAPI)
	if err != nil {
		return empty, err
	}
	if admission.UserID != userID || admission.PolicyKey != UserRiskPolicyCyber || admission.Scope != UserRiskScopeAPI ||
		admission.Generation <= 0 || admission.EventID == "" || admission.AdmittedAt.IsZero() {
		return empty, infraerrors.InternalServer("INVALID_USER_RISK_ADMISSION", "用户请求准入凭证无效")
	}
	return admission, nil
}
