//go:build unit

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestPartialPlazaCatalog_IncludesAccountModelsWithoutDuplicatingConfiguredPrice(t *testing.T) {
	card := ChannelModelPricing{Platform: PlatformOpenAI, SourceCurrency: "CNY", Models: []string{"priced-model"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6)}
	channel := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{201}, ModelPricing: []ChannelModelPricing{card}}
	group := Group{ID: 201, Platform: PlatformOpenAI}
	s := newPlazaService([]Channel{channel}, []Group{group}, nil)
	s.accountRepo = &plazaDiscoveryAccounts{accounts: []Account{{Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true, GroupIDs: []int64{201}, Credentials: map[string]any{"model_mapping": map[string]any{"priced-model": "priced-model", "other-model": "other-model", "MiniMax-example": "MiniMax-example", "minimax-example": "minimax-example"}}}}}
	groups, err := s.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Len(t, groups[0].Models, 3, "partial pricing must not hide configured account models; case aliases must not duplicate")
	m := plazaModelsByName(groups[0].Models)["priced-model"]
	require.Equal(t, "CNY", m.Pricing.SourceCurrency)
	require.InDelta(t, 2e-6, *m.Pricing.InputPrice, 1e-15)
	channel.RestrictModels = true
	restricted := newPlazaService([]Channel{channel}, []Group{group}, nil)
	restricted.accountRepo = &plazaDiscoveryAccounts{err: errors.New("restricted catalogs must not expand native models")}
	groups, err = restricted.ListGroups(context.Background())
	require.NoError(t, err)
	require.Len(t, groups[0].Models, 1)
}

func TestPartialPlazaCatalog_CustomerAliasUsesActualUpstreamSalesCard(t *testing.T) {
	card := ChannelModelPricing{Platform: PlatformOpenAI, SourceCurrency: "CNY", Models: []string{"supplier-model"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6), CacheReadPrice: testPtrFloat64(.04e-6)}
	ch := &Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{201}, BillingModelSource: BillingModelSourceUpstream, ModelPricing: []ChannelModelPricing{card}, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{101}, Pricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{"supplier-model"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(99e-6)}}}}}
	g := &Group{ID: 201, Platform: PlatformOpenAI, RateMultiplier: .075}
	account := Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.06), Credentials: map[string]any{"model_mapping": map[string]any{"public-alias": "supplier-model"}}}
	catalog := newStubPricingServiceFromJSON(t, `{"public-alias":{"input_cost_per_token":0.0000001,"output_cost_per_token":0.0000002}}`)
	bs := NewBillingService(&config.Config{}, catalog)
	cs := newTestChannelService(makeStandardRepo(*ch, map[int64]string{g.ID: g.Platform}))
	cs.groupRepo = quoteGroupRepo{group: g}
	quotes := NewModelPriceQuoteService(&quoteCandidateRepo{accounts: []Account{account}}, cs, bs, NewModelPricingResolver(cs, bs))
	groups := []PlazaGroup{{ID: g.ID, Models: []PlazaModel{{Name: "public-alias", Platform: PlatformOpenAI}}}}
	quotes.FillDirectPrices(context.Background(), groups)
	q := groups[0].Models[0].PriceQuote
	require.NotNil(t, q)
	require.Equal(t, "quoted", q.Status)
	require.Equal(t, "customer", q.Kind)
	require.Equal(t, "CNY", q.Variants[0].Pricing.SourceCurrency)
	require.InDelta(t, 2e-6, *q.Variants[0].Pricing.InputPrice, 1e-15, "use the upstream sales card, never alias fallback or supplier cost rules")
	require.InDelta(t, 8e-6, *q.Variants[0].Pricing.OutputPrice, 1e-15)
}

func TestCustomerQuote_OpenAIMappingMatchesForwardingMode(t *testing.T) {
	for _, mode := range []struct {
		name  string
		extra map[string]any
		want  float64
	}{
		{"passthrough_ignores_stale_mapping", map[string]any{"openai_passthrough": true, "openai_responses_supported": true}, 2e-6},
		{"ordinary_mapping", map[string]any{"openai_responses_supported": true}, 8e-6},
		{"raw_chat_precedes_passthrough", map[string]any{"openai_passthrough": true, "openai_responses_supported": false}, 8e-6},
	} {
		t.Run(mode.name, func(t *testing.T) {
			g := &Group{ID: 201, Platform: PlatformOpenAI}
			ch := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{g.ID}, BillingModelSource: BillingModelSourceUpstream, ModelPricing: []ChannelModelPricing{
				{Platform: PlatformOpenAI, Models: []string{"public-model"}, SourceCurrency: "CNY", InputPrice: testPtrFloat64(2e-6)},
				{Platform: PlatformOpenAI, Models: []string{"mapped-model"}, SourceCurrency: "CNY", InputPrice: testPtrFloat64(8e-6)},
			}}
			cs := newTestChannelService(makeStandardRepo(ch, map[int64]string{g.ID: g.Platform}))
			cs.groupRepo = quoteGroupRepo{group: g}
			bs := NewBillingService(&config.Config{}, nil)
			a := Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Extra: mode.extra, Credentials: map[string]any{"model_mapping": map[string]any{"public-model": "mapped-model"}}}
			s := NewModelPriceQuoteService(&quoteCandidateRepo{accounts: []Account{a}}, cs, bs, NewModelPricingResolver(cs, bs))
			groups := []PlazaGroup{{ID: g.ID, Models: []PlazaModel{{Name: "public-model", Platform: PlatformOpenAI}}}}
			s.FillDirectPrices(context.Background(), groups)
			q := groups[0].Models[0].PriceQuote
			require.Equal(t, "quoted", q.Status)
			require.InDelta(t, mode.want, *q.Variants[0].Pricing.InputPrice, 1e-15)
		})
	}
}

func TestCustomerQuote_SubscriptionExemptionRetainsChannelTimeSchedule(t *testing.T) {
	model := "gpt-6.1-sol"
	g := &Group{ID: 201, Platform: PlatformOpenAI, LongContextPricingEnabled: true}
	ch := Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{g.ID}, ModelPricing: []ChannelModelPricing{{Platform: PlatformOpenAI, Models: []string{model}, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6), TimePricing: &ChannelTimePricing{Timezone: "UTC", Periods: []ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "00:00", Multiplier: 2}}}}}}
	// Split the full day into valid adjacent windows, so the regression cannot
	// accidentally pass just because the machine clock is outside peak hours.
	ch.ModelPricing[0].TimePricing.Periods = []ChannelTimePricingPeriod{{StartTime: "00:00", EndTime: "12:00", Multiplier: 2}, {StartTime: "12:00", EndTime: "00:00", Multiplier: 2}}
	cs := newTestChannelService(makeStandardRepo(ch, map[int64]string{g.ID: g.Platform}))
	cs.groupRepo = quoteGroupRepo{group: g}
	bs := NewBillingService(&config.Config{}, nil)
	a := Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeOAuth, Extra: map[string]any{openAILongContextBillingEnabledKey: true}}
	s := NewModelPriceQuoteService(&quoteCandidateRepo{accounts: []Account{a}}, cs, bs, NewModelPricingResolver(cs, bs))
	at := time.Date(2026, time.November, 12, 9, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return at }
	groups := []PlazaGroup{{ID: g.ID, Models: []PlazaModel{{Name: model, Platform: PlatformOpenAI}}}}
	s.FillDirectPrices(context.Background(), groups)
	q := groups[0].Models[0].PriceQuote
	require.Equal(t, "quoted", q.Status)
	v := q.Variants[0]
	require.NotNil(t, v.TimePricing)
	require.InDelta(t, 2e-6, *v.Pricing.InputPrice, 1e-15, "an exemption must not bake channel time multipliers into standard prices")
	require.Empty(t, v.Pricing.Intervals)
	actual, err := bs.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: model, Group: g, GroupID: &g.ID, Resolver: s.resolver, Tokens: UsageTokens{InputTokens: 300000}, RateMultiplier: 1, PricingAt: at, DisableModelLongContextPricing: true})
	require.NoError(t, err)
	require.InDelta(t, 300000**v.Pricing.InputPrice*2, actual.ActualCost, 1e-10)
}
