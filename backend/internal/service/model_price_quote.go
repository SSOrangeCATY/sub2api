package service

import (
	"context"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"math"
	"strings"
	"time"
)

// PlazaPriceQuote contains public final prices, never account identities,
// supplier credentials or internal rule IDs. Variants represent configured
// candidate routes; availability and actual selection can change per request.
type PlazaPriceQuote struct {
	QuotedAt time.Time
	Status   string
	Reason   string
	Variants []PlazaPriceVariant
}

type PlazaPriceVariant struct {
	Pricing           *ChannelModelPricing
	TimePricing       *TimePricingSchedule
	LongContextBasis  ContextPricingBasis
	ContextMetric     string
	IntervalPriceMode string
}

type ModelPriceQuoteService struct {
	now      func() time.Time
	accounts AccountRepository
	channels *ChannelService
	billing  *BillingService
	resolver *ModelPricingResolver
}

func NewModelPriceQuoteService(accounts AccountRepository, channels *ChannelService, billing *BillingService, resolver *ModelPricingResolver) *ModelPriceQuoteService {
	return &ModelPriceQuoteService{accounts: accounts, channels: channels, billing: billing, resolver: resolver, now: time.Now}
}

// FillPurchasePrices runs only after group visibility has been checked.
// It deliberately keeps transiently unavailable configured routes in the
// range: a price display must not promise the cheapest currently idle route.
func (s *ModelPriceQuoteService) FillPurchasePrices(ctx context.Context, groups []PlazaGroup) {
	at := time.Now()
	if s.now != nil {
		at = s.now()
	}
	snapshot := *s
	snapshot.now = func() time.Time { return at }
	for gi := range groups {
		g := &groups[gi]
		platforms := make([]string, 0, len(g.Models))
		platformSet := map[string]bool{}
		for _, model := range g.Models {
			if model.Platform != "" && !platformSet[model.Platform] {
				platforms = append(platforms, model.Platform)
				platformSet[model.Platform] = true
			}
		}
		if platformSet[PlatformAnthropic] || platformSet[PlatformGemini] {
			platforms = append(platforms, PlatformAntigravity)
		}
		accounts, err := s.accounts.ListModelAvailabilityCandidates(ctx, &g.ID, platforms, false)
		group, groupErr := s.channels.groupRepo.GetByID(ctx, g.ID)
		channel, channelErr := s.channels.GetChannelForGroup(ctx, g.ID)
		for mi := range g.Models {
			m := &g.Models[mi]
			m.PriceQuote = &PlazaPriceQuote{Status: "unavailable", Reason: "pricing_unavailable"}
			if err != nil || groupErr != nil || channelErr != nil || group == nil {
				continue
			}
			q := snapshot.quoteModel(ctx, group, channel, m, accounts)
			q.QuotedAt = at
			m.PriceQuote = q
		}
	}
}

func (s *ModelPriceQuoteService) quoteModel(ctx context.Context, group *Group, channel *Channel, model *PlazaModel, accounts []Account) *PlazaPriceQuote {
	q := &PlazaPriceQuote{Status: "unavailable", Reason: "no_candidates"}
	if s.billing == nil || s.resolver == nil {
		return q
	}
	ctx = WithResolvedTargetPlatform(ctx, model.Platform)
	mapping := s.channels.ResolveChannelMapping(ctx, group.ID, model.Name)
	if mapping.BillingModelSource == "response_model" {
		q.Reason = "response_model"
		return q
	}
	upstream := mapping.MappedModel
	if upstream == "" {
		upstream = model.Name
	}
	if restrictedModel := billingModelForRestriction(mapping.BillingModelSource, model.Name, upstream); restrictedModel != "" && s.channels.IsModelRestricted(ctx, group.ID, restrictedModel) {
		return q
	}
	seen := map[string]bool{}
	for i := range accounts {
		a := &accounts[i]
		platformMatch := a.Platform == model.Platform
		if a.Platform == PlatformAntigravity && (model.Platform == PlatformAnthropic || model.Platform == PlatformGemini) {
			platformMatch = a.IsMixedSchedulingEnabled()
		}
		if !platformMatch || !(&GatewayService{}).isModelSupportedByAccountWithContext(ctx, a, upstream) {
			continue
		}
		mapped := quoteAccountUpstreamModel(a, upstream)
		if mapped == "" {
			mapped = upstream
		}
		if mapping.BillingModelSource == BillingModelSourceUpstream && s.channels.IsModelRestricted(ctx, group.ID, mapped) {
			continue
		}
		billingModel := model.Name
		switch mapping.BillingModelSource {
		case "upstream":
			billingModel = mapped
		case "channel_mapped":
			billingModel = upstream
		}
		variant := s.accountVariant(ctx, group, channel, a, mapped, billingModel)
		if variant == nil {
			return &PlazaPriceQuote{Status: "unavailable", Reason: "pricing_unavailable"}
		}
		data, err := json.Marshal(variant)
		if err != nil {
			return &PlazaPriceQuote{Status: "unavailable", Reason: "pricing_unavailable"}
		}
		key := string(data)
		if seen[key] {
			continue
		}
		seen[key] = true
		if len(seen) > 32 {
			return &PlazaPriceQuote{Status: "unavailable", Reason: "too_many_variants"}
		}
		q.Variants = append(q.Variants, *variant)
	}
	if len(q.Variants) > 0 {
		q.Status, q.Reason = "quoted", ""
	}
	return q
}

func (s *ModelPriceQuoteService) accountVariant(ctx context.Context, group *Group, channel *Channel, account *Account, upstream, billingModel string) *PlazaPriceVariant {
	var pricing *ChannelModelPricing
	variant := &PlazaPriceVariant{ContextMetric: "input"}
	if channel != nil {
		for _, rule := range channel.AccountStatsPricingRules {
			if !matchAccountStatsRule(&rule, account.ID, group.ID) {
				continue
			}
			if card := findPricingForModel(rule.Pricing, s.channels.GetGroupPlatform(ctx, group.ID), strings.ToLower(upstream)); card != nil {
				cp := card.Clone()
				pricing = &cp
				variant.ContextMetric = "total"
				variant.IntervalPriceMode = "absolute"
				// Account-cost token intervals replace the flat card; they do
				// not inherit its missing fields or use sales multipliers.
				if pricing.BillingMode == BillingModeImage || pricing.BillingMode == BillingModeVideo || pricing.BillingMode == BillingModePerRequest {
					pricing.Intervals = nil
					// Custom account stats count video requests, not seconds.
					if pricing.BillingMode == BillingModeVideo {
						pricing.BillingMode = BillingModePerRequest
					}
				} else {
					for i := range pricing.Intervals {
						iv := &pricing.Intervals[i]
						iv.InputMultiplier, iv.OutputMultiplier, iv.CacheWriteMultiplier, iv.CacheReadMultiplier = nil, nil, nil, nil
					}
					pricing.ImageInputPrice = nil
				}
				break
			}
		}
	}
	if pricing == nil && channel != nil && !channel.ApplyPricingToAccountStats {
		neutral := s.resolver.Resolve(ctx, PricingInput{Model: billingModel, Group: group, GroupID: &group.ID})
		if neutral != nil && neutral.Mode != "" && neutral.Mode != BillingModeToken {
			if native, err := s.billing.GetModelPricing(upstream); err == nil && native != nil {
				// The native override depends on reported token usage. A
				// per-image/second number cannot represent both possibilities.
				return nil
			}
		}
		long := accountStatsLongContextPricingEnabled(openAILongContextBillingGate(account))
		long = long && !openAISubscriptionLongContextExempt(account, &OpenAIForwardResult{Model: upstream, UpstreamModel: upstream})
		at := time.Now()
		if s.now != nil {
			at = s.now()
		}
		sched, err := s.billing.ResolveContextPricingSchedule(ctx, NewModelPricingResolver(nil, s.billing), ContextPricingScheduleInput{Model: upstream, LongContextPricingEnabled: &long, PricingAt: &at})
		if err == nil && sched != nil && len(sched.Tiers) > 0 {
			pricing = plazaPricingFromSchedule(nil, sched)
			variant.LongContextBasis, variant.TimePricing = sched.Basis, sched.TimePricing
		}
	}
	if pricing == nil {
		platform := account.Platform
		if target, ok := ResolvedTargetPlatformFromContext(ctx); ok {
			platform = target
		}
		m := PlazaModel{Name: billingModel, Platform: platform}
		// Use the same resolved source as the group's actual neutral charge.
		resolved := s.resolver.Resolve(ctx, PricingInput{Model: billingModel, Group: group, GroupID: &group.ID})
		if resolved != nil {
			m.Pricing = resolved.channelPricing
		}
		plaza := &ModelPlazaService{billingService: s.billing, resolver: s.resolver}
		plaza.fillDisplayPricing(ctx, &m, group)
		pricing = m.Pricing
		variant.LongContextBasis, variant.TimePricing = m.LongContextBasis, m.TimePricing
	}
	if pricing == nil {
		return nil
	}
	cp := pricing.Clone()
	pricing = &cp
	if pricing.BillingMode == "" {
		pricing.BillingMode = BillingModeToken
	}
	// Unsupported media catalog prices must not silently become a token quote.
	if pricing.BillingMode == BillingModeToken && pricing.InputPrice == nil && pricing.OutputPrice == nil && len(pricing.Intervals) == 0 {
		return nil
	}
	if !scaleQuotedPrices(pricing, account.BillingRateMultiplier()) {
		return nil
	}
	variant.Pricing = pricing
	return variant
}

func quoteAccountUpstreamModel(account *Account, model string) string {
	if account.Platform == PlatformAntigravity {
		return mapAntigravityModel(account, model)
	}
	if account.IsBedrock() {
		mapped, _ := ResolveBedrockModelID(account, model)
		return mapped
	}
	if account.Platform == PlatformAnthropic && account.Type != AccountTypeAPIKey {
		if account.Type == AccountTypeServiceAccount {
			if mapped, ok := account.ResolveMappedModel(model); ok {
				return mapped
			}
			return normalizeVertexAnthropicModelID(claude.NormalizeModelID(model))
		}
		return claude.NormalizeModelID(model)
	}
	return account.GetMappedModel(model)
}

func scaleQuotedPrices(p *ChannelModelPricing, rate float64) bool {
	if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0 {
		return false
	}
	scale := func(value **float64) bool {
		if *value == nil {
			return true
		}
		n := **value * rate
		if math.IsNaN(n) || math.IsInf(n, 0) || n < 0 {
			return false
		}
		*value = &n
		return true
	}
	for _, field := range []**float64{&p.InputPrice, &p.OutputPrice, &p.CacheWritePrice, &p.CacheWrite1hPrice, &p.CacheWrite12hPrice, &p.CacheReadPrice, &p.ImageInputPrice, &p.ImageOutputPrice, &p.PerRequestPrice} {
		if !scale(field) {
			return false
		}
	}
	for i := range p.Intervals {
		iv := &p.Intervals[i]
		iv.ID, iv.PricingID, iv.SortOrder = 0, 0, 0
		iv.CreatedAt, iv.UpdatedAt = time.Time{}, time.Time{}
		for _, field := range []**float64{&iv.InputPrice, &iv.OutputPrice, &iv.CacheWritePrice, &iv.CacheWrite1hPrice, &iv.CacheWrite12hPrice, &iv.CacheReadPrice, &iv.PerRequestPrice} {
			if !scale(field) {
				return false
			}
		}
	}
	// Rule matching lists are internal metadata; do not participate in public
	// deduplication or reveal how individual supplier accounts were selected.
	p.ID, p.ChannelID = 0, 0
	p.Models = nil
	p.CreatedAt, p.UpdatedAt = time.Time{}, time.Time{}
	p.Platform = ""
	p.TimePricing = nil
	p.FastMultiplier, p.FlexMultiplier = nil, nil
	return true
}
