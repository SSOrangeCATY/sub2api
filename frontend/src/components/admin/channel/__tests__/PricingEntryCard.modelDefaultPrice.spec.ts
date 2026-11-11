import { flushPromises, shallowMount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import PricingEntryCard from '../PricingEntryCard.vue'
import type { PricingFormEntry } from '../types'
import channelsAPI from '@/api/admin/channels'
import type { ModelDefaultPricing } from '@/api/admin/channels'

vi.mock('@/api/admin/channels', () => ({
  default: { getModelDefaultPricing: vi.fn() },
}))

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

const getModelDefaultPricing = vi.mocked(channelsAPI.getModelDefaultPricing)

function createEntry(models: string[] = []): PricingFormEntry {
  return {
    models,
    billing_mode: 'token',
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    fast_multiplier: null,
    flex_multiplier: null,
    reasoning_effort_multipliers: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    time_pricing: {
      timezone: 'Asia/Shanghai',
      weekdays_only: false,
      periods: [],
    },
  }
}

function addModels(entry: PricingFormEntry, models: string[]) {
  const wrapper = shallowMount(PricingEntryCard, { props: { entry } })
  wrapper.findComponent({ name: 'ModelTagInput' }).vm.$emit('update:models', models)
  return wrapper
}

const emittedEntries = (wrapper: ReturnType<typeof shallowMount>) =>
  (wrapper.emitted('update') ?? []).map(e => e[0] as PricingFormEntry)

describe('PricingEntryCard default price auto-fill', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getModelDefaultPricing.mockResolvedValue({ found: true, input_price: 3e-6, output_price: 15e-6 })
  })

  it('auto-fills the default price when a model is added to an empty entry', async () => {
    const wrapper = addModels(createEntry(), ['claude-opus-4-8'])
    await flushPromises()

    expect(getModelDefaultPricing).toHaveBeenCalledTimes(1)
    expect(getModelDefaultPricing).toHaveBeenCalledWith('claude-opus-4-8')
    expect(emittedEntries(wrapper).at(-1)).toMatchObject({
      models: ['claude-opus-4-8'],
      input_price: 3,
      output_price: 15,
    })
  })

  it('does not overwrite prices when appending a model to an entry that already has one', async () => {
    const entry = createEntry(['claude-opus-4-7'])
    const wrapper = addModels(entry, ['claude-opus-4-7', 'claude-opus-4-8'])
    await flushPromises()

    expect(getModelDefaultPricing).not.toHaveBeenCalled()
    expect(emittedEntries(wrapper).at(-1)).toMatchObject({
      models: ['claude-opus-4-7', 'claude-opus-4-8'],
    })
    for (const emitted of emittedEntries(wrapper)) {
      expect(emitted.input_price).toBeNull()
      expect(emitted.output_price).toBeNull()
      expect(emitted.cache_write_price).toBeNull()
      expect(emitted.cache_read_price).toBeNull()
    }
  })

  it('does not auto-fill a shared price when several models are added to an empty entry at once', async () => {
    const wrapper = addModels(createEntry(), ['claude-opus-4-7', 'claude-opus-4-8'])
    await flushPromises()

    expect(getModelDefaultPricing).not.toHaveBeenCalled()
    expect(emittedEntries(wrapper).at(-1)).toMatchObject({
      models: ['claude-opus-4-7', 'claude-opus-4-8'],
      input_price: null,
      output_price: null,
    })
  })

  it('keeps manual prices untouched when a model is appended', async () => {
    const entry = { ...createEntry(['claude-opus-4-7']), input_price: '1.5' }
    const wrapper = addModels(entry, ['claude-opus-4-7', 'claude-opus-4-8'])
    await flushPromises()

    expect(getModelDefaultPricing).not.toHaveBeenCalled()
    for (const emitted of emittedEntries(wrapper)) {
      expect(emitted.input_price).toBe('1.5')
    }
  })

  it('imports CNY numbers as USD numbers without an exchange multiplier', async () => {
    getModelDefaultPricing.mockResolvedValue({ found: true, source_currency: 'CNY', input_price: 1e-6, output_price: 4e-6, cache_read_price: 0.02e-6 })
    const wrapper = addModels({ ...createEntry(), source_currency: 'CNY' }, ['deepseek-flash'])
    await flushPromises()
    expect(getModelDefaultPricing).toHaveBeenCalledWith('deepseek-flash', 'CNY')
    expect(emittedEntries(wrapper).at(-1)).toMatchObject({ source_currency: 'CNY', input_price: 1, output_price: 4, cache_read_price: 0.02 })
  })

  it('changing the source unit preserves entered numbers and does not import implicitly', async () => {
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: { ...createEntry(['any-model']), input_price: 7 } } })
    await wrapper.find('[data-testid="source-currency"]').setValue('CNY')
    expect(emittedEntries(wrapper).at(-1)).toMatchObject({ source_currency: 'CNY', input_price: 7 })
    expect(getModelDefaultPricing).not.toHaveBeenCalled()
  })

  it('leaves current prices intact when no native price card is available', async () => {
    getModelDefaultPricing.mockResolvedValue({ found: false })
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: { ...createEntry(['any-model']), source_currency: 'EUR', input_price: 7 } } })
    await wrapper.find('[data-testid="load-source-price"]').trigger('click')
    await flushPromises()
    expect(emittedEntries(wrapper)).toHaveLength(0)
    expect(wrapper.find('[role="status"]').text()).toBe('admin.channels.form.sourcePriceUnavailable')
  })

  it.each(['price', 'model', 'mode', 'currency'])('ignores a pending preset after a %s edit and permits another import', async (edit) => {
    let resolve!: (value: ModelDefaultPricing) => void
    getModelDefaultPricing.mockReturnValue(new Promise<ModelDefaultPricing>(done => { resolve = done }))
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: createEntry(['old-model']) } })
    await wrapper.find('[data-testid="load-source-price"]').trigger('click')
    if (edit === 'price') await wrapper.find('input[type="number"]').setValue('9')
    if (edit === 'model') wrapper.findComponent({ name: 'ModelTagInput' }).vm.$emit('update:models', ['new-model'])
    if (edit === 'mode') wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'image')
    if (edit === 'currency') await wrapper.find('[data-testid="source-currency"]').setValue('CNY')
    const afterEdit = emittedEntries(wrapper).length
    resolve({ found: true, input_price: 2e-6 })
    await flushPromises()
    expect(emittedEntries(wrapper)).toHaveLength(afterEdit)
    expect(wrapper.find('[data-testid="load-source-price"]').attributes('disabled')).toBeUndefined()
  })
})
