package middleware

import (
	"context"
	"crypto/rand"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// Existing authentication tests need an explicit durable-admission capability.
// Production still rejects a missing capability; these fixtures do not change
// the production constructor or its fail-closed behavior.
type authRiskTestUserRepo struct {
	service.UserRepository
}

func (*authRiskTestUserRepo) BeginUserRiskRequest(_ context.Context, userID int64, policyKey, scope string) (service.UserRiskAdmission, error) {
	return service.UserRiskAdmission{
		UserID: userID, PolicyKey: policyKey, Scope: scope, EventID: rand.Text(),
		Generation: 1, AdmittedAt: time.Now(),
	}, nil
}

func newAPIKeyServiceWithRiskTestRepo(
	keyRepo service.APIKeyRepository,
	userRepo service.UserRepository,
	groupRepo service.GroupRepository,
	subscriptionRepo service.UserSubscriptionRepository,
	groupRateRepo service.UserGroupRateRepository,
	cache service.APIKeyCache,
	cfg *config.Config,
) *service.APIKeyService {
	if userRepo == nil {
		userRepo = &authRiskTestUserRepo{}
	}
	return service.NewAPIKeyService(keyRepo, userRepo, groupRepo, subscriptionRepo, groupRateRepo, cache, cfg)
}
