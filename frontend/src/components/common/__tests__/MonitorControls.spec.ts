import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import Toggle from '../Toggle.vue'
import AutoRefreshButton from '../AutoRefreshButton.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('monitor controls', () => {
  it('keeps switch state accessible and emits only when enabled', async () => {
    const wrapper = mount(Toggle, { props: { modelValue: false } })
    expect(wrapper.attributes('aria-checked')).toBe('false')
    await wrapper.trigger('click')
    expect(wrapper.emitted('update:modelValue')).toEqual([[true]])
    await wrapper.setProps({ modelValue: true })
    expect(wrapper.attributes('aria-checked')).toBe('true')
    wrapper.unmount()
    const disabled = mount(Toggle, { props: { modelValue: true }, attrs: { disabled: true } })
    await disabled.trigger('click')
    expect(disabled.emitted('update:modelValue')).toBeUndefined()
  })

  it('exposes selected interval, emits updates, and closes with Escape', async () => {
    const wrapper = mount(AutoRefreshButton, { props: { enabled: true, intervalSeconds: 60, countdown: 20, intervals: [30, 60, 120] } })
    const trigger = wrapper.get('.auto-refresh__trigger')
    expect(trigger.attributes('type')).toBe('button')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    await trigger.trigger('click')
    expect(trigger.attributes('aria-expanded')).toBe('true')
    const choices = wrapper.findAll('.auto-refresh__menu button')
    expect(choices.map(button => button.attributes('aria-pressed'))).toEqual(['true', 'false', 'true', 'false'])
    await choices[1].trigger('click')
    expect(wrapper.emitted('update:interval')).toEqual([[30]])
    await wrapper.trigger('keydown', { key: 'Escape' })
    expect(wrapper.find('.auto-refresh__menu').exists()).toBe(false)
    wrapper.unmount()
  })
})
