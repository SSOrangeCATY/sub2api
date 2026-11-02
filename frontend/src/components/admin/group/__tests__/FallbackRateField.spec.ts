import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import FallbackRateField from '../FallbackRateField.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('fallback fixed multiplier input', () => {
  it('clears to null and accepts a positive fixed multiplier', async () => {
    const wrapper = mount(FallbackRateField, { props: { id: 'rate', modelValue: 0.2 } })
    await wrapper.find('button').trigger('click')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
    await wrapper.setProps({ modelValue: null })
    expect(wrapper.find('button').exists()).toBe(false)
    expect(wrapper.find('input').element.value).toBe('')
    await wrapper.find('input').setValue('0.35')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([0.35])
    await wrapper.find('input').setValue('')
    expect(wrapper.emitted('update:modelValue')?.at(-1)).toEqual([null])
  })
})
