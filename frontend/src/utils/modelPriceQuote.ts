import type { ModelPlazaGroup, ModelPriceVariant, PlazaModel, PlazaTimePricing } from '@/api/modelPlaza'
import type { UserSupportedModelPricing, UserPricingInterval } from '@/api/channels'
import { resolveIntervalPrices, formatScaled } from './pricing'

export type PriceField = 'input_price' | 'output_price' | 'cache_read_price' | 'cache_write_price' | 'cache_write_1h_price' | 'cache_write_12h_price' | 'per_request_price' | 'image_input_price' | 'image_output_price'
const fields: PriceField[] = ['input_price', 'output_price', 'cache_read_price', 'cache_write_price', 'cache_write_1h_price', 'cache_write_12h_price', 'per_request_price', 'image_input_price', 'image_output_price']
export type QuoteGroup = Pick<ModelPlazaGroup, 'rate_multiplier'> & Partial<ModelPlazaGroup>

export function scalePricing(pricing: UserSupportedModelPricing, rate: number): UserSupportedModelPricing {
  const scaled = { ...pricing, intervals: pricing.intervals.map(iv => ({ ...iv })) }
  for (const field of fields) {
    const value = pricing[field]
    if (value != null) scaled[field] = value * rate
  }
  for (const interval of scaled.intervals) {
    for (const field of fields) {
      if (field in interval) {
        const key = field as keyof UserPricingInterval
        const value = interval[key]
        if (typeof value === 'number') Object.assign(interval, { [field]: value * rate })
      }
    }
  }
  return scaled
}

export function pricingVariants(model: PlazaModel, group: QuoteGroup, purchase: boolean): ModelPriceVariant[] {
  if (purchase) return model.price_quote?.status === 'quoted' ? model.price_quote.variants : []
  if (!model.pricing) return []
  let rate = group.user_rate_multiplier ?? group.rate_multiplier
  if (model.pricing.billing_mode === 'image' && group.image_rate_independent) rate = group.image_rate_multiplier ?? 1
  if (model.pricing.billing_mode === 'video' && group.video_rate_independent) rate = Math.max(0, group.video_rate_multiplier ?? 1)
  if (!Number.isFinite(rate) || rate < 0) return []
  return [{ pricing: scalePricing(model.pricing, rate), time_pricing: model.time_pricing, long_context_basis: model.long_context_basis, context_metric: 'input' }]
}

const clockSeconds = (value: string) => {
  const [hour, minute, second = 0] = value.split(':').map(Number)
  return hour * 3600 + minute * 60 + second
}

export function scheduledMultiplier(schedule: PlazaTimePricing | undefined, at: Date): number {
  if (!schedule) return 1
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone: schedule.timezone, weekday: 'short', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23' }).formatToParts(at)
    const get = (type: string) => parts.find(part => part.type === type)?.value ?? ''
    if (schedule.weekdays_only && ['Sat', 'Sun'].includes(get('weekday'))) return 1
    const seconds = Number(get('hour')) * 3600 + Number(get('minute')) * 60 + Number(get('second'))
    return schedule.periods.find(period => {
      const start = clockSeconds(period.start_time), end = clockSeconds(period.end_time) || 86400
      return seconds >= start && seconds < end
    })?.multiplier ?? 1
  } catch { return 1 }
}

export function peakMultiplier(group: QuoteGroup, at: Date, offset?: string): number {
  if (group.subscription_type !== 'subscription' || !group.peak_rate_enabled || !group.peak_start || !group.peak_end || !offset) return 1
  const match = /^([+-])(\d{2}):(\d{2})$/.exec(offset)
  if (!match) return 1
  const minutes = (Number(match[2]) * 60 + Number(match[3])) * (match[1] === '-' ? -1 : 1)
  const local = new Date(at.getTime() + minutes * 60000)
  const seconds = local.getUTCHours() * 3600 + local.getUTCMinutes() * 60 + local.getUTCSeconds()
  const start = clockSeconds(group.peak_start), end = clockSeconds(group.peak_end)
  const active = start < end && seconds >= start && seconds < end
  return active ? group.peak_rate_multiplier ?? 1 : 1
}

export function contextualPricing(variant: ModelPriceVariant, contextTokens: number): UserSupportedModelPricing {
  const base = variant.pricing
  if (!base.intervals.length || base.billing_mode !== 'token') return base
  const effectiveContext = contextTokens || 1
  const interval = base.intervals.find(iv => effectiveContext > iv.min_tokens && (iv.max_tokens == null || effectiveContext <= iv.max_tokens))
  if (!interval) return base
  if (variant.interval_price_mode === 'absolute') {
    const selected = { ...base }
    for (const field of fields) {
      const value = Reflect.get(interval, field)
      selected[field] = typeof value === 'number' ? value : null
    }
    return selected
  }
  return { ...base, ...resolveIntervalPrices(interval, base) }
}

export function hasUncoveredContext(pricing: UserSupportedModelPricing): boolean {
  if (!pricing.intervals.length) return false
  let covered = 0
  for (const interval of [...pricing.intervals].sort((a, b) => a.min_tokens - b.min_tokens)) {
    if (interval.min_tokens > covered) return true
    if (interval.max_tokens == null) return false
    covered = Math.max(covered, interval.max_tokens)
  }
  return true
}

export function currentVariants(model: PlazaModel, group: QuoteGroup, purchase: boolean, at: Date, offset?: string): ModelPriceVariant[] {
  return pricingVariants(model, group, purchase).map(variant => ({
    ...variant,
    pricing: scalePricing(variant.pricing, scheduledMultiplier(variant.time_pricing, at) * (purchase || variant.pricing.billing_mode !== 'token' ? 1 : peakMultiplier(group, at, offset)))
  }))
}

export function priceRanges(variants: ModelPriceVariant[], field: PriceField, contextTokens = 0): Array<{ currency: string; min: number; max: number; text: string }> {
  const buckets = new Map<string, number[]>()
  const unresolved = new Set<string>()
  for (const variant of variants) {
    const pricing = contextualPricing(variant, contextTokens)
    const value = pricing[field]
    const currency = pricing.source_currency || 'USD'
    // An unresolved component is not a confirmed zero-price component.
    if (value == null || !Number.isFinite(value) || value < 0) { unresolved.add(currency); continue }
    const scale = pricing.billing_mode === 'token' ? 1_000_000 : 1
    buckets.set(currency, [...(buckets.get(currency) ?? []), value * scale])
  }
  return [...buckets.entries()].filter(([currency]) => !unresolved.has(currency)).sort(([a], [b]) => a.localeCompare(b)).map(([currency, values]) => {
    const min = Math.min(...values), max = Math.max(...values)
    const a = formatScaled(min, 1, 2, currency), b = formatScaled(max, 1, 2, currency)
    return { currency, min, max, text: Math.abs(max - min) <= Math.max(1e-12, max * 1e-10) ? a : a + ' – ' + b }
  })
}
