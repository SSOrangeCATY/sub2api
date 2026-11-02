import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import GroupBadge from '../GroupBadge.vue'
import GroupOptionItem from '../GroupOptionItem.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ cachedPublicSettings: null }) }))

for (const [name, component] of [['badge', GroupBadge], ['option', GroupOptionItem]] as const) {
  describe('fallback rate ' + name, () => {
    it('follows route changes, prioritizes fixed rate, and hides stale entry and personal prices', async () => {
      const wrapper = mount(component, { props: {
        name: 'Fallback', platform: 'openai', isFallback: true, rateMultiplier: 0.075,
        userRateMultiplier: 0.1, peakRateEnabled: true, peakStart: '09:00', peakEnd: '18:00', peakRateMultiplier: 2
      }, global: { stubs: { PlatformIcon: true } } })
      expect(wrapper.text()).toContain('admin.groups.monitorFallback.followTarget')
      expect(wrapper.text()).not.toContain('0.075x')
      expect(wrapper.text()).not.toContain('0.1x')
      expect(wrapper.find('.pt-group-peak').exists()).toBe(false)
      await wrapper.setProps({ fallbackEffectiveRateMultiplier: 0.35 })
      expect(wrapper.text()).toContain('0.35x')
      await wrapper.setProps({ fallbackEffectiveRateMultiplier: 0.3 })
      expect(wrapper.text()).toContain('0.3x')
      expect(wrapper.text()).not.toContain('0.35x')
      await wrapper.setProps({ fallbackRateMultiplier: 0.2 })
      expect(wrapper.text()).toContain('0.2x')
      expect(wrapper.text()).not.toContain('0.3x')
      await wrapper.setProps({ fallbackRateMultiplier: null })
      expect(wrapper.text()).toContain('0.3x')
      await wrapper.setProps({ purchasePricing: true })
      expect(wrapper.text()).toContain('modelPlaza.purchaseDynamic')
      expect(wrapper.text()).not.toContain('0.3x')
    })
  })
}
