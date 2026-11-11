//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNominalPricePresetUsesNativeNumbersWithoutFX(t *testing.T) {
	quote, ok := LookupNominalPricePreset("DeepSeek-V4.1-Flash", "CNY")
	require.True(t, ok)
	require.Equal(t, "USD", quote.SettlementCurrency)
	require.Equal(t, "nominal", quote.UnitPolicy)
	require.InDelta(t, 1.0, quote.InputPrice*1e6, 1e-12)
	require.InDelta(t, 4.0, quote.OutputPrice*1e6, 1e-12)
	require.InDelta(t, 0.02, quote.CacheReadPrice*1e6, 1e-12)
	for _, candidate := range []struct{ model, currency string }{{"unknown-model", "CNY"}, {"deepseek-v4-pro-unknown", "CNY"}, {"deepseek-flash", "EUR"}} {
		_, found := LookupNominalPricePreset(candidate.model, candidate.currency)
		require.False(t, found, "no FX or alias guesses")
	}
}

func TestNominalCurrencyUsesExistingPriceAndSettlementPaths(t *testing.T) {
	for _, currency := range []string{"", "USD", "CNY", "EUR", "JPY"} {
		t.Run(currency, func(t *testing.T) {
			card := ChannelModelPricing{Models: []string{"generic-model"}, SourceCurrency: currency, InputPrice: float64Ptr(1e-6), OutputPrice: float64Ptr(4e-6), CacheReadPrice: float64Ptr(0.02e-6)}
			require.NoError(t, validatePricingEntries([]ChannelModelPricing{card}))
			encoded, err := json.Marshal(card)
			require.NoError(t, err)
			var restored ChannelModelPricing
			require.NoError(t, json.Unmarshal(encoded, &restored))
			require.Equal(t, currency, restored.SourceCurrency)
			group := &Group{ID: 10, ModelPricing: []ChannelModelPricing{restored}}
			billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{})
			resolver := NewModelPricingResolver(nil, billing)
			cost, err := billing.CalculateCostUnified(CostInput{Ctx: context.Background(), Model: "generic-model", Group: group, Tokens: UsageTokens{InputTokens: 1_000_000, OutputTokens: 1_000_000, CacheReadTokens: 1_000_000}, RateMultiplier: 0.4, Resolver: resolver})
			require.NoError(t, err)
			require.InDelta(t, 5.02, cost.TotalCost, 1e-10)
			require.InDelta(t, 2.008, cost.ActualCost, 1e-10)
			stats := calculateStatsCost(&restored, UsageTokens{InputTokens: 1_000_000, OutputTokens: 1_000_000, CacheReadTokens: 1_000_000}, 1)
			require.NotNil(t, stats)
			log := &UsageLog{AccountStatsCost: stats}
			require.NoError(t, applyPurchasePricing(&User{BillingIdentity: BillingIdentityPurchase}, cost, log, 0.35))
			require.InDelta(t, 1.757, cost.ActualCost, 1e-10)
		})
	}
	for _, currency := range []string{"cny", "CN", "USDT", "C1Y"} {
		require.Error(t, validatePricingEntries([]ChannelModelPricing{{SourceCurrency: currency}}))
	}
}
