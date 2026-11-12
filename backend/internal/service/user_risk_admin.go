package service

import (
	"context"
	"errors"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

func (s *ContentModerationService) userRiskRepository() (UserRiskPolicyRepository, error) {
	if s == nil || s.userRiskRepo == nil {
		return nil, infraerrors.InternalServer("USER_RISK_REPOSITORY_UNAVAILABLE", "用户限制记录暂不可用")
	}
	return s.userRiskRepo, nil
}

func (s *ContentModerationService) ListUserRiskRestrictions(ctx context.Context, filter UserRiskListFilter) (*UserRiskRestrictionPage, error) {
	repo, err := s.userRiskRepository()
	if err != nil {
		return nil, err
	}
	if s.settingRepo == nil {
		return nil, infraerrors.InternalServer("USER_RISK_CONFIG_UNAVAILABLE", "用户限制规则暂不可用")
	}
	cfg, err := s.loadConfig(ctx)
	if err != nil {
		return nil, err
	}
	filter.PolicyKey, filter.WindowDays = UserRiskPolicyCyber, cfg.CyberPolicyUserRestriction.WindowDays
	result, err := repo.ListUserRiskRestrictions(ctx, filter)
	if err != nil {
		return nil, err
	}
	result.WindowDays, result.PolicyRevision = filter.WindowDays, cfg.Revision
	return result, nil
}

func (s *ContentModerationService) ListUserRiskActions(ctx context.Context, filter UserRiskListFilter) (*UserRiskActionPage, error) {
	repo, err := s.userRiskRepository()
	if err != nil {
		return nil, err
	}
	filter.PolicyKey = UserRiskPolicyCyber
	return repo.ListUserRiskActions(ctx, filter)
}

func (s *ContentModerationService) ReleaseUserRiskRestriction(ctx context.Context, input UserRiskReleaseInput) (*UserRiskReleaseResult, error) {
	repo, err := s.userRiskRepository()
	if err != nil {
		return nil, err
	}
	input.PolicyKey = UserRiskPolicyCyber
	result, err := repo.ReleaseUserRiskRestriction(ctx, input)
	if errors.Is(err, ErrUserNotFound) {
		return nil, infraerrors.NotFound("USER_NOT_FOUND", "用户不存在")
	}
	if err != nil {
		return nil, err
	}
	if result.Released && s.authCacheInvalidator != nil {
		s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, input.UserID)
	}
	return result, nil
}
