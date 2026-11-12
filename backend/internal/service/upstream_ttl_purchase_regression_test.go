//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStatsExplicitFree12hCacheWriteRequiresValidBreakdown(t *testing.T) {
	for _, tc := range []struct {
		name   string
		tokens UsageTokens
		priced bool
	}{
		{"reported 12h", UsageTokens{CacheCreationTokens: 100, CacheCreation12hTokens: 100}, true},
		{"reported 30m", UsageTokens{CacheCreationTokens: 100, CacheCreation30mTokens: 100}, false},
		{"missing TTL", UsageTokens{CacheCreationTokens: 100}, false},
		{"invalid TTL", UsageTokens{CacheCreationTokens: 100, CacheCreation12hTokens: 99}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, intervals := range []bool{false, true} {
				pricing := &ChannelModelPricing{CacheWrite12hPrice: float64Ptr(0)}
				if intervals {
					pricing = &ChannelModelPricing{Intervals: []PricingInterval{{CacheWrite12hPrice: float64Ptr(0)}}}
				}
				cost := calculateStatsCost(pricing, tc.tokens, 1)
				if tc.priced {
					require.NotNil(t, cost)
					require.Zero(t, *cost)
				} else {
					require.Nil(t, cost)
				}
			}
		})
	}
}

func TestPurchaseOpenAIRecordUsagePreservesFree12hProcurement(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{}
	svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
	svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"gpt-6-luna": {CacheCreationPricePerToken: 0.001}})
	channel := &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"gpt-6-luna"}, CacheWrite12hPrice: float64Ptr(0)}}}}}
	svc.channelService = purchaseCostChannelService(channel, PlatformOpenAI)
	svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
	group := &Group{ID: 10, Platform: PlatformOpenAI, RateMultiplier: 9, Hydrated: true}
	require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
		Result: &OpenAIForwardResult{RequestID: "purchase-free-12h", Model: "gpt-6-luna", Usage: OpenAIUsage{CacheCreationInputTokens: 100, CacheCreation12hInputTokens: 100}},
		APIKey: &APIKey{ID: 2, Quota: 100, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: BillingIdentityPurchase},
		APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformOpenAI, RateMultiplier: float64Ptr(0.3)},
	}))
	require.NotNil(t, usageRepo.lastLog.AccountStatsCost)
	require.Zero(t, *usageRepo.lastLog.AccountStatsCost)
	require.Zero(t, usageRepo.lastLog.ActualCost)
	require.Zero(t, billingRepo.lastCmd.BalanceCost)
	require.Zero(t, billingRepo.lastCmd.APIKeyQuotaCost)
}
