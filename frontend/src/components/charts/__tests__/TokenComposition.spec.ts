import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import TokenComposition from '../TokenComposition.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('Token composition data', () => {
  it('reports the prompt cache hit rate independently of output volume, including accessible text', () => {
    const wrapper = mount(TokenComposition, { props: { input: 60, output: 500, creation: 10, read: 30 } })
    expect(wrapper.get('.sr-only').text()).toBe('usage.cacheHitRate: 30.0%')
    expect(wrapper.findAll('.pt-mix-legend li').map(row => row.get('b').text())).toEqual(['60', '500', '10', '30'])
    wrapper.unmount()
  })

  it('does not invent a rate for zero traffic or non-finite input', () => {
    const wrapper = mount(TokenComposition, { props: { input: NaN, output: 0, creation: -1, read: Infinity } })
    expect(wrapper.get('.sr-only').text()).toBe('usage.cacheHitRate: —')
    expect(wrapper.html()).not.toMatch(/NaN|Infinity/)
    wrapper.unmount()
  })

  it('updates the displayed data after refreshed stats arrive', async () => {
    const wrapper = mount(TokenComposition, { props: { input: 75, output: 100, creation: 0, read: 25 } })
    await wrapper.setProps({ input: 20, creation: 10, read: 70 })
    expect(wrapper.get('.sr-only').text()).toBe('usage.cacheHitRate: 70.0%')
    expect(wrapper.findAll('.pt-mix-legend li')[3].get('b').text()).toBe('70')
    wrapper.unmount()
  })
})
