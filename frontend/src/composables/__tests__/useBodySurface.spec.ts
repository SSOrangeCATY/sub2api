import { afterEach, describe, expect, it } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { defineComponent, h, ref } from 'vue'
import { useBodySurface } from '../useBodySurface'

enableAutoUnmount(afterEach)

describe('body surface context', () => {
  it('retains portal styling until the last layout owner leaves', () => {
    const Surface = defineComponent({ setup() {
      useBodySurface(['pt-product-surface', 'pt-group-surface'])
      return () => h('div')
    } })
    const first = mount(Surface)
    const second = mount(Surface)
    first.unmount()
    expect(document.body.classList.contains('pt-group-surface')).toBe(true)
    second.unmount()
    expect(document.body.classList.contains('pt-group-surface')).toBe(false)
    expect(document.body.classList.contains('pt-product-surface')).toBe(false)
  })

  it('removes product overrides when switching to custom home HTML', () => {
    const defaultHome = ref(true)
    const Surface = defineComponent({ setup() {
      useBodySurface(['pt-product-surface'], defaultHome)
      return () => h('div')
    } })
    const wrapper = mount(Surface)
    expect(document.body.classList.contains('pt-product-surface')).toBe(true)
    defaultHome.value = false
    expect(document.body.classList.contains('pt-product-surface')).toBe(false)
    defaultHome.value = true
    expect(document.body.classList.contains('pt-product-surface')).toBe(true)
    wrapper.unmount()
    expect(document.body.classList.contains('pt-product-surface')).toBe(false)
  })
})
