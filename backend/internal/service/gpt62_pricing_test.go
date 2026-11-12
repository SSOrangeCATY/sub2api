//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

var gpt62PriceModels = []string{"gpt-6.2-astra", "gpt-6.2-sol", "gpt-6.2-astra-pro", "gpt-6.2-sol-pro"}

func gpt62PricingCatalog(t *testing.T, bundled bool) *PricingService {
	t.Helper()
	cfg := &config.Config{}
	if bundled {
		cfg.Pricing.FallbackFile = "../../resources/model-pricing/model_prices_and_context_window.json"
	}
	s := &PricingService{cfg: cfg}
	data, _, err := s.buildPricingData([]byte(`{"gpt-5.4":{"input_cost_per_token":0.0000025,"output_cost_per_token":0.000015,"cache_read_input_token_cost":0.00000025,"litellm_provider":"openai","mode":"chat"}}`))
	require.NoError(t, err)
	s.pricingData = data
	return s
}

func TestGPT62DefaultPricingDoesNotUseOldModelRates(t *testing.T) {
	for _, path := range []string{"bundled", "dynamic_missing", "service_unavailable"} {
		t.Run(path, func(t *testing.T) {
			var catalog *PricingService
			if path != "service_unavailable" {
				catalog = gpt62PricingCatalog(t, path == "bundled")
			}
			billing := NewBillingService(&config.Config{}, catalog)
			for _, model := range gpt62PriceModels {
				for _, name := range []string{model, "models/" + model, model + "-20261112"} {
					p, err := billing.GetModelPricing(name)
					require.NoError(t, err, name)
					require.True(t, billing.HasIdentifiedTokenPricing(name), name)
					require.InDelta(t, 5e-6, p.InputPricePerToken, 1e-15, name)
					require.InDelta(t, 30e-6, p.OutputPricePerToken, 1e-15, name)
					require.InDelta(t, .5e-6, p.CacheReadPricePerToken, 1e-15, name)
					require.Zero(t, p.CacheCreationPricePerToken, name)
					require.Nil(t, p.CacheCreation12hPricePerToken, name)
					require.Zero(t, p.LongContextInputThreshold, "no unspecified ladder is inherited from an older model")
				}
			}
		})
	}
}

func TestGPT62RecordUsageAndDisplayedQuoteUseTheSameBasePrices(t *testing.T) {
	catalog := gpt62PricingCatalog(t, true)
	for _, model := range gpt62PriceModels {
		for _, identity := range []string{BillingIdentityStandard, BillingIdentityPurchase} {
			t.Run(model+"/"+identity, func(t *testing.T) {
				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				repo := &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}
				s := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, repo, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				s.billingService = NewBillingService(&config.Config{}, catalog)
				key := openAIRecordUsageAPIKeyWithGroup(s, 1, true)
				key.GroupID = &key.Group.ID
				key.Group.Platform, key.Group.RateMultiplier = PlatformOpenAI, .5
				account := &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.25)}
				err := s.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{Model: model, UpstreamModel: model, Usage: OpenAIUsage{InputTokens: 3500, CacheReadInputTokens: 2000, CacheCreationInputTokens: 500, OutputTokens: 3000}}, APIKey: key, User: &User{ID: 1, BillingIdentity: identity}, Account: account})
				require.NoError(t, err)
				require.NotNil(t, logs.lastLog)
				require.InDelta(t, .096, logs.lastLog.TotalCost, 1e-12)
				require.Equal(t, 500, logs.lastLog.CacheCreationTokens)
				require.Zero(t, logs.lastLog.CacheCreationCost)
				want := .048
				if identity == BillingIdentityPurchase {
					want = .024
				}
				require.InDelta(t, want, logs.lastLog.ActualCost, 1e-12)
				plaza := &ModelPlazaService{billingService: s.billingService, resolver: s.resolver}
				m := PlazaModel{Name: model, Platform: PlatformOpenAI}
				plaza.fillDisplayPricing(context.Background(), &m, key.Group)
				require.NotNil(t, m.Pricing)
				require.InDelta(t, 5e-6, *m.Pricing.InputPrice, 1e-15)
				require.InDelta(t, 30e-6, *m.Pricing.OutputPrice, 1e-15)
				require.InDelta(t, .5e-6, *m.Pricing.CacheReadPrice, 1e-15)
			})
		}
	}
}

func TestGPT62PricingPreservesExplicitCatalogAndOperatorOverrides(t *testing.T) {
	remote := `{"gpt-6.2-astra":{"input_cost_per_token":0.000007,"output_cost_per_token":0.000040,"cache_read_input_token_cost":0.000001,"litellm_provider":"openai","mode":"chat"}}`
	catalog := gpt62PricingCatalog(t, true)
	data, _, err := catalog.buildPricingData([]byte(remote))
	require.NoError(t, err)
	catalog.pricingData = data
	require.InDelta(t, 7e-6, catalog.GetModelPricing("gpt-6.2-astra").InputCostPerToken, 1e-15)
	patched := newPricingServiceWithOverride(t, `{"gpt-6.2-astra":{"input_cost_per_token":0.000009}}`)
	data, _, err = patched.buildPricingData([]byte(remote))
	require.NoError(t, err)
	patched.pricingData = data
	require.InDelta(t, 9e-6, patched.GetModelPricing("gpt-6.2-astra").InputCostPerToken, 1e-15)

	billing := NewBillingService(&config.Config{}, catalog)
	channel := plazaPricedChannel(1, "configured", []int64{1}, PlatformOpenAI, "gpt-6.2-astra")
	channel.ModelPricing[0].InputPrice = testPtrFloat64(3e-6)
	channels := newTestChannelService(makeStandardRepo(channel, map[int64]string{1: PlatformOpenAI}))
	resolver := NewModelPricingResolver(channels, billing)
	group := &Group{ID: 1, Platform: PlatformOpenAI, ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-6.2-astra"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(11e-6)}}}
	resolved := resolver.Resolve(context.Background(), PricingInput{Model: "gpt-6.2-astra", Group: group, GroupID: &group.ID})
	require.Equal(t, PricingSourceGroup, resolved.Source)
	require.InDelta(t, 11e-6, resolver.GetIntervalPricing(resolved, 1).InputPricePerToken, 1e-15)
	group.ModelPricing = nil
	resolved = resolver.Resolve(context.Background(), PricingInput{Model: "gpt-6.2-astra", Group: group, GroupID: &group.ID})
	require.Equal(t, PricingSourceChannel, resolved.Source)
	require.InDelta(t, 3e-6, resolver.GetIntervalPricing(resolved, 1).InputPricePerToken, 1e-15)
}

func TestGPT62PricingDoesNotClaimUnspecifiedModels(t *testing.T) {
	for _, model := range []string{"gpt-6.1-astra", "gpt-6.2-luna", "gpt-6.20-astra", "gpt-6.2-astraish", "gpt-6.2-astra-pro-max"} {
		require.False(t, isGPT62PricingModel(model), model)
	}
}

func TestGPT62ResponseModelBillingRecognizesExactFallbackDuringDegradation(t *testing.T) {
	for _, withCatalog := range []bool{false, true} {
		t.Run(boolLabel(withCatalog), func(t *testing.T) {
			var catalog *PricingService
			if withCatalog {
				catalog = gpt62PricingCatalog(t, false)
			}
			logs := &openAIRecordUsageLogRepoStub{inserted: true}
			s := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
			s.billingService = NewBillingService(&config.Config{}, catalog)
			key := openAIRecordUsageAPIKeyWithGroup(s, 1, true)
			key.GroupID = &key.Group.ID
			key.Group.RateMultiplier = .5
			err := s.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{Model: "gpt-6-astra", UpstreamModel: "gpt-6-astra", UpstreamResponseModel: "gpt-6.2-astra", Usage: OpenAIUsage{InputTokens: 3500, CacheReadInputTokens: 2000, CacheCreationInputTokens: 500, OutputTokens: 3000}}, APIKey: key, User: &User{ID: 1}, Account: &Account{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}, ChannelUsageFields: ChannelUsageFields{OriginalModel: "gpt-6-astra", BillingModelSource: BillingModelSourceResponse}})
			require.NoError(t, err)
			require.NotNil(t, logs.lastLog)
			require.InDelta(t, .096, logs.lastLog.TotalCost, 1e-12)
			require.InDelta(t, .048, logs.lastLog.ActualCost, 1e-12)
		})
	}
}
