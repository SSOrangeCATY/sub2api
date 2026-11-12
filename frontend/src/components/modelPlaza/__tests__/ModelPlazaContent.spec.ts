import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import ModelPlazaContent from '../ModelPlazaContent.vue'
import Select from '@/components/common/Select.vue'
import { group, model, pricing } from './fixtures'
const mounted: VueWrapper[] = []
beforeEach(() => setActivePinia(createPinia()))
afterEach(() => mounted.splice(0).forEach(w => w.unmount()))
function content(groups = [group(), group({ id: 2, name: 'CNY group', platform: 'deepseek', models: [model({ name: 'deepseek-test', platform: 'deepseek', pricing: pricing({ source_currency: 'CNY' }) })] })]) {
  const wrapper = mount(ModelPlazaContent, { props: { loading: false, response: { description: '', billing_identity: 'standard', groups } }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
  mounted.push(wrapper)
  return wrapper
}
describe('model price discovery', () => {
  it('searches by model and group and resets a no-match filter', async () => {
    const wrapper = content()
    await wrapper.find('input[type="search"]').setValue('deepseek CNY')
    expect(wrapper.findAll('.price-summary')).toHaveLength(1)
    expect(wrapper.find('.price-summary').text()).toContain('¥1.00')
    await wrapper.find('input[type="search"]').setValue('missing-model')
    expect(wrapper.findAll('.price-summary')).toHaveLength(0)
    expect(wrapper.find('.price-empty').text()).toContain('没有匹配')
    await wrapper.find('.price-active-filters .price-reset').trigger('click')
    expect(wrapper.findAll('.price-summary')).toHaveLength(2)
  })
  it('filters denomination without converting or relabeling prices', async () => {
    const wrapper = content()
    await wrapper.find('.price-more-filters').trigger('click')
    await wrapper.find('[data-testid="price-currency"]').setValue('CNY')
    expect(wrapper.findAll('.price-summary')).toHaveLength(1)
    expect(wrapper.find('.price-summary').text()).toContain('deepseek-test')
    expect(wrapper.find('.price-summary').text()).not.toContain('$')
  })
  it('sorts token price rows within their own currency and puts unknown prices last', async () => {
    const wrapper = content([group({ models: [model({ name: 'expensive', pricing: pricing({ output_price: 20e-6 }) }), model({ name: 'cheap', pricing: pricing({ output_price: 2e-6 }) }), model({ name: 'unknown', pricing: null })] })])
    await wrapper.find('.price-sort select').setValue('output')
    expect(wrapper.findAll('.price-model-cell strong').map(w => w.text())).toEqual(['cheap', 'expensive', 'unknown'])
  })
  it('paginates a large catalog and returns to the first page after a search', async () => {
    const wrapper = content([group({ models: Array.from({ length: 31 }, (_, index) => model({ name: 'model-' + String(index).padStart(2, '0') })) })])
    expect(wrapper.findAll('.price-summary')).toHaveLength(25)
    await wrapper.findAll('.price-pagination button')[1].trigger('click')
    expect(wrapper.findAll('.price-summary')).toHaveLength(6)
    await wrapper.find('input[type="search"]').setValue('model-00')
    expect(wrapper.findAll('.price-summary')).toHaveLength(1)
    expect(wrapper.find('.price-pagination').text()).toContain('第 1 / 1 页')
  })
  it('preserves the selected page across a background response refresh', async () => {
    const g = group({ models: Array.from({ length: 31 }, (_, index) => model({ name: 'model-' + String(index).padStart(2, '0') })) })
    const wrapper = content([g])
    await wrapper.findAll('.price-pagination button')[1].trigger('click')
    await wrapper.setProps({ response: { description: '', billing_identity: 'standard', groups: [{ ...g }] } })
    expect(wrapper.find('.price-pagination').text()).toContain('第 2 / 2 页')
  })
  it('keeps group switching visible and reveals only advanced conditions on demand', async () => {
    const wrapper = content()
    expect(wrapper.find('.price-group-field').exists()).toBe(true)
    expect(wrapper.find('[data-testid="price-currency"]').exists()).toBe(false)
    await wrapper.find('.price-more-filters').trigger('click')
    expect(wrapper.find('[data-testid="price-currency"]').exists()).toBe(true)
    expect(wrapper.find('.price-more-filters').attributes('aria-expanded')).toBe('true')
  })
  it('switches to a group catalog without retaining an incompatible platform', async () => {
    const wrapper = content()
    const platformButtons = wrapper.findAll('.price-platform-filter button')
    await platformButtons.find(button => button.text().includes('openai'))!.trigger('click')
    expect(wrapper.findAll('.price-summary')).toHaveLength(1)
    wrapper.findComponent(Select).vm.$emit('update:modelValue', '2')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.price-summary').text()).toContain('deepseek-test')
    expect(wrapper.find('.price-platform-filter button').attributes('aria-pressed')).toBe('true')
    await wrapper.find('.price-active-filters button').trigger('click')
    expect(wrapper.findAll('.price-summary')).toHaveLength(2)
  })
  it('keeps groups without configured models selectable and explains the empty state', async () => {
    const wrapper = content([group(), group({ id: 3, name: 'Empty group', models: [] })])
    const select = wrapper.findComponent(Select)
    expect(select.props('options')).toContainEqual({ value: '3', label: 'Empty group / 暂无模型' })
    select.vm.$emit('update:modelValue', '3')
    await wrapper.vm.$nextTick()
    expect(wrapper.find('.price-empty').text()).toContain('暂未配置')
    expect(wrapper.findAll('.price-summary')).toHaveLength(0)
  })
  it('clears one selected condition while preserving the other conditions', async () => {
    const wrapper = content()
    await wrapper.find('.price-more-filters').trigger('click')
    await wrapper.find('[data-testid="price-currency"]').setValue('CNY')
    await wrapper.find('input[type="search"]').setValue('deepseek')
    await wrapper.find('.price-active-filters button').trigger('click')
    expect(wrapper.find('input[type="search"]').element.value).toBe('')
    expect(wrapper.find('[data-testid="price-currency"]').element.value).toBe('CNY')
    expect(wrapper.findAll('.price-summary')).toHaveLength(1)
  })

})
