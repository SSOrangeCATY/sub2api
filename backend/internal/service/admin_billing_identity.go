package service

import (
	"context"
	"fmt"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

type BillingIdentityMigrationResult struct {
	BackupID      int64   `json:"backup_id"`
	MigratedUsers int     `json:"migrated_users"`
	ClearedRates  int64   `json:"cleared_rates"`
	UserIDs       []int64 `json:"-"`
}

// BillingIdentityMigrator is optional for repository/service implementations.
type BillingIdentityMigrator interface {
	MigratePersonalRatesToPurchase(ctx context.Context, actorAdminID int64) (*BillingIdentityMigrationResult, error)
}

func (s *adminServiceImpl) MigratePersonalRatesToPurchase(ctx context.Context, actorAdminID int64) (*BillingIdentityMigrationResult, error) {
	repo, ok := s.userRepo.(BillingIdentityMigrator)
	if !ok {
		return nil, fmt.Errorf("billing identity migration unavailable")
	}
	result, err := repo.MigratePersonalRatesToPurchase(ctx, actorAdminID)
	if err != nil {
		return nil, err
	}
	if resetter, ok := s.authCacheInvalidator.(interface{ InvalidatePersonalRateCaches(context.Context) }); ok && result.ClearedRates > 0 {
		resetter.InvalidatePersonalRateCaches(ctx)
	}
	if s.authCacheInvalidator != nil {
		for _, id := range result.UserIDs {
			s.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, id)
		}
	}
	logger.LegacyPrintf("service.admin", "audit: billing identity migration actor_admin_id=%d backup_id=%d users=%d cleared_rates=%d", actorAdminID, result.BackupID, result.MigratedUsers, result.ClearedRates)
	return result, nil
}
