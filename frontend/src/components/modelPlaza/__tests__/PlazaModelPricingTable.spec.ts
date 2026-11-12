import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'
import PriceUnitHelp from '../PriceUnitHelp.vue'
import { group, model, pricing } from './fixtures'
import type { PlazaModel } from '@/api/modelPlaza'

const mounted: VueWrapper[] = []
beforeEach(() => { setActivePinia(createPinia()); vi.useFakeTimers(); vi.setSystemTime(new Date('2026-11-12T02:00:00Z')) })
afterEach(() => { mounted.splice(0).forEach(wrapper => wrapper.unmount()); vi.useRealTimers() })
function table(models: PlazaModel[], props = {}) {
  const wrapper = mount(PlazaModelPricingTable, { props: { models, rateMultiplier: .5, ...props }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
  mounted.push(wrapper)
  return wrapper
}

describe('effective model prices', () => {
  it('shows effective prices directly and keeps the official reference in details', async () => {
    const wrapper = table([model()], { userRateMultiplier: .25 })
    expect(wrapper.find('.price-summary').text()).toContain('$0.50')
    expect(wrapper.find('.price-summary').text()).toContain('$2.00')
    expect(wrapper.find('.price-summary').text()).not.toContain('0.25x')
    expect(wrapper.find('.price-official').exists()).toBe(false)
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.find('.price-official').text()).toContain('$10.00')
    expect(wrapper.find('.price-official').text()).toContain('$40.00')
    expect(wrapper.find('.price-details-button').attributes('aria-expanded')).toBe('true')
  })

  it('uses a procurement quote without multiplying it by a sales or personal rate', () => {
    const m = model({ price_quote: { status: 'quoted', variants: [{ pricing: pricing({ input_price: .3e-6, output_price: .9e-6 }) }] } })
    const wrapper = table([m], { purchasePricing: true, rateMultiplier: 99, userRateMultiplier: 88 })
    expect(wrapper.find('.price-summary').text()).toContain('$0.30')
    expect(wrapper.find('.price-summary').text()).toContain('$0.90')
    expect(wrapper.text()).not.toContain('99x')
    expect(wrapper.text()).not.toContain('$2.00')
  })

  it('does not substitute a sales card when the procurement quote is unavailable', () => {
    const wrapper = table([model()], { purchasePricing: true })
    expect(wrapper.find('.price-summary').text()).not.toMatch(/\$\d/)
    expect(wrapper.find('.price-unavailable-row').text()).toContain('不会使用销售价代替进货价')
  })

  it('displays route ranges by denomination and preserves explicit free prices', () => {
    const m = model({ price_quote: { status: 'quoted', variants: [
      { pricing: pricing({ source_currency: 'CNY', input_price: 0, output_price: 1e-6 }) },
      { pricing: pricing({ source_currency: 'CNY', input_price: 2e-6, output_price: 4e-6 }) }
    ] } })
    const wrapper = table([m], { purchasePricing: true })
    expect(wrapper.text()).toContain('¥0.00 – ¥2.00')
    expect(wrapper.text()).toContain('¥1.00 – ¥4.00')
  })

  it('uses CNY from the price card and never relabels the USD reference', async () => {
    const wrapper = table([model({ pricing: pricing({ source_currency: 'CNY' }) })])
    expect(wrapper.find('.price-summary').text()).toContain('¥1.00')
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.find('.price-official').text()).toContain('USD')
    expect(wrapper.find('.price-official').text()).toContain('$10.00')
    expect(wrapper.findAllComponents(PriceUnitHelp)).toHaveLength(1)
  })

  it('discloses every cache duration and applicable reasoning factor in details', async () => {
    const wrapper = table([model({ pricing: pricing({ reasoning_effort_multipliers: { high: 2 } }) })])
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.text()).toContain('缓存写入 / 5m')
    expect(wrapper.text()).toContain('缓存写入 / 1h')
    expect(wrapper.text()).toContain('缓存写入 / 12h')
    expect(wrapper.text()).toContain('$3.00')
    expect(wrapper.text()).toContain('high')
    expect(wrapper.text()).toContain('未配置的等级按 1 倍计费')
  })

  it('uses configured tier prices for a selected context without hiding the full sheet', async () => {
    const m = model()
    m.pricing!.intervals = [
      { min_tokens: 0, max_tokens: 100, input_price: 2e-6, output_price: 8e-6, cache_write_price: null, cache_read_price: null, per_request_price: null },
      { min_tokens: 100, max_tokens: null, input_price: 4e-6, output_price: 16e-6, cache_write_price: null, cache_read_price: null, per_request_price: null }
    ]
    const wrapper = table([m], { contextTokens: 101 })
    expect(wrapper.find('.price-summary').text()).toContain('$2.00')
    expect(wrapper.find('.price-summary').text()).toContain('$8.00')
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.findAll('.price-tier')).toHaveLength(3)
  })

  it('does not inherit an absent procurement interval output price from the base', async () => {
    const p = pricing({ output_price: 10e-6, intervals: [{ min_tokens: 0, max_tokens: null, input_price: 2e-6, output_price: null, cache_write_price: null, cache_read_price: null, output_multiplier: 3, per_request_price: null }] })
    const wrapper = table([model({ price_quote: { status: 'quoted', variants: [{ pricing: p, interval_price_mode: 'absolute', context_metric: 'total' }] } })], { purchasePricing: true, contextTokens: 5 })
    expect(wrapper.findAll('.price-number-cell')[1].text()).toBe('—')
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.find('.price-variant').text()).not.toContain('$10.00')
  })

  it.each(['image', 'video', 'per_request'] as const)('keeps %s prices in their own unit', type => {
    const m = model({ pricing: pricing({ billing_mode: type, per_request_price: .2 }), official_pricing: null })
    const wrapper = table([m], { imageRateIndependent: true, imageRateMultiplier: .25, videoRateIndependent: true, videoRateMultiplier: 0 })
    const expected = type === 'video' ? '$0.00' : type === 'image' ? '$0.05' : '$0.10'
    expect(wrapper.find('.price-request-cell').text()).toContain(expected)
    expect(wrapper.text()).not.toContain('$100000')
  })

  it('renders mixed token and per-request routes independently', async () => {
    const m = model({ price_quote: { status: 'quoted', variants: [{ pricing: pricing() }, { pricing: pricing({ billing_mode: 'per_request', per_request_price: .07 }) }] } })
    const wrapper = table([m], { purchasePricing: true })
    expect(wrapper.find('.price-summary').text()).toContain('$2.00')
    expect(wrapper.find('.price-mixed-row').text()).toContain('$0.07')
    expect(wrapper.find('.price-mixed-row').text()).toContain('/ 次')
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.find('.price-details').text()).toContain('$0.07')
    expect(wrapper.find('.price-details').text()).not.toContain('$70000')
  })

  it('keeps composite models with identical names distinct by group and platform', () => {
    const a = group()
    const b = group({ id: 2, name: 'Another group', platform: 'anthropic' })
    const wrapper = table([], { entries: [{ group: a, model: model() }, { group: b, model: model({ platform: 'anthropic' }) }] })
    expect(wrapper.findAll('.price-summary')).toHaveLength(2)
    expect(wrapper.text()).toContain('Another group')
    expect(wrapper.text()).toContain('anthropic')
  })

  it.each(['quoted_at', 'pricing_as_of'])('removes an expired %s snapshot from expanded price details too', async field => {
    const quotedAt = new Date().toISOString()
    const m = field === 'quoted_at'
      ? model({ price_quote: { status: 'quoted', quoted_at: quotedAt, variants: [{ pricing: pricing() }] } })
      : model({ pricing_as_of: quotedAt })
    const wrapper = table([m], { purchasePricing: field === 'quoted_at' })
    await wrapper.find('.price-details-button').trigger('click')
    expect(wrapper.find('.price-variant').exists()).toBe(true)
    await vi.advanceTimersByTimeAsync(105000)
    expect(wrapper.find('.price-variant').exists()).toBe(false)
    expect(wrapper.find('.price-unavailable-row').exists()).toBe(true)
  })

  it('retains flat route fallbacks when comparing image resolution tiers', () => {
    const m = model({ price_quote: { status: 'quoted', variants: [
      { pricing: pricing({ billing_mode: 'image', per_request_price: .3, intervals: [
        { min_tokens: 0, max_tokens: null, tier_label: '1K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: .1 },
        { min_tokens: 0, max_tokens: null, tier_label: '2K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: .2 }
      ] }) },
      { pricing: pricing({ billing_mode: 'image', per_request_price: .4 }), interval_price_mode: 'absolute' }
    ] } })
    const wrapper = table([m], { purchasePricing: true })
    expect(wrapper.find('.price-request-cell').text()).toContain('$0.10 – $0.40')
    expect(wrapper.find('.price-request-cell').text()).toContain('$0.20 – $0.40')
  })

  it('does not advertise a neutral zero tier as free while preserving custom flat zero', async () => {
    const p = pricing({ billing_mode: 'image', per_request_price: .4, intervals: [{ min_tokens: 0, max_tokens: null, tier_label: '1K', input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: 0 }] })
    const neutral = table([model({ pricing: p })])
    expect(neutral.find('.price-request-cell').text()).toContain('暂不能确认价格')
    expect(neutral.find('.price-request-cell').text()).not.toContain('$0.00')
    await neutral.find('.price-details-button').trigger('click')
    expect(neutral.find('.price-variant').text()).toContain('暂不能确认价格')
    const custom = table([model({ price_quote: { status: 'quoted', variants: [{ pricing: pricing({ billing_mode: 'image', per_request_price: 0 }), interval_price_mode: 'absolute' }] } })], { purchasePricing: true })
    expect(custom.find('.price-request-cell').text()).toContain('$0.00')
  })

  it('does not guess a missing named tier when another route has context-tier fallback', () => {
    const interval = (label: string, price: number) => ({ min_tokens: 0, max_tokens: 1000, tier_label: label, input_price: null, output_price: null, cache_write_price: null, cache_read_price: null, per_request_price: price })
    const m = model({ price_quote: { status: 'quoted', variants: [
      { pricing: pricing({ billing_mode: 'image', per_request_price: .4, intervals: [interval('1K', .1)] }) },
      { pricing: pricing({ billing_mode: 'image', per_request_price: .4, intervals: [interval('2K', .2)] }) }
    ] } })
    const wrapper = table([m], { purchasePricing: true, contextTokens: 100 })
    expect(wrapper.find('.price-request-cell').text()).toContain('暂不能确认价格')
    expect(wrapper.find('.price-request-cell').text()).not.toContain('$0.10 – $0.40')
  })
})
