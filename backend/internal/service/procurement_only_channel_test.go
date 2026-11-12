//go:build unit

package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func procurementOnlyFixture(t *testing.T) (*Channel, *Group, []Account, *PricingService) {
	t.Helper()
	card := ChannelModelPricing{Platform: PlatformOpenAI, SourceCurrency: "USD", Models: []string{"gpt-6.2-astra", "gpt-6.2-sol", "gpt-6.2-astra-pro", "gpt-6.2-sol-pro"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(5e-6), OutputPrice: testPtrFloat64(30e-6), CacheReadPrice: testPtrFloat64(.5e-6), CacheWritePrice: testPtrFloat64(0)}
	ch := &Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{201}, BillingModelSource: BillingModelSourceRequested, ApplyPricingToAccountStats: true, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{101}, Pricing: []ChannelModelPricing{card}}}}
	g := &Group{ID: 201, Platform: PlatformOpenAI, RateMultiplier: .85, LongContextPricingEnabled: true}
	accounts := []Account{{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{201}, RateMultiplier: testPtrFloat64(.8), Credentials: map[string]any{"model_mapping": map[string]any{"gpt-6.2-astra": "gpt-6.2-astra", "gpt-6.1-sol": "gpt-6.1-sol"}}}}
	catalog := newStubPricingServiceFromJSON(t, `{"gpt-5.4":{"input_cost_per_token":0.0000025,"output_cost_per_token":0.000015,"cache_read_input_token_cost":0.00000025},"gpt-6.2-astra":{"input_cost_per_token":0.0000025,"output_cost_per_token":0.000015,"cache_read_input_token_cost":0.00000025},"gpt-6.1-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.000010,"cache_read_input_token_cost":0.0000001}}`)
	return ch, g, accounts, catalog
}

func TestProcurementOnlyChannel_KeepsAccountCatalogAndCustomerPrice(t *testing.T) {
	ch, g, accounts, catalog := procurementOnlyFixture(t)
	plaza := newPlazaService([]Channel{*ch}, []Group{*g}, catalog)
	plaza.accountRepo = &plazaDiscoveryAccounts{accounts: accounts}
	groups, err := plaza.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Models, 2)
	cs := newTestChannelServiceForStats(t, ch, g.ID, g.Platform)
	cs.groupRepo = quoteGroupRepo{group: g}
	bs := NewBillingService(&config.Config{}, catalog)
	quotes := NewModelPriceQuoteService(&quoteCandidateRepo{accounts: accounts}, cs, bs, NewModelPricingResolver(cs, bs))
	quotes.FillDirectPrices(context.Background(), groups)
	m := plazaModelsByName(groups[0].Models)["gpt-6.2-astra"]
	require.NotNil(t, m.PriceQuote, "a cost-only channel must retain the account-aware customer quote")
	require.Equal(t, "customer", m.PriceQuote.Kind)
	require.Equal(t, "quoted", m.PriceQuote.Status)
	require.InDelta(t, 2.5e-6, *m.PriceQuote.Variants[0].Pricing.InputPrice, 1e-15, "supplier cost must never replace the standard sales card")
	quotes.FillPurchasePrices(context.Background(), groups)
	m = plazaModelsByName(groups[0].Models)["gpt-6.2-astra"]
	require.Equal(t, "account_cost", m.PriceQuote.Kind)
	require.InDelta(t, 4e-6, *m.PriceQuote.Variants[0].Pricing.InputPrice, 1e-15)
}

func TestProcurementOnlyChannel_RecordUsageChangesOnlyMatchedAccountCost(t *testing.T) {
	ch, g, accounts, catalog := procurementOnlyFixture(t)
	for _, name := range []string{"gpt-6.2-astra", "gpt-6.1-sol"} {
		for _, identity := range []string{BillingIdentityStandard, BillingIdentityPurchase} {
			t.Run(name+"/"+identity, func(t *testing.T) {
				logs := &openAIRecordUsageLogRepoStub{inserted: true}
				s := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
				s.billingService = NewBillingService(&config.Config{}, catalog)
				s.channelService = newTestChannelServiceForStats(t, ch, g.ID, g.Platform)
				s.resolver = NewModelPricingResolver(s.channelService, s.billingService)
				key := &APIKey{ID: 1, GroupID: &g.ID, Group: g}
				err := s.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{Model: name, UpstreamModel: name, Usage: OpenAIUsage{InputTokens: 3000, CacheReadInputTokens: 2000, OutputTokens: 3000}}, APIKey: key, User: &User{ID: 1, BillingIdentity: identity}, Account: &accounts[0]})
				require.NoError(t, err)
				require.NotNil(t, logs.lastLog)
				base := .0322
				accountBase := base
				if name == "gpt-6.2-astra" {
					base = .048
					accountBase = .096
				}
				require.InDelta(t, base, logs.lastLog.TotalCost, 1e-12)
				debit := base * .85
				if identity == BillingIdentityPurchase {
					debit = accountBase * .8
				}
				require.InDelta(t, debit, logs.lastLog.ActualCost, 1e-12)
			})
		}
	}
	other := *ch
	other.AccountStatsPricingRules[0].AccountIDs = []int64{999}
	require.Nil(t, tryCustomRules(&other, 101, 201, PlatformOpenAI, "gpt-6.2-astra", UsageTokens{InputTokens: 100}, 1), "an account-specific rule must not act as a group-wide rule")
}

func TestProcurementOnlyChannel_PreservesEveryNonTargetModelAmount(t *testing.T) {
	ch, g, accounts, catalog := procurementOnlyFixture(t)
	names := []string{"gpt-5.6-pro", "gpt-5.6-sol", "gpt-6-astra", "gpt-6.1-pro", "gpt-6.1-sol", "gpt-6.1-terra", "gpt-6-astra-pro"}
	for _, name := range names {
		for _, identity := range []string{BillingIdentityStandard, BillingIdentityPurchase} {
			t.Run(name+"/"+identity, func(t *testing.T) {
				record := func(channel *Channel) *UsageLog {
					logs := &openAIRecordUsageLogRepoStub{inserted: true}
					s := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, &openAIRecordUsageBillingRepoStub{result: &UsageBillingApplyResult{Applied: true}}, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					s.billingService = NewBillingService(&config.Config{}, catalog)
					if channel != nil {
						s.channelService = newTestChannelServiceForStats(t, channel, g.ID, g.Platform)
					}
					s.resolver = NewModelPricingResolver(s.channelService, s.billingService)
					err := s.RecordUsage(context.Background(), &OpenAIRecordUsageInput{Result: &OpenAIForwardResult{Model: name, UpstreamModel: name, Usage: OpenAIUsage{InputTokens: 300000, CacheReadInputTokens: 2000, OutputTokens: 1000}}, APIKey: &APIKey{ID: 1, GroupID: &g.ID, Group: g}, User: &User{ID: 1, BillingIdentity: identity}, Account: &accounts[0]})
					require.NoError(t, err)
					require.NotNil(t, logs.lastLog)
					return logs.lastLog
				}
				before, after := record(nil), record(ch)
				require.InDelta(t, before.TotalCost, after.TotalCost, 1e-12)
				require.InDelta(t, before.ActualCost, after.ActualCost, 1e-12)
				require.Nil(t, before.AccountStatsCost)
				require.NotNil(t, after.AccountStatsCost, "neutral costs become explicit but retain the same amount")
			})
		}
	}
}

func TestProcurementOnlyChannel_CatalogPredicateDoesNotExpandSalesRestrictions(t *testing.T) {
	require.False(t, channelHasSalesCatalog(nil))
	require.False(t, channelHasSalesCatalog(&Channel{AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{101}}}}))
	require.True(t, channelHasSalesCatalog(&Channel{RestrictModels: true}))
	require.True(t, channelHasSalesCatalog(&Channel{ModelPricing: []ChannelModelPricing{{Models: []string{"gpt-*"}}}}))
	require.True(t, channelHasSalesCatalog(&Channel{ModelMapping: map[string]map[string]string{PlatformOpenAI: {"gpt-*": "*"}}}))
}

func TestProcurementOnlyChannel_CostCardIsMutableAndAccountScoped(t *testing.T) {
	ch, g, accounts, catalog := procurementOnlyFixture(t)
	cs := newTestChannelServiceForStats(t, ch, g.ID, g.Platform)
	bs := NewBillingService(&config.Config{}, catalog)
	quote := NewModelPriceQuoteService(nil, cs, bs, NewModelPricingResolver(cs, bs))
	for _, name := range ch.AccountStatsPricingRules[0].Pricing[0].Models {
		q := quote.quoteModel(context.Background(), g, ch, &PlazaModel{Name: name, Platform: PlatformOpenAI}, accounts)
		if name == "gpt-6.2-astra" {
			require.Equal(t, "quoted", q.Status)
			require.InDelta(t, 4e-6, *q.Variants[0].Pricing.InputPrice, 1e-15)
		}
		value := tryCustomRules(ch, 101, 201, PlatformOpenAI, name, UsageTokens{InputTokens: 1000, CacheReadTokens: 2000, OutputTokens: 3000}, 1)
		require.NotNil(t, value)
		require.InDelta(t, .096, *value, 1e-12)
	}
	ch.AccountStatsPricingRules[0].Pricing[0].InputPrice = testPtrFloat64(6e-6)
	q := quote.quoteModel(context.Background(), g, ch, &PlazaModel{Name: "gpt-6.2-astra", Platform: PlatformOpenAI}, accounts)
	require.Equal(t, "quoted", q.Status)
	require.InDelta(t, 4.8e-6, *q.Variants[0].Pricing.InputPrice, 1e-15, "edited supplier base price is used without a deployment")
	accounts[0].RateMultiplier = testPtrFloat64(.5)
	q = quote.quoteModel(context.Background(), g, ch, &PlazaModel{Name: "gpt-6.2-astra", Platform: PlatformOpenAI}, accounts)
	require.InDelta(t, 3e-6, *q.Variants[0].Pricing.InputPrice, 1e-15, "live account rate is applied once and is not frozen into the base card")
}
