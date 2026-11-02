//go:build unit

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPurchasePricingUsesActualSelectedAccountCost(t *testing.T) {
	for _, tt := range []struct {
		name       string
		identity   string
		base       float64
		stats      *float64
		rate, want float64
	}{
		{"standard unchanged", BillingIdentityStandard, 10, float64Ptr(7), 0.3, 9},
		{"purchase account fallback", BillingIdentityPurchase, 10, nil, 0.06, 0.6},
		{"purchase next fallback account", BillingIdentityPurchase, 10, nil, 0.3, 3},
		{"upstream pricing overrides sales card", BillingIdentityPurchase, 10, float64Ptr(6), 0.35, 2.1},
		{"zero sale still has procurement cost", BillingIdentityPurchase, 0, float64Ptr(4), 0.35, 1.4},
		{"zero account is free", BillingIdentityPurchase, 10, float64Ptr(6), 0, 0},
		{"zero procurement rule is free", BillingIdentityPurchase, 10, float64Ptr(0), 0.35, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cost := &CostBreakdown{TotalCost: tt.base, ActualCost: 9}
			log := &UsageLog{AccountStatsCost: tt.stats, ActualCost: 9}
			require.NoError(t, applyPurchasePricing(&User{BillingIdentity: tt.identity}, cost, log, tt.rate))
			require.InDelta(t, tt.want, cost.ActualCost, 1e-12)
			require.InDelta(t, tt.want, log.ActualCost, 1e-12)
			require.Equal(t, tt.base, cost.TotalCost, "sales base remains auditable")
		})
	}
}

func TestPurchasePricingRejectsInvalidCostInsteadOfChargingSalesFallback(t *testing.T) {
	for _, invalid := range []float64{-1, math.NaN(), math.Inf(1)} {
		for _, invalidRate := range []bool{false, true} {
			cost := &CostBreakdown{TotalCost: 10, ActualCost: 9}
			base, rate := invalid, 0.3
			if invalidRate {
				base, rate = 6, invalid
			}
			require.Error(t, applyPurchasePricing(&User{BillingIdentity: BillingIdentityPurchase}, cost, &UsageLog{AccountStatsCost: &base}, rate))
			require.Equal(t, 9.0, cost.ActualCost)
		}
	}
}

func TestPurchaseSubscriptionWithZeroSalesBaseUsesSubscription(t *testing.T) {
	cost := &CostBreakdown{}
	log := &UsageLog{AccountStatsCost: float64Ptr(4)}
	user := &User{ID: 1, BillingIdentity: BillingIdentityPurchase}
	require.NoError(t, applyPurchasePricing(user, cost, log, 0.35))
	p := &postUsageBillingParams{Cost: cost, User: user, APIKey: &APIKey{ID: 2}, Account: &Account{ID: 3}, Subscription: &UserSubscription{ID: 4}, IsSubscriptionBill: true}
	cmd := buildUsageBillingCommand("purchase-subscription", log, p)
	require.NotNil(t, cmd)
	require.InDelta(t, 1.4, cmd.SubscriptionCost, 1e-12)
	require.Zero(t, cmd.BalanceCost)
	require.Equal(t, int64(4), *cmd.SubscriptionID)
}

func TestPurchaseInflightReservationIncludesMediaAccountRulesAndUnits(t *testing.T) {
	for _, kind := range []InflightEstimateKind{InflightEstimateToken, InflightEstimateImage, InflightEstimateVideo, InflightEstimatePerRequest} {
		t.Run(fmt.Sprint(kind), func(t *testing.T) {
			billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{"purchase-model": {InputPricePerToken: 0.001, OutputPricePerToken: 0.002}})
			groupID := int64(10)
			key := &APIKey{User: &User{BillingIdentity: BillingIdentityPurchase}, GroupID: &groupID, Group: &Group{ID: groupID, RateMultiplier: 0.075}}
			calls := 0
			d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(nil, billing),
				purchaseAccounts: func(context.Context, *APIKey, string) ([]Account, error) {
					return []Account{{ID: 1, RateMultiplier: float64Ptr(0.06)}, {ID: 2, RateMultiplier: float64Ptr(0.35)}}, nil
				},
				accountStatsEstimate: func(_ context.Context, accountID, groupID int64, model string, tokens UsageTokens, count int, total float64, at time.Time, long bool) *float64 {
					calls++
					require.Equal(t, 3, count)
					return float64Ptr(6 * float64(count))
				},
			}
			got, ok := d.estimate(context.Background(), key, InflightEstimateRequest{Model: "purchase-model", Kind: kind, Units: 3, BodyBytes: 40, MaxTokens: 10})
			require.True(t, ok)
			require.Equal(t, 2, calls)
			require.InDelta(t, 6.3, got, 1e-12, "reserve highest selectable account cost")
		})
	}
}

func TestPurchaseAuthSnapshotRoundtrip(t *testing.T) {
	s := &APIKeyService{}
	key := &APIKey{ID: 1, UserID: 2, Key: "identity-test", Status: StatusActive, User: &User{ID: 2, BillingIdentity: BillingIdentityPurchase, Role: RoleUser, AllowedGroups: []int64{7}, RestrictPublicGroups: true}}
	payload, err := json.Marshal(&APIKeyAuthCacheEntry{Snapshot: s.snapshotFromAPIKey(context.Background(), key)})
	require.NoError(t, err)
	var entry APIKeyAuthCacheEntry
	require.NoError(t, json.Unmarshal(payload, &entry))
	got, used, err := s.applyAuthCacheEntry(key.Key, &entry)
	require.NoError(t, err)
	require.True(t, used)
	require.True(t, got.User.UsesPurchasePricing())
	require.Equal(t, RoleUser, got.User.Role)
	require.Equal(t, []int64{7}, got.User.AllowedGroups)
	require.True(t, got.User.RestrictPublicGroups)
}

func TestPurchaseInflightReservationDistinguishesFreeFromUnknownPricing(t *testing.T) {
	for _, tt := range []struct {
		name  string
		stats *float64
		ok    bool
	}{
		{"explicit free", float64Ptr(0), true},
		{"unknown", nil, false},
		{"invalid", float64Ptr(math.NaN()), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(10)
			billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{})
			d := inflightEstimateDeps{billing: billing, resolver: NewModelPricingResolver(nil, billing),
				purchaseAccounts: func(context.Context, *APIKey, string) ([]Account, error) { return []Account{{ID: 1}}, nil },
				accountStatsEstimate: func(context.Context, int64, int64, string, UsageTokens, int, float64, time.Time, bool) *float64 {
					return tt.stats
				},
			}
			got, ok := d.estimate(context.Background(), &APIKey{User: &User{BillingIdentity: BillingIdentityPurchase}, GroupID: &groupID}, InflightEstimateRequest{Model: "unknown-procurement", Kind: InflightEstimatePerRequest})
			require.Equal(t, tt.ok, ok)
			require.Zero(t, got)
		})
	}
}

func TestBatchImagePurchaseSettlementUsesSubmittedAccountSnapshot(t *testing.T) {
	ctx := context.Background()
	svc, repo, _, _, _ := newTestBatchImagePublicService(true)
	groupID := int64(10)
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{groupID: {ID: groupID, Platform: PlatformGemini, AllowBatchImageGeneration: true, RateMultiplier: 9, ImagePrice1K: float64Ptr(99), BatchImageDiscountMultiplier: 0.9, BatchImageHoldMultiplier: 0.95}}}
	svc.ChannelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{101}, Pricing: []ChannelModelPricing{{Models: []string{"gemini-2.5-flash-image"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(0.8)}}}}}, groupID, PlatformGemini)
	accounts := svc.AccountRepo.(*publicBatchImageAccountRepo)
	accounts.accounts[0].RateMultiplier = float64Ptr(0.3)
	owner := testBatchImageOwner()
	owner.GroupID, owner.BillingIdentity = &groupID, BillingIdentityPurchase
	batch, err := svc.Submit(ctx, owner, validBatchImageSubmitRequest(), "")
	require.NoError(t, err)
	job := repo.jobs[batch.ID]
	require.InDelta(t, 0.12, job.BillableUnitPrice, 1e-12)
	require.InDelta(t, 0.288, *job.HoldAmount, 1e-12)
	job.Status, job.SuccessCount, job.FailCount = BatchImageJobStatusSettling, 1, 1
	// Later edits and a changed catalog must not reprice this submitted job.
	accounts.accounts[0].RateMultiplier = float64Ptr(8)
	settlement := &BatchImageSettlementService{Repo: repo, BillingRepo: svc.BillingRepo, Pricing: &fakeBatchImagePricingResolver{unitPrice: 99}}
	result, err := settlement.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.InDelta(t, 0.12, result.ActualCost, 1e-12)
	billing := svc.BillingRepo.(*fakeBatchImageBillingRepo)
	require.Len(t, billing.captures, 1)
	require.InDelta(t, 0.12, billing.captures[0].ActualAmount, 1e-12)
	again, err := settlement.Settle(ctx, job.BatchID)
	require.NoError(t, err)
	require.True(t, again.AlreadySettled)
	require.Len(t, billing.captures, 1)
}

func TestAdminBillingIdentityChangeInvalidatesAuthWithoutChangingAccess(t *testing.T) {
	for _, identity := range []string{BillingIdentityPurchase, BillingIdentityStandard} {
		t.Run(identity, func(t *testing.T) {
			old := BillingIdentityStandard
			if identity == BillingIdentityStandard {
				old = BillingIdentityPurchase
			}
			repo := &mockUserRepo{getByIDUser: &User{ID: 7, Role: RoleUser, BillingIdentity: old, AllowedGroups: []int64{10}, RestrictPublicGroups: true, Balance: 12}}
			invalidator := &authCacheInvalidatorStub{}
			svc := &adminServiceImpl{userRepo: repo, authCacheInvalidator: invalidator}
			got, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{BillingIdentity: &identity})
			require.NoError(t, err)
			require.Equal(t, identity, got.BillingIdentity)
			require.Equal(t, []UserUpdateFields{{BillingIdentity: true}}, repo.updateFields)
			require.Equal(t, []int64{7}, invalidator.userIDs)
			require.Equal(t, RoleUser, got.Role)
			require.Equal(t, []int64{10}, got.AllowedGroups)
			require.True(t, got.RestrictPublicGroups)
			require.Equal(t, 12.0, got.Balance)
		})
	}
}

func TestAdminRetiredPersonalRatesCannotBeRecreated(t *testing.T) {
	svc := &adminServiceImpl{}
	_, err := svc.UpdateUser(context.Background(), 1, &UpdateUserInput{GroupRates: map[int64]*float64{10: float64Ptr(0.1)}})
	require.ErrorContains(t, err, "billing identity")
	invalid := "admin"
	_, err = svc.UpdateUser(context.Background(), 1, &UpdateUserInput{BillingIdentity: &invalid})
	require.Error(t, err)
}

type failingIdentityLegacyRateRepo struct {
	UserGroupRateRepository
}

func (*failingIdentityLegacyRateRepo) SyncUserGroupRates(context.Context, int64, map[int64]*float64) error {
	return fmt.Errorf("legacy cleanup failed")
}

func TestAdminBillingIdentityInvalidatesPersistedChangeWhenLegacyCleanupFails(t *testing.T) {
	repo := &mockUserRepo{getByIDUser: &User{ID: 7, BillingIdentity: BillingIdentityStandard}}
	invalidator := &authCacheInvalidatorStub{}
	svc := &adminServiceImpl{userRepo: repo, authCacheInvalidator: invalidator, userGroupRateRepo: &failingIdentityLegacyRateRepo{}}
	identity := BillingIdentityPurchase
	_, err := svc.UpdateUser(context.Background(), 7, &UpdateUserInput{BillingIdentity: &identity, GroupRates: map[int64]*float64{10: nil}})
	require.ErrorContains(t, err, "legacy cleanup failed")
	require.Equal(t, []UserUpdateFields{{BillingIdentity: true}}, repo.updateFields)
	require.Equal(t, []int64{7}, invalidator.userIDs, "persisted identity must take effect even if optional cleanup fails")
}

func TestPurchaseInflightCompositeUsesResolvedProvider(t *testing.T) {
	for _, platform := range []string{PlatformDeepseek, PlatformGrok} {
		t.Run(platform, func(t *testing.T) {
			groupID := int64(90)
			base := newInflightEstimateGateway(t, nil)
			model := "deepseek-v4-flash"
			if platform == PlatformGrok {
				model = "grok-4.7"
			}
			billing := newTestBillingServiceWithPrices(map[string]*ModelPricing{model: {InputPricePerToken: 0.001, OutputPricePerToken: 0.002}})
			snap := &inflightSnapshotCacheStub{byBucket: map[string][]Account{
				inflightBucketKey(groupID, platform):       {{ID: 11, Platform: platform, RateMultiplier: float64Ptr(0.35), Credentials: map[string]any{"model_mapping": map[string]any{"purchase-alias": model}}}},
				inflightBucketKey(groupID, PlatformOpenAI): {{ID: 99, Platform: PlatformOpenAI, RateMultiplier: float64Ptr(9)}},
			}}
			repo := &inflightCountingAccountRepo{}
			svc := &OpenAIGatewayService{cfg: base.cfg, billingService: billing, resolver: NewModelPricingResolver(nil, billing), schedulerSnapshot: NewSchedulerSnapshotService(snap, nil, repo, nil, base.cfg)}
			key := &APIKey{User: &User{BillingIdentity: BillingIdentityPurchase}, GroupID: &groupID, Group: &Group{ID: groupID, Platform: PlatformComposite, RateMultiplier: 7}}
			ctx := WithResolvedTargetPlatform(context.Background(), platform)
			d := svc.inflightEstimateDeps()
			accounts, err := d.purchaseAccounts(ctx, key, model)
			require.NoError(t, err)
			require.Len(t, accounts, 1)
			require.Equal(t, int64(11), accounts[0].ID)
			require.Equal(t, []string{model}, d.accountMappedModels(ctx, key, "purchase-alias"))
			req := InflightEstimateRequest{Model: "purchase-alias", BodyBytes: 40, MaxTokens: 10}
			estimate, ok := svc.EstimateInflightReservation(ctx, key, req)
			require.True(t, ok)
			neutral := d.estimateOne(ctx, key, model, req, 1, 1)
			priced, err := billing.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, GroupID: key.GroupID, Group: key.Group, Tokens: UsageTokens{InputTokens: 10, OutputTokens: 10}, RateMultiplier: 1, PricingAt: time.Now(), Resolver: svc.resolver})
			require.NoError(t, err)
			require.InDelta(t, math.Max(neutral, priced.TotalCost)*0.35, estimate, 1e-12)
			require.Greater(t, estimate, 0.0)
			_, ok = svc.EstimateInflightReservation(context.Background(), key, req)
			require.False(t, ok, "unresolved composite must not reserve against an unrelated OpenAI account")
			require.Zero(t, repo.dbCalls.Load())
		})
	}
}

func TestBatchImagePurchasePricingIgnoresSalesCardAndUsesAccountRule(t *testing.T) {
	svc, _, _, _, _ := newTestBatchImagePublicService(true)
	groupID := int64(10)
	svc.GroupRepo = &publicBatchImageGroupRepo{groups: map[int64]*Group{groupID: {ID: groupID, Platform: PlatformGemini, AllowBatchImageGeneration: true, RateMultiplier: 9, ImagePrice1K: float64Ptr(99), BatchImageDiscountMultiplier: 0.9, BatchImageHoldMultiplier: 0.95}}}
	svc.ChannelService = newTestChannelServiceForStats(t, &Channel{ID: 1, Status: StatusActive, AccountStatsPricingRules: []AccountStatsPricingRule{{AccountIDs: []int64{3}, Pricing: []ChannelModelPricing{{Models: []string{"image-model"}, BillingMode: BillingModeImage, PerRequestPrice: float64Ptr(0.8)}}}}}, groupID, PlatformGemini)
	got, err := svc.resolvePricingSnapshot(context.Background(), BatchImageOwner{UserID: 1, GroupID: &groupID, BillingIdentity: BillingIdentityPurchase}, BatchImageSubmitRequest{Model: "image-model", ImageSize: "1K", Items: make([]BatchImageSubmitItem, 2)}, BatchImageProviderGeminiAPI, &Account{ID: 3, RateMultiplier: float64Ptr(0.3)})
	require.NoError(t, err)
	require.Equal(t, 0.8, got.BaseUnitPrice)
	require.Equal(t, 1.0, got.GroupRateMultiplier)
	require.Equal(t, 0.5, got.BatchDiscountMultiplier)
	require.InDelta(t, 0.12, got.BillableUnitPrice, 1e-12)
	require.InDelta(t, 0.24, got.EstimatedCost, 1e-12)
	require.GreaterOrEqual(t, got.HoldUnitPrice, got.BillableUnitPrice)
}
