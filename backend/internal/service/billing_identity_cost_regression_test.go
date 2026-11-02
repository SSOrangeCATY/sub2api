//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func purchaseCostChannelService(channel *Channel, platform string) *ChannelService {
	channel.GroupIDs = []int64{10}
	for i := range channel.ModelPricing {
		channel.ModelPricing[i].Platform = platform
	}
	service := &ChannelService{}
	service.cache.Store(populateChannelCache([]Channel{*channel}, map[int64]string{10: platform}))
	return service
}

func TestPurchaseRecordUsagePreservesOneHourCacheCostAndQuote(t *testing.T) {
	for _, source := range []string{"model_file", "account_rule"} {
		t.Run(source, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
			svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {
				CacheCreationPricePerToken: 0.001, CacheCreation5mPrice: 0.001, CacheCreation1hPrice: 0.002, SupportsCacheBreakdown: true,
			}})
			channel := &Channel{ID: 1, Status: StatusActive}
			if source == "account_rule" {
				channel.AccountStatsPricingRules = []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, CacheWritePrice: float64Ptr(0.001), CacheWrite1hPrice: float64Ptr(0.002)}}}}
			}
			svc.channelService = purchaseCostChannelService(channel, PlatformAnthropic)
			svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
			group := &Group{ID: 10, Platform: PlatformAnthropic, RateMultiplier: 9, Hydrated: true}
			input := &RecordUsageInput{
				Result: &ForwardResult{RequestID: "purchase-cache-" + source, Model: "claude-sonnet-4", Usage: ClaudeUsage{CacheCreationInputTokens: 200, CacheCreation5mTokens: 80, CacheCreation1hTokens: 120}},
				APIKey: &APIKey{ID: 2, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: BillingIdentityPurchase},
				APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformAnthropic, RateMultiplier: float64Ptr(0.3)},
			}
			quote := svc.QuoteUsageCost(context.Background(), input)
			require.NoError(t, svc.RecordUsage(context.Background(), input))
			require.NotNil(t, usageRepo.lastLog.AccountStatsCost)
			require.InDelta(t, 0.32, *usageRepo.lastLog.AccountStatsCost, 1e-12)
			require.InDelta(t, 0.096, usageRepo.lastLog.ActualCost, 1e-12)
			require.InDelta(t, quote, billingRepo.lastCmd.BalanceCost, 1e-12)
			require.InDelta(t, 0.096, quote, 1e-12)
		})
	}
}

func TestPurchaseRecordUsagePreservesSearchAndStorageOnce(t *testing.T) {
	for _, source := range []string{"model_file", "account_rule", "sales_card"} {
		for _, identity := range []string{BillingIdentityPurchase, BillingIdentityStandard} {
			t.Run(source+"/"+identity, func(t *testing.T) {
				usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
				billingRepo := &openAIRecordUsageBillingRepoStub{}
				svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
				svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"gemini-3.8-flash": {InputPricePerToken: 0.001}})
				channel := &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: source == "sales_card"}
				if source == "sales_card" {
					channel.ModelPricing = []ChannelModelPricing{{Models: []string{"gemini-3.8-flash"}, InputPrice: float64Ptr(0.004)}}
				}
				if source == "account_rule" {
					channel.AccountStatsPricingRules = []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"gemini-3.8-flash"}, InputPrice: float64Ptr(0.002)}}}}
				}
				svc.channelService = purchaseCostChannelService(channel, PlatformGemini)
				svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
				group := &Group{ID: 10, Platform: PlatformGemini, RateMultiplier: 2, SearchPricePer1k: float64Ptr(10), Hydrated: true}
				input := &RecordUsageInput{
					Result: &ForwardResult{RequestID: "purchase-search-" + source + identity, Model: "gemini-3.8-flash", Usage: ClaudeUsage{InputTokens: 100}, SearchCount: 100, CacheStorageTokenHours: 400000},
					APIKey: &APIKey{ID: 2, Quota: 100, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: identity},
					APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformGemini, RateMultiplier: float64Ptr(0.3)},
				}
				quote := svc.QuoteUsageCost(context.Background(), input)
				require.NoError(t, svc.RecordUsage(context.Background(), input))
				wantStats := 1.3 // 0.1 token + 1 search + 0.2 storage
				if source == "account_rule" {
					wantStats = 1.4
				}
				if source == "sales_card" {
					wantStats = 1.6
				}
				require.InDelta(t, wantStats, *usageRepo.lastLog.AccountStatsCost, 1e-12)
				wantCharge := wantStats * 0.3
				if identity == BillingIdentityStandard {
					wantCharge = 1.3 * 2
					if source == "sales_card" {
						wantCharge = 1.6 * 2
					}
				}
				require.InDelta(t, wantCharge, usageRepo.lastLog.ActualCost, 1e-12)
				require.InDelta(t, wantCharge, billingRepo.lastCmd.BalanceCost, 1e-12)
				require.InDelta(t, wantCharge, billingRepo.lastCmd.APIKeyQuotaCost, 1e-12)
				require.InDelta(t, wantCharge, quote, 1e-12)
			})
		}
	}
}

func TestPurchaseOpenAIRecordUsagePreservesSearchOnce(t *testing.T) {
	for _, source := range []string{"model_file", "account_rule", "sales_card"} {
		t.Run(source, func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{}
			svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {InputPricePerToken: 0.001}})
			channel := &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: source == "sales_card"}
			if source == "sales_card" {
				channel.ModelPricing = []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: float64Ptr(0.004)}}
			}
			if source == "account_rule" {
				channel.AccountStatsPricingRules = []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: float64Ptr(0.002)}}}}
			}
			svc.channelService = purchaseCostChannelService(channel, PlatformOpenAI)
			svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
			group := &Group{ID: 10, Platform: PlatformOpenAI, RateMultiplier: 9, SearchPricePer1k: float64Ptr(10), Hydrated: true}
			require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
				Result: &OpenAIForwardResult{RequestID: "purchase-openai-search-" + source, Model: "claude-sonnet-4", Usage: OpenAIUsage{InputTokens: 100}, SearchCount: 100},
				APIKey: &APIKey{ID: 2, Quota: 100, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: BillingIdentityPurchase},
				APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformOpenAI, RateMultiplier: float64Ptr(0.3)},
			}))
			wantStats := 1.1
			if source == "account_rule" {
				wantStats = 1.2
			}
			if source == "sales_card" {
				wantStats = 1.4
			}
			require.InDelta(t, wantStats, *usageRepo.lastLog.AccountStatsCost, 1e-12)
			require.InDelta(t, wantStats*0.3, billingRepo.lastCmd.BalanceCost, 1e-12)
			require.InDelta(t, wantStats*0.3, billingRepo.lastCmd.APIKeyQuotaCost, 1e-12)
		})
	}
}

func TestPurchaseRecordUsageExplicitZeroAccountCards(t *testing.T) {
	for _, mode := range []BillingMode{BillingModeToken, BillingModePerRequest} {
		t.Run(string(mode), func(t *testing.T) {
			usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
			billingRepo := &openAIRecordUsageBillingRepoStub{}
			svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
			svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {InputPricePerToken: 0.001}})
			svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, BillingMode: mode, InputPrice: float64Ptr(0), PerRequestPrice: float64Ptr(0)}}}}}, 10, PlatformAnthropic)
			svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
			group := &Group{ID: 10, Platform: PlatformAnthropic, RateMultiplier: 9, Hydrated: true}
			require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
				Result: &ForwardResult{RequestID: "purchase-free-" + string(mode), Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100}},
				APIKey: &APIKey{ID: 2, Quota: 100, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: BillingIdentityPurchase},
				APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformAnthropic, RateMultiplier: float64Ptr(0.3)},
			}))
			require.NotNil(t, usageRepo.lastLog.AccountStatsCost)
			require.Zero(t, *usageRepo.lastLog.AccountStatsCost)
			require.Zero(t, usageRepo.lastLog.ActualCost)
			require.Zero(t, billingRepo.lastCmd.BalanceCost)
			require.Zero(t, billingRepo.lastCmd.APIKeyQuotaCost)
		})
	}
}

func TestStatsZeroRequiresAnExplicitPriceForMeteredUsage(t *testing.T) {
	require.Nil(t, calculateTokenStatsCost(&ChannelModelPricing{}, UsageTokens{InputTokens: 10}))
	require.Nil(t, calculateTokenStatsCost(&ChannelModelPricing{OutputPrice: float64Ptr(0)}, UsageTokens{InputTokens: 10}))
	require.Nil(t, calculateTokenStatsCost(&ChannelModelPricing{InputPrice: float64Ptr(0)}, UsageTokens{}))
	require.NotNil(t, calculateTokenStatsCost(&ChannelModelPricing{InputPrice: float64Ptr(0)}, UsageTokens{InputTokens: 10}))
	require.Nil(t, calculatePerRequestStatsCost(&ChannelModelPricing{}, 1))
}

func TestPurchaseInflightSearchPreservesAdditiveCost(t *testing.T) {
	for _, source := range []string{"model_file", "account_rule", "sales_card"} {
		t.Run(source, func(t *testing.T) {
			billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {InputPricePerToken: 0.001, OutputPricePerToken: 0.001}})
			channel := &Channel{ID: 1, Status: StatusActive, ApplyPricingToAccountStats: source == "sales_card"}
			if source == "account_rule" {
				channel.AccountStatsPricingRules = []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: float64Ptr(0.002), OutputPrice: float64Ptr(0.002)}}}}
			}
			cs := purchaseCostChannelService(channel, PlatformAnthropic)
			group := &Group{ID: 10, Platform: PlatformAnthropic, RateMultiplier: 9, SearchPricePer1k: float64Ptr(10)}
			key := &APIKey{GroupID: &group.ID, Group: group, User: &User{BillingIdentity: BillingIdentityPurchase}}
			d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(cs, billing),
				purchaseAccounts: func(context.Context, *APIKey, string) ([]Account, error) {
					return []Account{{ID: 4, RateMultiplier: float64Ptr(0.3)}}, nil
				},
				accountStatsEstimate: func(ctx context.Context, accountID, groupID int64, model string, tokens UsageTokens, count int, total float64, at time.Time, long bool) *float64 {
					return resolveAccountStatsCost(ctx, cs, billing, accountID, groupID, model, tokens, count, total, "", at, long)
				},
			}
			got, ok := d.estimate(context.Background(), key, InflightEstimateRequest{Model: "claude-sonnet-4", BodyBytes: 40, MaxTokens: 10, SearchCalls: 100})
			require.True(t, ok)
			wantBase := 1.02
			if source == "account_rule" {
				wantBase = 1.04
			}
			require.InDelta(t, wantBase*0.3, got, 1e-12)
		})
	}
}

func TestPurchaseRecordUsageFreeTokenCardStillChargesSearch(t *testing.T) {
	usageRepo := &openAIRecordUsageLogRepoStub{inserted: true}
	billingRepo := &openAIRecordUsageBillingRepoStub{}
	svc := newGatewayRecordUsageServiceWithBillingRepoForTest(usageRepo, billingRepo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{})
	svc.billingService = newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {InputPricePerToken: 0.001}})
	svc.channelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"claude-sonnet-4"}, InputPrice: float64Ptr(0)}}}}}, 10, PlatformAnthropic)
	svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
	group := &Group{ID: 10, Platform: PlatformAnthropic, RateMultiplier: 9, SearchPricePer1k: float64Ptr(10), Hydrated: true}
	require.NoError(t, svc.RecordUsage(context.Background(), &RecordUsageInput{
		Result: &ForwardResult{RequestID: "purchase-free-token-search", Model: "claude-sonnet-4", Usage: ClaudeUsage{InputTokens: 100}, SearchCount: 100},
		APIKey: &APIKey{ID: 2, Quota: 100, GroupID: &group.ID, Group: group}, User: &User{ID: 3, BillingIdentity: BillingIdentityPurchase},
		APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, Account: &Account{ID: 4, Platform: PlatformAnthropic, RateMultiplier: float64Ptr(0.3)},
	}))
	require.InDelta(t, 1.0, *usageRepo.lastLog.AccountStatsCost, 1e-12)
	require.InDelta(t, 0.3, billingRepo.lastCmd.BalanceCost, 1e-12)
}

func TestStatsModelFileExplicitZeroIsNotUnknownPricing(t *testing.T) {
	billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{"claude-sonnet-4": {InputPricePerToken: 0}})
	got := tryModelFilePricing(billing, "claude-sonnet-4", UsageTokens{InputTokens: 100}, "", time.Time{}, true)
	require.NotNil(t, got)
	require.Zero(t, *got)
	require.Nil(t, tryModelFilePricing(billing, "unpriced-model-xyz", UsageTokens{InputTokens: 100}, "", time.Time{}, true))
	require.Nil(t, tryModelFilePricing(billing, "claude-sonnet-4", UsageTokens{}, "", time.Time{}, true))
}
