import { afterEach, describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import zh from '@/i18n/locales/zh'
import PriceUnitHelp from '../PriceUnitHelp.vue'
let wrapper: VueWrapper | undefined
afterEach(() => wrapper?.unmount())
describe('currency help accessibility', () => {
  it('keeps the first focus plus touch/click open and closes with Escape', async () => {
    wrapper = mount(PriceUnitHelp, { props: { currencies: ['CNY', 'USD'], purchase: true }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
    const button = wrapper.find('button')
    await button.trigger('focus')
    await button.trigger('click')
    expect(button.attributes('aria-expanded')).toBe('true')
    expect(wrapper.find('[role="tooltip"]').text()).toContain('¥0.20')
    expect(wrapper.find('[role="tooltip"]').text()).toContain('1:1')
    expect(wrapper.find('[role="tooltip"]').text()).toContain('不再叠加分组销售倍率')
    await button.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
  })
  it('opens by hover without repeating a help icon for each amount', async () => {
    wrapper = mount(PriceUnitHelp, { props: { currencies: ['CNY', 'USD'] }, global: { plugins: [createI18n({ legacy: false, locale: 'zh', messages: { zh } })] } })
    expect(wrapper.findAll('button')).toHaveLength(1)
    await wrapper.find('.price-unit-help').trigger('mouseenter')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(true)
    await wrapper.find('.price-unit-help').trigger('mouseleave')
    expect(wrapper.find('[role="tooltip"]').exists()).toBe(false)
  })
})
