import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import GroupPricePreview from '../GroupPricePreview.vue'
import { group, model, pricing } from './fixtures'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
const { getPrices } = vi.hoisted(() => ({ getPrices: vi.fn() }))
vi.mock('@/api/modelPlaza', () => ({ getGroupModelPrices: getPrices }))
let wrapper: VueWrapper | undefined
beforeEach(() => { setActivePinia(createPinia()); getPrices.mockReset() })
afterEach(() => wrapper?.unmount())
function preview() {
  wrapper = mount(GroupPricePreview, { props: { groups: [{ id: 1, name: 'Group A' }, { id: 2, name: 'Group B' }] }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })], stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show" class="test-dialog"><slot /></div>' } } } })
  return wrapper
}
describe('shared effective price preview', () => {
  it('loads numerical procurement prices only after opening', async () => {
    getPrices.mockResolvedValue({ billing_identity: 'purchase', description: '', groups: [group({ models: [model({ price_quote: { status: 'quoted', variants: [{ pricing: pricing({ input_price: .3e-6 }) }] } })] })] })
    const wrapper = preview()
    expect(getPrices).not.toHaveBeenCalled()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    expect(getPrices).toHaveBeenCalledWith(1, expect.any(AbortSignal))
    expect(wrapper.find('.price-summary').text()).toContain('$0.30')
  })
  it('ignores a late response for the previously selected group', async () => {
    let resolveFirst!: (value: ModelPlazaResponse) => void
    getPrices.mockImplementationOnce(() => new Promise<ModelPlazaResponse>(resolve => { resolveFirst = resolve }))
    getPrices.mockResolvedValueOnce({ billing_identity: 'standard', description: '', groups: [group({ id: 2, name: 'Group B', models: [model({ name: 'new-group-model' })] })] })
    const wrapper = preview()
    await wrapper.find('button').trigger('click')
    await wrapper.find('select').setValue('2')
    await flushPromises()
    resolveFirst({ billing_identity: 'standard', description: '', groups: [group({ models: [model({ name: 'stale-group-model' })] })] })
    await flushPromises()
    expect(wrapper.text()).toContain('new-group-model')
    expect(wrapper.text()).not.toContain('stale-group-model')
  })
  it('shows a retryable failure instead of a previous group price', async () => {
    getPrices.mockResolvedValueOnce({ billing_identity: 'standard', description: '', groups: [group()] })
    getPrices.mockRejectedValueOnce(new Error('unavailable'))
    const wrapper = preview()
    await wrapper.find('button').trigger('click')
    await flushPromises()
    await wrapper.find('select').setValue('2')
    await flushPromises()
    expect(wrapper.find('.price-summary').exists()).toBe(false)
    expect(wrapper.find('[role="alert"]').text()).toContain('重新加载')
  })
})
