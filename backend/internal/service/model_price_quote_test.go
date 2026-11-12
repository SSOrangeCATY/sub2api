//go:build unit

package service

import (
	"context"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func quoteFixture(t *testing.T, card ChannelModelPricing) (*ModelPriceQuoteService, *Group, *Channel) {
	t.Helper()
	g := &Group{ID: 10, Platform: PlatformAnthropic, RateMultiplier: 99}
	card.Models = []string{"quote-unit"}
	ch := &Channel{ID: 8, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{1, 2}, Pricing: []ChannelModelPricing{card}}}}
	cs := newTestChannelServiceForStats(t, ch, g.ID, g.Platform)
	bs := NewBillingService(&config.Config{}, nil)
	s := NewModelPriceQuoteService(nil, cs, bs, NewModelPricingResolver(cs, bs))
	return s, g, ch
}

func TestPurchaseQuote_FollowsActualAccountCostAndDoesNotMutateCard(t *testing.T) {
	s, g, ch := quoteFixture(t, ChannelModelPricing{ID: 72, ChannelID: 8, SourceCurrency: "CNY", BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(8e-6), CacheReadPrice: testPtrFloat64(.04e-6)})
	a := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.25)}
	q := s.quoteModel(context.Background(), g, ch, &PlazaModel{Name: "quote-unit", Platform: PlatformAnthropic}, []Account{a})
	require.Equal(t, "quoted", q.Status)
	require.Len(t, q.Variants, 1)
	p := q.Variants[0].Pricing
	require.Equal(t, "CNY", p.SourceCurrency)
	require.InDelta(t, .5e-6, *p.InputPrice, 1e-15)
	require.InDelta(t, 2e-6, *p.OutputPrice, 1e-15)
	tokens := UsageTokens{InputTokens: 40, OutputTokens: 7, CacheReadTokens: 13}
	actual := resolveAccountStatsCost(context.Background(), s.channels, s.billing, a.ID, g.ID, "quote-unit", tokens, 1, 999, "", time.Now(), true)
	require.NotNil(t, actual)
	require.InDelta(t, *actual*a.BillingRateMultiplier(), float64(40)**p.InputPrice+float64(7)**p.OutputPrice+float64(13)**p.CacheReadPrice, 1e-15)
	require.Equal(t, int64(0), p.ID)
	require.Nil(t, p.Models)
	require.InDelta(t, 2e-6, *ch.AccountStatsPricingRules[0].Pricing[0].InputPrice, 1e-15)
}

func TestPurchaseQuote_CustomIntervalsAreAbsoluteAndRequestIntervalsIgnored(t *testing.T) {
	t.Run("token", func(t *testing.T) {
		s, g, ch := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(9e-6), OutputPrice: testPtrFloat64(10e-6), Intervals: []PricingInterval{{MinTokens: 0, InputPrice: testPtrFloat64(2e-6), OutputMultiplier: testPtrFloat64(4)}}})
		a := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
		v := s.accountVariant(context.Background(), g, ch, &a, "quote-unit", "quote-unit")
		require.NotNil(t, v)
		require.Equal(t, "absolute", v.IntervalPriceMode)
		require.Equal(t, "total", v.ContextMetric)
		require.Nil(t, v.Pricing.Intervals[0].OutputPrice)
		require.Nil(t, v.Pricing.Intervals[0].OutputMultiplier)
		cost := resolveAccountStatsCost(context.Background(), s.channels, s.billing, 1, 10, "quote-unit", UsageTokens{InputTokens: 10, OutputTokens: 20}, 1, 99, "", time.Now(), true)
		require.NotNil(t, cost)
		require.InDelta(t, 10**v.Pricing.Intervals[0].InputPrice, *cost, 1e-15)
	})
	t.Run("per_request", func(t *testing.T) {
		s, g, ch := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModePerRequest, PerRequestPrice: testPtrFloat64(.2), Intervals: []PricingInterval{{PerRequestPrice: testPtrFloat64(999)}}})
		a := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.5)}
		v := s.accountVariant(context.Background(), g, ch, &a, "quote-unit", "quote-unit")
		require.NotNil(t, v)
		require.Empty(t, v.Pricing.Intervals)
		require.Equal(t, .1, *v.Pricing.PerRequestPrice)
	})
	t.Run("video_request_unit", func(t *testing.T) {
		s, g, ch := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModeVideo, PerRequestPrice: testPtrFloat64(.2)})
		a := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
		v := s.accountVariant(context.Background(), g, ch, &a, "quote-unit", "quote-unit")
		require.Equal(t, BillingModePerRequest, v.Pricing.BillingMode)
		require.Equal(t, .2, *v.Pricing.PerRequestPrice)
	})
	t.Run("finite_interval_keeps_fallback", func(t *testing.T) {
		max := 100
		s, g, ch := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(9e-6), Intervals: []PricingInterval{{MinTokens: 0, MaxTokens: &max, InputPrice: testPtrFloat64(2e-6)}}})
		a := Account{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}
		v := s.accountVariant(context.Background(), g, ch, &a, "quote-unit", "quote-unit")
		require.InDelta(t, 9e-6, *v.Pricing.InputPrice, 1e-15)
		actual := resolveAccountStatsCost(context.Background(), s.channels, s.billing, 1, 10, "quote-unit", UsageTokens{InputTokens: 200}, 1, 99, "", time.Now(), true)
		require.NotNil(t, actual)
		require.InDelta(t, 200**v.Pricing.InputPrice, *actual, 1e-15)
	})
}

func TestPurchaseQuote_ExactPlatformAndAllConfiguredRouteRates(t *testing.T) {
	s, g, ch := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6), OutputPrice: testPtrFloat64(4e-6)})
	model := &PlazaModel{Name: "quote-unit", Platform: PlatformAnthropic}
	accounts := []Account{
		{ID: 1, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(100)},
		{ID: 1, Platform: PlatformGemini, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(100)},
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.25)},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.5)},
	}
	q := s.quoteModel(context.Background(), g, ch, model, accounts)
	require.Equal(t, "quoted", q.Status)
	require.Len(t, q.Variants, 2)
	require.InDelta(t, .5e-6, *q.Variants[0].Pricing.InputPrice, 1e-15)
	require.InDelta(t, 1e-6, *q.Variants[1].Pricing.InputPrice, 1e-15)
	q = s.quoteModel(context.Background(), g, ch, model, accounts[:2])
	require.Equal(t, "unavailable", q.Status)
	require.Empty(t, q.Variants)
}

type quoteCandidateRepo struct {
	AccountRepository
	platforms []string
	accounts  []Account
}

func (r *quoteCandidateRepo) ListModelAvailabilityCandidates(_ context.Context, _ *int64, platforms []string, _ bool) ([]Account, error) {
	r.platforms = platforms
	// Mirror the real repository's empty-platform guard.
	if len(platforms) == 0 {
		return []Account{}, nil
	}
	return r.accounts, nil
}

type quoteGroupRepo struct {
	GroupRepository
	group *Group
}

func (r quoteGroupRepo) GetByID(context.Context, int64) (*Group, error) { return r.group, nil }

func TestPurchaseQuote_EnumeratesNonemptyConcretePlatformsBeforePricing(t *testing.T) {
	s, g, _ := quoteFixture(t, ChannelModelPricing{BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6)})
	r := &quoteCandidateRepo{accounts: []Account{{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey}}}
	s.accounts = r
	s.channels.groupRepo = quoteGroupRepo{group: g}
	groups := []PlazaGroup{{ID: g.ID, Models: []PlazaModel{{Name: "quote-unit", Platform: PlatformAnthropic}}}}
	s.FillPurchasePrices(context.Background(), groups)
	require.ElementsMatch(t, []string{PlatformAnthropic, PlatformAntigravity}, r.platforms)
	require.Equal(t, "quoted", groups[0].Models[0].PriceQuote.Status)
	require.False(t, groups[0].Models[0].PriceQuote.QuotedAt.IsZero())
}

func TestPurchaseQuote_NativeSnapshotIncludesActualDeepseekPeakPolicy(t *testing.T) {
	for _, hour := range []int{0, 2} {
		t.Run(time.Date(2026, 11, 12, hour, 0, 0, 0, time.UTC).Format("15"), func(t *testing.T) {
			catalog := newStubPricingServiceFromJSON(t, `{"deepseek-flash":{"input_cost_per_token":0.000001,"output_cost_per_token":0.000004,"cache_read_input_token_cost":0.00000002}}`)
			bs := NewBillingService(&config.Config{}, catalog)
			ch := &Channel{ID: 8, Status: StatusActive}
			cs := newTestChannelServiceForStats(t, ch, 10, PlatformDeepseek)
			s := NewModelPriceQuoteService(nil, cs, bs, NewModelPricingResolver(cs, bs))
			at := time.Date(2026, 11, 12, hour, 0, 0, 0, time.UTC)
			s.now = func() time.Time { return at }
			a := &Account{ID: 1, Platform: PlatformDeepseek, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.5)}
			v := s.accountVariant(context.Background(), &Group{ID: 10, Platform: PlatformDeepseek}, ch, a, "deepseek-flash", "deepseek-flash")
			require.NotNil(t, v)
			actual := resolveAccountStatsCost(context.Background(), cs, bs, 1, 10, "deepseek-flash", UsageTokens{InputTokens: 100, OutputTokens: 20}, 1, 999, "", at, true)
			require.NotNil(t, actual)
			require.InDelta(t, *actual*.5, 100**v.Pricing.InputPrice+20**v.Pricing.OutputPrice, 1e-12)
		})
	}
}

func TestPurchaseQuote_UpstreamRestrictionsExcludeUnselectableAccounts(t *testing.T) {
	ch := Channel{ID: 8, Status: StatusActive, GroupIDs: []int64{10}, BillingModelSource: BillingModelSourceUpstream, RestrictModels: true,
		ModelPricing:             []ChannelModelPricing{{Platform: PlatformAnthropic, Models: []string{"allowed-upstream"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6)}},
		AccountStatsPricingRules: []AccountStatsPricingRule{{GroupIDs: []int64{10}, Pricing: []ChannelModelPricing{{Models: []string{"*"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6)}}}},
	}
	cs := newTestChannelService(makeStandardRepo(ch, map[int64]string{10: PlatformAnthropic}))
	bs := NewBillingService(&config.Config{}, nil)
	s := NewModelPriceQuoteService(nil, cs, bs, NewModelPricingResolver(cs, bs))
	accounts := []Account{
		{ID: 1, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(.5), Credentials: map[string]any{"model_mapping": map[string]any{"public-alias": "allowed-upstream"}}},
		{ID: 2, Platform: PlatformAnthropic, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(99), Credentials: map[string]any{"model_mapping": map[string]any{"public-alias": "blocked-upstream"}}},
	}
	q := s.quoteModel(context.Background(), &Group{ID: 10, Platform: PlatformAnthropic}, &ch, &PlazaModel{Name: "public-alias", Platform: PlatformAnthropic}, accounts)
	require.Equal(t, "quoted", q.Status)
	require.Len(t, q.Variants, 1)
	require.InDelta(t, 1e-6, *q.Variants[0].Pricing.InputPrice, 1e-15)
}

func TestPurchaseQuote_PublicDedupIgnoresInternalCardAndIntervalMetadata(t *testing.T) {
	s, g, ch := quoteFixture(t, ChannelModelPricing{})
	ch.AccountStatsPricingRules = nil
	accounts := []Account{}
	for i := 1; i <= 33; i++ {
		id := int64(i)
		accounts = append(accounts, Account{ID: id, Platform: PlatformAnthropic, Type: AccountTypeAPIKey})
		card := ChannelModelPricing{ID: id, ChannelID: id, Models: []string{"quote-unit"}, BillingMode: BillingModeToken, InputPrice: testPtrFloat64(2e-6), CreatedAt: time.Unix(id, 0),
			Intervals: []PricingInterval{{ID: id, PricingID: id, SortOrder: i, InputPrice: testPtrFloat64(2e-6), CreatedAt: time.Unix(id, 0)}},
		}
		ch.AccountStatsPricingRules = append(ch.AccountStatsPricingRules, AccountStatsPricingRule{AccountIDs: []int64{id}, Pricing: []ChannelModelPricing{card}})
	}
	q := s.quoteModel(context.Background(), g, ch, &PlazaModel{Name: "quote-unit", Platform: PlatformAnthropic}, accounts)
	require.Equal(t, "quoted", q.Status)
	require.Len(t, q.Variants, 1)
}

func TestPurchaseQuote_SubscriptionLongContextMatchesProcurementExemption(t *testing.T) {
	catalog := newStubPricingServiceFromJSON(t, `{"gpt-6.1-sol":{"input_cost_per_token":0.000002,"output_cost_per_token":0.00001,"cache_read_input_token_cost":0.0000001,"input_cost_per_token_above_272k_tokens":0.000004,"output_cost_per_token_above_272k_tokens":0.000015}}`)
	bs := NewBillingService(&config.Config{}, catalog)
	ch := &Channel{ID: 8, Status: StatusActive}
	cs := newTestChannelServiceForStats(t, ch, 10, PlatformOpenAI)
	s := NewModelPriceQuoteService(nil, cs, bs, NewModelPricingResolver(cs, bs))
	for _, accountType := range []string{AccountTypeOAuth, AccountTypeSetupToken, AccountTypeAPIKey} {
		t.Run(accountType, func(t *testing.T) {
			a := &Account{ID: 1, Platform: PlatformOpenAI, Type: accountType, Extra: map[string]any{openAILongContextBillingEnabledKey: true}}
			v := s.accountVariant(context.Background(), &Group{ID: 10, Platform: PlatformOpenAI}, ch, a, "gpt-6.1-sol", "gpt-6.1-sol")
			require.NotNil(t, v)
			long := accountStatsLongContextPricingEnabled(openAILongContextBillingGate(a)) && !openAISubscriptionLongContextExempt(a, &OpenAIForwardResult{UpstreamModel: "gpt-6.1-sol"})
			actual := resolveAccountStatsCost(context.Background(), cs, bs, 1, 10, "gpt-6.1-sol", UsageTokens{InputTokens: 300000, OutputTokens: 1000}, 1, 999, "", time.Now(), long)
			p := v.Pricing
			if iv := FindMatchingInterval(p.Intervals, 300000); iv != nil {
				p = &ChannelModelPricing{InputPrice: iv.InputPrice, OutputPrice: iv.OutputPrice}
			}
			require.NotNil(t, actual)
			require.InDelta(t, *actual, 300000**p.InputPrice+1000**p.OutputPrice, 1e-9)
			if accountType != AccountTypeAPIKey {
				require.Empty(t, v.Pricing.Intervals)
			}
		})
	}
}
