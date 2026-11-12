//go:build unit

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDomesticPriceParity_CNYQuotesMatchRecordedBalanceAtTimeBoundaries(t *testing.T) {
	location, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	slots := []struct {
		name       string
		day        int
		clock      string
		multiplier float64
	}{
		{"before_morning", 12, "08:59:59", 1},
		{"morning_start", 12, "09:00:00", 2},
		{"morning_last_second", 12, "11:59:59", 2},
		{"morning_end", 12, "12:00:00", 1},
		{"before_afternoon", 12, "13:59:59", 1},
		{"afternoon_start", 12, "14:00:00", 2},
		{"afternoon_last_second", 12, "17:59:59", 2},
		{"afternoon_end", 12, "18:00:00", 1},
		{"saturday", 14, "09:00:00", 1},
		{"sunday", 15, "14:00:00", 1},
	}
	for _, route := range []struct {
		name        string
		model       string
		input       float64
		output      float64
		cache       float64
		groupRate   float64
		accountRate float64
		timed       bool
	}{
		{"flat_flash", "deepseek-v4.1-flash", 2, 8, .04, .075, .06, false},
		{"flat_pro", "deepseek-v4-pro", 9, 27, .30, .075, .06, false},
		{"timed_flash", "deepseek-v4.1-flash", 1, 4, .02, .35, .30, true},
		{"timed_pro", "deepseek-v4-pro", 4.5, 13.5, .15, .35, .30, true},
	} {
		for _, identity := range []string{BillingIdentityStandard, BillingIdentityPurchase} {
			for _, slot := range slots {
				t.Run(route.name+"/"+identity+"/"+slot.name, func(t *testing.T) {
					clock, parseErr := time.Parse("15:04:05", slot.clock)
					require.NoError(t, parseErr)
					at := time.Date(2026, time.November, slot.day, clock.Hour(), clock.Minute(), clock.Second(), 0, location)
					card := ChannelModelPricing{Platform: PlatformOpenAI, Models: []string{route.model}, SourceCurrency: "CNY", BillingMode: BillingModeToken,
						InputPrice: testPtrFloat64(route.input / 1e6), OutputPrice: testPtrFloat64(route.output / 1e6), CacheReadPrice: testPtrFloat64(route.cache / 1e6),
						CacheWritePrice: testPtrFloat64(0), CacheWrite1hPrice: testPtrFloat64(0),
					}
					if route.timed {
						card.TimePricing = &ChannelTimePricing{Timezone: "Asia/Shanghai", WeekdaysOnly: true, Periods: []ChannelTimePricingPeriod{
							{StartTime: "09:00", EndTime: "12:00", Multiplier: 2},
							{StartTime: "14:00", EndTime: "18:00", Multiplier: 2},
						}}
					}
					group := &Group{ID: 201, Platform: PlatformOpenAI, RateMultiplier: route.groupRate, LongContextPricingEnabled: true, Hydrated: true}
					channel := &Channel{ID: 1, Status: StatusActive, GroupIDs: []int64{group.ID}, BillingModelSource: BillingModelSourceUpstream, ApplyPricingToAccountStats: true, ModelPricing: []ChannelModelPricing{card}}
					account := Account{ID: 101, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, RateMultiplier: testPtrFloat64(route.accountRate), Credentials: map[string]any{"model_mapping": map[string]any{"public-model": route.model}}}
					logs := &openAIRecordUsageLogRepoStub{inserted: true}
					ledger := &openAIRecordUsageBillingRepoStub{}
					svc := newOpenAIRecordUsageServiceWithBillingRepoForTest(logs, ledger, &openAIRecordUsageUserRepoStub{}, &openAIRecordUsageSubRepoStub{}, nil)
					svc.channelService = newTestChannelService(makeStandardRepo(*channel, map[int64]string{group.ID: group.Platform}))
					svc.channelService.groupRepo = quoteGroupRepo{group: group}
					svc.resolver = NewModelPricingResolver(svc.channelService, svc.billingService)
					quotes := NewModelPriceQuoteService(&quoteCandidateRepo{accounts: []Account{account}}, svc.channelService, svc.billingService, svc.resolver)
					quotes.now = func() time.Time { return at }
					groups := []PlazaGroup{{ID: group.ID, Models: []PlazaModel{{Name: "public-model", Platform: PlatformOpenAI}}}}
					rate := route.groupRate
					if identity == BillingIdentityPurchase {
						quotes.FillPurchasePrices(context.Background(), groups)
						rate = route.accountRate
					} else {
						quotes.FillDirectPrices(context.Background(), groups)
					}
					quote := groups[0].Models[0].PriceQuote
					require.Equal(t, "quoted", quote.Status)
					require.Equal(t, at, quote.QuotedAt)
					require.Len(t, quote.Variants, 1)
					variant := quote.Variants[0]
					require.Equal(t, "CNY", variant.Pricing.SourceCurrency)
					timeFactor := 1.0
					if route.timed {
						require.NotNil(t, variant.TimePricing, "the price sheet must retain configured peak windows")
						require.Equal(t, "Asia/Shanghai", variant.TimePricing.Timezone)
						require.True(t, variant.TimePricing.WeekdaysOnly)
						require.Len(t, variant.TimePricing.Periods, 2)
						timeFactor = slot.multiplier
					} else {
						require.Nil(t, variant.TimePricing, "a channel card must not inherit native DeepSeek peak policy")
					}
					// Purchase variants already contain the selected account rate.
					// Customer variants receive the user/group rate in the frontend.
					quoteRate := route.groupRate
					if identity == BillingIdentityPurchase {
						quoteRate = 1
					}
					quoted := (1e6**variant.Pricing.InputPrice + 2e5**variant.Pricing.OutputPrice + 1e5**variant.Pricing.CacheReadPrice) * timeFactor * quoteRate
					wantBase := (route.input + .2*route.output + .1*route.cache) * timeFactor
					want := wantBase * rate
					require.InDelta(t, want, quoted, 1e-10, "CNY numbers are nominal balance units without FX")
					require.NoError(t, svc.RecordUsage(context.Background(), &OpenAIRecordUsageInput{
						Result: &OpenAIForwardResult{RequestID: "domestic-parity", Model: "public-model", UpstreamModel: route.model, Usage: OpenAIUsage{InputTokens: 1100000, OutputTokens: 200000, CacheReadInputTokens: 100000}},
						APIKey: &APIKey{ID: 301, GroupID: &group.ID, Group: group, Quota: 100}, User: &User{ID: 401, BillingIdentity: identity}, Account: &account,
						APIKeyService: &openAIRecordUsageAPIKeyQuotaStub{}, PricingAt: at,
						ChannelUsageFields: ChannelUsageFields{OriginalModel: "public-model", BillingModelSource: BillingModelSourceUpstream},
					}))
					require.NotNil(t, logs.lastLog)
					require.NotNil(t, logs.lastLog.AccountStatsCost)
					require.InDelta(t, wantBase, logs.lastLog.TotalCost, 1e-10)
					require.InDelta(t, wantBase, *logs.lastLog.AccountStatsCost, 1e-10)
					require.InDelta(t, want, logs.lastLog.ActualCost, 1e-10)
					require.NotNil(t, ledger.lastCmd)
					require.InDelta(t, quoted, ledger.lastCmd.BalanceCost, 1e-10)
					require.InDelta(t, quoted, ledger.lastCmd.APIKeyQuotaCost, 1e-10)
				})
			}
		}
	}
}
