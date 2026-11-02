package service

import (
	"context"
	"fmt"
	"math"
	"time"
)

const (
	BillingIdentityStandard = "standard"
	BillingIdentityPurchase = "purchase"
)

func (u *User) UsesPurchasePricing() bool {
	return u != nil && u.BillingIdentity == BillingIdentityPurchase
}

// Before account selection reserve the highest candidate procurement estimate.
// Unknown pricing is propagated to the existing configured admission policy.
func (d inflightEstimateDeps) estimatePurchase(ctx context.Context, key *APIKey, req InflightEstimateRequest) (float64, bool) {
	if d.purchaseAccounts == nil || d.billing == nil {
		return 0, false
	}
	accounts, err := d.purchaseAccounts(ctx, key, req.Model)
	if err != nil || len(accounts) == 0 {
		return 0, false
	}
	primary, _, upstream := inflightBillingModelCandidates(ctx, d, key, req.Model)
	input, output := tokenCounts(inflightReservationCfg(d.cfg), req.BodyBytes, req.MaxTokens)
	searchCost := 0.0
	if req.SearchCalls > 0 {
		searchCost = d.billing.CalculateSearchCost(req.SearchCalls, groupSearchPricePer1kFromAPIKey(key), 1).TotalCost
	}
	now := time.Now()
	best := 0.0
	for i := range accounts {
		account := &accounts[i]
		model := account.GetMappedModel(upstream)
		if model == "" {
			model = req.Model
		}
		neutral := d.estimateOne(ctx, key, model, req, 1, 1)
		for _, candidate := range primary {
			if c := d.estimateOne(ctx, key, candidate, req, 1, 1); c > neutral {
				neutral = c
			}
		}
		base := neutral
		knownCost := validCost(base)
		if d.resolver != nil {
			resolved := d.resolver.Resolve(ctx, PricingInput{Model: model, GroupID: key.GroupID, Group: key.Group})
			cost, costErr := d.billing.CalculateCostUnified(CostInput{Ctx: ctx, Model: model, GroupID: key.GroupID, Group: key.Group, Tokens: UsageTokens{InputTokens: input, OutputTokens: output}, RateMultiplier: 1, PricingAt: now, Resolver: d.resolver, Resolved: resolved})
			if req.Kind == InflightEstimateToken && costErr == nil && cost != nil && cost.TotalCost > base {
				base = cost.TotalCost
				knownCost = true
			}
			if key.GroupID != nil && d.accountStatsEstimate != nil {
				count := req.Units
				if count <= 0 {
					count = 1
				}
				if stats := d.accountStatsEstimate(ctx, account.ID, *key.GroupID, model, UsageTokens{InputTokens: input, OutputTokens: output}, count, base-searchCost, now, accountStatsLongContextPricingEnabled(openAILongContextBillingGate(account))); stats != nil {
					base = *stats + searchCost
					knownCost = base >= 0 && !math.IsNaN(base) && !math.IsInf(base, 0)
				}
			}
		}
		if !knownCost && account.BillingRateMultiplier() != 0 {
			return 0, false
		}
		charge := base * account.BillingRateMultiplier()
		if charge < 0 || math.IsNaN(charge) || math.IsInf(charge, 0) {
			return 0, false
		}
		if charge > best {
			best = charge
		}
	}
	return best, true
}

// applyPurchasePricing charges the selected account's recorded procurement cost.
// AccountStatsCost already includes channel/time pricing; never apply sales peak
// factors again. This runs before the shared balance/quota/dedup billing command.
func applyPurchasePricing(user *User, cost *CostBreakdown, log *UsageLog, accountRate float64) error {
	if !user.UsesPurchasePricing() {
		return nil
	}
	if cost == nil || log == nil {
		return fmt.Errorf("purchase pricing requires cost and usage details")
	}
	base := cost.TotalCost
	if log.AccountStatsCost != nil {
		base = *log.AccountStatsCost
	}
	actual := base * accountRate
	if base < 0 || accountRate < 0 || math.IsNaN(base) || math.IsInf(base, 0) || math.IsNaN(accountRate) || math.IsInf(accountRate, 0) || math.IsNaN(actual) || math.IsInf(actual, 0) {
		return fmt.Errorf("invalid selected-account procurement cost")
	}
	cost.ActualCost = actual
	log.ActualCost = actual
	if cost.TotalCost > 0 {
		log.RateMultiplier = actual / cost.TotalCost
	} else {
		log.RateMultiplier = accountRate
	}
	return nil
}
