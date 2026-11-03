import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import WelfareView from '../WelfareView.vue'
const api = vi.hoisted(() => ({ binding: vi.fn(), center: vi.fn(), action: vi.fn() }))
const auth = vi.hoisted(() => ({ user: { id: 7 }, refreshUser: vi.fn() }))
vi.mock('@/api/qqWelfare', () => ({ qqWelfare: api }))
vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key, locale: ref('zh-CN') }) }))
const snapshot = { points: 2, checked_in: false, products: [{ id: 1, version: 1, title: 'Reward', points_cost: 1, stock: 2, balance_value: 1 }], orders: [] }
const render = () => mount(WelfareView, { global: { stubs: {
  AppLayout: { template: '<div><slot /></div>' },
  ProfileQQWelfareCard: { template: '<button data-verify @click="$emit(\'verified\')">verify</button>' },
  ConfirmDialog: { props: ['show'], template: '<button v-if="show" data-confirm @click="$emit(\'confirm\')">confirm</button>' }
} } })
describe('Website welfare center', () => {
  beforeEach(() => {
    vi.clearAllMocks(); sessionStorage.clear()
    api.binding.mockResolvedValue({ verified: true, ready: true })
    api.center.mockResolvedValue({ snapshot: structuredClone(snapshot) })
    api.action.mockResolvedValue({ message: 'success' })
    auth.refreshUser.mockResolvedValue({})
  })
  it('gates all rewards until first group verification, then unlocks automatically', async () => {
    api.binding.mockResolvedValueOnce({ verified: false, ready: true })
    const wrapper = render(); await flushPromises()
    expect(api.center).not.toHaveBeenCalled(); expect(wrapper.find('#welfare-code').exists()).toBe(false)
    await wrapper.find('[data-verify]').trigger('click'); await flushPromises()
    expect(api.center).toHaveBeenCalledOnce(); expect(wrapper.find('#welfare-code').exists()).toBe(true)
    wrapper.unmount()
  })
  it('refreshes the shared daily check-in state after signing in', async () => {
    const wrapper = render(); await flushPromises()
    api.center.mockResolvedValue({ snapshot: { ...snapshot, points: 3, checked_in: true } })
    await wrapper.findAll('button').find(b => b.text() === 'qqWelfare.checkIn')!.trigger('click'); await flushPromises()
    expect(api.action).toHaveBeenCalledWith('checkin', '', '', 0, 0)
    expect(wrapper.findAll('button').find(b => b.text() === 'qqWelfare.checkedIn')!.attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
  it('requires confirmation and preserves an unresolved purchase ID across remounts', async () => {
    let wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'qqWelfare.exchange')!.trigger('click')
    expect(api.action).not.toHaveBeenCalled()
    api.action.mockRejectedValueOnce(new Error('timeout'))
    await wrapper.find('[data-confirm]').trigger('click'); await flushPromises()
    const id = api.action.mock.calls[0]![1]; expect(id).toMatch(/^[a-f0-9-]{36}$/)
    wrapper.unmount(); wrapper = render(); await flushPromises()
    await wrapper.findAll('button').find(b => b.text() === 'qqWelfare.exchange')!.trigger('click')
    await wrapper.find('[data-confirm]').trigger('click'); await flushPromises()
    expect(api.action.mock.calls[1]![1]).toBe(id); expect(auth.refreshUser).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
  it('normalizes shared codes and refreshes credited website balance', async () => {
    const wrapper = render(); await flushPromises()
    await wrapper.find('#welfare-code').setValue(' welcome ')
    await wrapper.find('form').trigger('submit'); await flushPromises()
    expect(api.action).toHaveBeenCalledWith('claim', expect.any(String), 'WELCOME', 0, 0)
    expect(auth.refreshUser).toHaveBeenCalledOnce(); expect((wrapper.find('#welfare-code').element as HTMLInputElement).value).toBe('')
    wrapper.unmount()
  })
  it('keeps a successful receipt and removes stale buying controls when refresh fails', async () => {
    const wrapper = render(); await flushPromises()
    api.center.mockRejectedValueOnce(new Error('refresh unavailable'))
    await wrapper.findAll('button').find(b => b.text() === 'qqWelfare.exchange')!.trigger('click')
    await wrapper.find('[data-confirm]').trigger('click'); await flushPromises()
    expect(wrapper.text()).toContain('success')
    expect(wrapper.text()).toContain('qqWelfare.refreshFailed')
    expect(wrapper.findAll('button').some(b => b.text() === 'qqWelfare.exchange')).toBe(false)
    expect(api.action).toHaveBeenCalledWith('exchange', expect.any(String), '', 1, 1)
    expect(api.action).toHaveBeenCalledOnce()
    wrapper.unmount()
  })
})
