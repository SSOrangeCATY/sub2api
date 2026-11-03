import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, onUnmounted, ref } from 'vue'
import { createMemoryHistory, createRouter, RouterView } from 'vue-router'
import AppLayout from '../AppLayout.vue'
import { withWorkspaceLayout } from '@/router/workspaceLayout'

const chrome = vi.hoisted(() => ({ mounts: 0, unmounts: 0 }))
vi.mock('../WorkspaceLayout.vue', async () => {
  const { defineComponent, h, onMounted, onUnmounted, ref } = await import('vue')
  return { default: defineComponent({ setup(_, { slots }) {
    const collapsed = ref(false)
    onMounted(() => chrome.mounts++)
    onUnmounted(() => chrome.unmounts++)
    return () => h('section', { class: 'workspace' }, [
      h('aside', [h('button', { onClick: () => { collapsed.value = !collapsed.value } }, collapsed.value ? 'collapsed' : 'expanded')]),
      slots.default?.(),
    ])
  } }) }
})

enableAutoUnmount(afterEach)

async function harness() {
  chrome.mounts = chrome.unmounts = 0
  const unmounted = vi.fn()
  const page = (name: string) => defineComponent({ setup() {
    const value = ref('')
    onUnmounted(() => unmounted(name))
    return () => h(AppLayout, null, { default: () => [
      h('h1', name),
      h('input', { value: value.value, onInput: (event: Event) => { value.value = (event.target as HTMLInputElement).value } }),
    ] })
  } })
  const publicPage = defineComponent({ render: () => h('h1', 'public') })
  const router = createRouter({ history: createMemoryHistory(), routes: withWorkspaceLayout([
    { path: '/', redirect: '/home' },
    { path: '/home', component: publicPage },
    { path: '/keys', name: 'Keys', component: page('keys'), meta: { workspaceLayout: true, requiresAuth: true } },
    { path: '/usage', name: 'Usage', component: page('usage'), meta: { workspaceLayout: true, requiresAuth: true } },
    { path: '/admin/users', component: page('users'), meta: { workspaceLayout: true, requiresAuth: true, requiresAdmin: true } },
    { path: '/docs/batch-image', alias: '/batch-image', component: page('guide'), meta: { workspaceLayout: true } },
    { path: '/popup', component: publicPage },
    { path: '/:pathMatch(.*)*', component: publicPage },
  ]) })
  await router.push('/keys')
  await router.isReady()
  const wrapper = mount(RouterView, { global: { plugins: [router] } })
  await flushPromises()
  return { router, wrapper, unmounted }
}

describe('persistent workspace routing', () => {
  it('keeps one navigation instance and its state while unmounting each page', async () => {
    const { router, wrapper, unmounted } = await harness()
    const sidebar = wrapper.get('aside').element
    await wrapper.get('button').trigger('click')
    await wrapper.get('input').setValue('temporary page state')
    await router.push('/usage')
    await flushPromises()
    expect(wrapper.get('aside').element).toBe(sidebar)
    expect(wrapper.get('button').text()).toBe('collapsed')
    expect(wrapper.get('h1').text()).toBe('usage')
    expect(wrapper.findAll('.workspace')).toHaveLength(1)
    expect(unmounted).toHaveBeenCalledWith('keys')
    router.back()
    await flushPromises()
    expect(wrapper.get('h1').text()).toBe('keys')
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe('')
    expect(chrome.mounts).toBe(1)
  })

  it('removes navigation for standalone pages and builds it again on returning', async () => {
    const { router, wrapper } = await harness()
    await router.push('/popup')
    await flushPromises()
    expect(wrapper.find('aside').exists()).toBe(false)
    expect(chrome.unmounts).toBe(1)
    await router.push('/keys')
    await flushPromises()
    expect(chrome.mounts).toBe(2)
    await router.push('/')
    await flushPromises()
    expect(router.currentRoute.value.path).toBe('/home')
    expect(wrapper.find('aside').exists()).toBe(false)
    await router.push('/unknown')
    expect(router.currentRoute.value.matched).toHaveLength(1)
  })

  it('preserves aliases and leaf permission metadata', async () => {
    const { router } = await harness()
    await router.push('/batch-image')
    expect(router.currentRoute.value.matched).toHaveLength(2)
    expect(router.currentRoute.value.meta.workspaceLayout).toBe(true)
    await router.push('/admin/users')
    expect(router.currentRoute.value.meta.requiresAdmin).toBe(true)
    expect(router.currentRoute.value.meta.requiresAuth).toBe(true)
  })

  it('still supports independently rendered views with exactly one layout', () => {
    chrome.mounts = 0
    const wrapper = mount(AppLayout, { slots: { default: () => h(AppLayout, null, { default: () => h('h1', 'standalone view') }) } })
    expect(wrapper.findAll('.workspace')).toHaveLength(1)
    expect(wrapper.get('h1').text()).toBe('standalone view')
    expect(chrome.mounts).toBe(1)
  })
})
