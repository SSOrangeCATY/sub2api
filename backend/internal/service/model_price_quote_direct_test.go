//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestDirectGroupQuote_AccountTierAndExemptionMatchActualUnifiedCost(t *testing.T) {
	catalog := newStubPricingServiceFromJSON(t, `{"gpt-6.1-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000001,"input_cost_per_token_above_272k_tokens":0.000004,"output_cost_per_token_above_272k_tokens":0.000015}}`)
	bs := NewBillingService(&config.Config{}, catalog)
	cs := newTestChannelServiceForStats(t, &Channel{Status: "inactive"}, 10, PlatformOpenAI)
	resolver := NewModelPricingResolver(cs, bs)
	s := NewModelPriceQuoteService(nil, cs, bs, resolver)
	at := time.Date(2026, 11, 12, 10, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return at }
	for _, groupLong := range []bool{false, true} {
		for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth, AccountTypeSetupToken} {
			for _, accountLong := range []bool{false, true} {
				t.Run(accountType+"/group="+boolLabel(groupLong)+"/account="+boolLabel(accountLong), func(t *testing.T) {
					g := &Group{ID: 10, Platform: PlatformOpenAI, LongContextPricingEnabled: groupLong}
					a := &Account{ID: 1, Platform: PlatformOpenAI, Type: accountType, RateMultiplier: testPtrFloat64(.5), Extra: map[string]any{openAILongContextBillingEnabledKey: accountLong}}
					for _, candidateChannel := range []*Channel{nil, {Status: StatusActive, ApplyPricingToAccountStats: true}} {
						v := s.accountVariant(context.Background(), g, candidateChannel, a, "gpt-6.1-sol", "gpt-6.1-sol")
						require.NotNil(t, v)
						for _, input := range []int{100, 300000} {
							gate := openAILongContextBillingGate(a)
							exempt := openAISubscriptionLongContextExempt(a, &OpenAIForwardResult{UpstreamModel: "gpt-6.1-sol"})
							actual, err := bs.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: "gpt-6.1-sol", Group: g, GroupID: &g.ID, Resolver: resolver, Tokens: UsageTokens{InputTokens: input, OutputTokens: 100}, RateMultiplier: 1, PricingAt: at, LongContextBillingEnabled: gate, DisableModelLongContextPricing: exempt})
							require.NoError(t, err)
							p := v.Pricing
							if iv := FindMatchingInterval(p.Intervals, input); iv != nil {
								p = &ChannelModelPricing{InputPrice: iv.InputPrice, OutputPrice: iv.OutputPrice}
							}
							require.InDelta(t, actual.ActualCost*.5, float64(input)**p.InputPrice+100**p.OutputPrice, 1e-8)
						}
					}
				})
			}
		}
	}
}

func boolLabel(value bool) string {
	if value {
		return "on"
	}
	return "off"
}

func TestDirectGroupQuote_CustomerVariantsPreserveAccountSnapshotAndSalesRate(t *testing.T) {
	catalog := newStubPricingServiceFromJSON(t, `{"gpt-6.1-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"input_cost_per_token_above_272k_tokens":0.000004,"output_cost_per_token_above_272k_tokens":0.000015}}`)
	bs := NewBillingService(&config.Config{}, catalog)
	cs := newTestChannelServiceForStats(t, &Channel{Status: "inactive"}, 10, PlatformOpenAI)
	g := &Group{ID: 10, Platform: PlatformOpenAI, RateMultiplier: .4}
	cs.groupRepo = quoteGroupRepo{group: g}
	r := &quoteCandidateRepo{accounts: []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(2), Extra: map[string]any{openAILongContextBillingEnabledKey: true}},
		{ID: 2, Platform: PlatformOpenAI, Type: AccountTypeOAuth, RateMultiplier: testPtrFloat64(.5), Extra: map[string]any{openAILongContextBillingEnabledKey: true}},
	}}
	s := NewModelPriceQuoteService(r, cs, bs, NewModelPricingResolver(cs, bs))
	groups := []PlazaGroup{{ID: 10, Models: []PlazaModel{{Name: "gpt-6.1-sol", Platform: PlatformOpenAI}}}}
	s.FillDirectPrices(context.Background(), groups)
	q := groups[0].Models[0].PriceQuote
	require.Equal(t, "customer", q.Kind)
	require.Equal(t, "quoted", q.Status)
	require.Len(t, q.Variants, 2, "different account tier policies remain separate safe variants")
	require.Equal(t, 2.0, *r.accounts[0].RateMultiplier, "display queries must not change repository snapshots")
	require.Equal(t, .5, *r.accounts[1].RateMultiplier)
	require.Equal(t, .4, g.RateMultiplier)
}
