//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestNominalCurrencyChannelAndProcurementPersistence(t *testing.T) {
	ctx := context.Background()
	price := 1e-6
	channel := &service.Channel{
		ModelPricing:             []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"nominal-model"}, SourceCurrency: "CNY", InputPrice: &price}},
		AccountStatsPricingRules: []service.AccountStatsPricingRule{{Name: "nominal procurement", GroupIDs: []int64{}, AccountIDs: []int64{}, Pricing: []service.ChannelModelPricing{{Platform: service.PlatformOpenAI, Models: []string{"nominal-model"}, SourceCurrency: "EUR", InputPrice: &price}}}},
	}
	repo := newReasoningPricingIntegrationChannel(t, channel)
	loaded, err := repo.GetByID(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, "CNY", loaded.ModelPricing[0].SourceCurrency)
	require.Equal(t, "EUR", loaded.AccountStatsPricingRules[0].Pricing[0].SourceCurrency)
	loaded.ModelPricing[0].SourceCurrency = "JPY"
	require.NoError(t, repo.Update(ctx, loaded))
	loaded, err = repo.GetByID(ctx, channel.ID)
	require.NoError(t, err)
	require.Equal(t, "JPY", loaded.ModelPricing[0].SourceCurrency)
	require.Equal(t, "EUR", loaded.AccountStatsPricingRules[0].Pricing[0].SourceCurrency)
	require.Equal(t, price, *loaded.ModelPricing[0].InputPrice)
	require.Equal(t, price, *loaded.AccountStatsPricingRules[0].Pricing[0].InputPrice)
	copy := loaded.Clone()
	require.Equal(t, "JPY", copy.ModelPricing[0].SourceCurrency)
	require.Equal(t, "EUR", copy.AccountStatsPricingRules[0].Pricing[0].SourceCurrency)
}

func TestNominalCurrencyGroupJSONPersistence(t *testing.T) {
	ctx := context.Background()
	tx := testEntTx(t)
	repo := newGroupRepositoryWithSQL(tx.Client(), tx)
	price := 1e-6
	group := &service.Group{Name: t.Name(), Platform: service.PlatformOpenAI, RateMultiplier: 1, Status: service.StatusActive, SubscriptionType: service.SubscriptionTypeStandard,
		ModelPricing: []service.ChannelModelPricing{{Models: []string{"any-model"}, SourceCurrency: "CNY", InputPrice: &price}}}
	require.NoError(t, repo.Create(ctx, group))
	loaded, err := repo.GetByID(ctx, group.ID)
	require.NoError(t, err)
	require.Equal(t, "CNY", loaded.ModelPricing[0].SourceCurrency)
	require.Equal(t, price, *loaded.ModelPricing[0].InputPrice)
}
