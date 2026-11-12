import { describe, expect, it } from 'vitest'
import { group, model, pricing } from '@/components/modelPlaza/__tests__/fixtures'
import { contextualPricing, currentVariants, peakMultiplier, priceRanges, pricingVariants, scheduledMultiplier } from '../modelPriceQuote'

describe('price quotation semantics', () => {
  it('formats native denominations without converting numerical values', () => {
    expect(priceRanges([{ pricing: pricing({ source_currency: 'CNY' }) }], 'input_price')[0].text).toBe('¥2.00')
    expect(priceRanges([{ pricing: pricing() }], 'input_price')[0].text).toBe('$2.00')
  })
  it('scales all customer route variants once and preserves an unavailable native quote', () => {
    const m = model({ price_quote: { kind: 'customer', status: 'quoted', variants: [{ pricing: pricing({ input_price: 2e-6 }) }, { pricing: pricing({ input_price: 4e-6 }) }] } })
    const quotes = pricingVariants(m, group({ user_rate_multiplier: .25 }), false)
    expect(priceRanges(quotes, 'input_price')[0].text).toBe('$0.50 – $1.00')
    expect(m.price_quote!.variants[0].pricing.input_price).toBe(2e-6)
    expect(pricingVariants(model({ price_quote: { kind: 'customer', status: 'unavailable', variants: [] } }), group(), false)).toEqual([])
  })
  it('never folds different denominations into a single range', () => {
    const ranges = priceRanges([{ pricing: pricing({ source_currency: 'CNY' }) }, { pricing: pricing({ source_currency: 'USD', input_price: 100e-6 }) }], 'input_price')
    expect(ranges).toHaveLength(2)
    expect(ranges.map(r => r.currency)).toEqual(['CNY', 'USD'])
  })
  it('does not advertise a complete range if a candidate has an unknown component', () => {
    expect(priceRanges([{ pricing: pricing() }, { pricing: pricing({ input_price: null }) }], 'input_price')).toEqual([])
  })
  it('does not mutate the shared base or interval card while applying a personal rate', () => {
    const p = pricing()
    const result = pricingVariants(model({ pricing: p }), group({ user_rate_multiplier: .1 }), false)
    expect(result[0].pricing.input_price).toBeCloseTo(.2e-6)
    expect(p.input_price).toBe(2e-6)
  })
  it.each([0, .25, 1])('preserves an explicitly configured independent media rate %s', rate => {
    const m = model({ pricing: pricing({ billing_mode: 'image', per_request_price: 2 }) })
    const result = pricingVariants(m, group({ image_rate_independent: true, image_rate_multiplier: rate }), false)
    expect(result[0].pricing.per_request_price).toBe(2 * rate)
  })
  it('uses an absolute account-cost interval without inheriting sales fields or multipliers', () => {
    const v = { pricing: pricing({ intervals: [{ min_tokens: 0, max_tokens: null, input_price: 3e-6, output_price: null, cache_write_price: null, cache_read_price: null, output_multiplier: 10, per_request_price: null }] }), interval_price_mode: 'absolute' as const }
    expect(contextualPricing(v, 5).input_price).toBe(3e-6)
    expect(contextualPricing(v, 5).output_price).toBeNull()
  })
  it('resolves relative sales intervals and the inclusive upper boundary', () => {
    const p = pricing({ intervals: [{ min_tokens: 0, max_tokens: 100, input_price: null, output_price: null, input_multiplier: 2, cache_write_price: null, cache_read_price: null, per_request_price: null }] })
    expect(contextualPricing({ pricing: p }, 100).input_price).toBe(4e-6)
    expect(contextualPricing({ pricing: p }, 101).input_price).toBe(2e-6)
  })
  it('uses the first procurement interval for base display while preserving gap and finite-tail fallback', () => {
    const p = pricing({ input_price: 9e-6, intervals: [
      { min_tokens: 0, max_tokens: 100, input_price: 2e-6, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null },
      { min_tokens: 200, max_tokens: 300, input_price: 4e-6, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: null }
    ] })
    const variant = { pricing: p, interval_price_mode: 'absolute' as const }
    expect(contextualPricing(variant, 0).input_price).toBe(2e-6)
    expect(contextualPricing(variant, 150).input_price).toBe(9e-6)
    expect(contextualPricing(variant, 400).input_price).toBe(9e-6)
  })
  it('applies weekdays-only periods in their configured timezone', () => {
    const schedule = { timezone: 'Asia/Shanghai', weekdays_only: true, periods: [{ start_time: '09:00', end_time: '11:00', multiplier: .5 }] }
    expect(scheduledMultiplier(schedule, new Date('2026-11-12T02:00:00Z'))).toBe(.5)
    expect(scheduledMultiplier(schedule, new Date('2026-11-14T02:00:00Z'))).toBe(1)
    expect(scheduledMultiplier(schedule, new Date('2026-11-12T03:00:00Z'))).toBe(1)
  })
  it('applies group peaks only to subscription token prices, including free peaks', () => {
    const g = group({ subscription_type: 'subscription', peak_rate_enabled: true, peak_start: '09:00', peak_end: '11:00', peak_rate_multiplier: 0 })
    const at = new Date('2026-11-12T02:00:00Z')
    expect(peakMultiplier(g, at, '+08:00')).toBe(0)
    expect(peakMultiplier({ ...g, subscription_type: 'standard' }, at, '+08:00')).toBe(1)
    expect(currentVariants(model(), g, false, at, '+08:00')[0].pricing.input_price).toBe(0)
    expect(currentVariants(model({ pricing: pricing({ billing_mode: 'image', per_request_price: 2 }) }), g, false, at, '+08:00')[0].pricing.per_request_price).toBe(1)
  })
  it('does not apply a sales peak or group rate to a procurement snapshot', () => {
    const g = group({ subscription_type: 'subscription', peak_rate_enabled: true, peak_start: '09:00', peak_end: '11:00', peak_rate_multiplier: 8, rate_multiplier: 99 })
    const m = model({ price_quote: { status: 'quoted', variants: [{ pricing: pricing() }] } })
    expect(currentVariants(m, g, true, new Date('2026-11-12T02:00:00Z'), '+08:00')[0].pricing.input_price).toBe(2e-6)
  })
})
