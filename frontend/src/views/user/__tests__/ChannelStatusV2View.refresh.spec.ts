import { flushPromises, shallowMount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import ChannelStatusV2View from '../ChannelStatusV2View.vue'

const api = vi.hoisted(() => ({
  getDimensions: vi.fn(), getSnapshot: vi.fn(), getMatrix: vi.fn(),
  getModels: vi.fn(), getErrors: vi.fn(), getUsers: vi.fn(),
}))
vi.mock('@/api/channelMonitorV2', () => api)
vi.mock('@/stores/auth', () => ({ useAuthStore: () => ({ isAdmin: false }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn() }) }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ replace: vi.fn() }) }))
vi.mock('vue-i18n', async (original) => ({
  ...await original<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, te: () => false, locale: ref('zh-CN') }),
}))

const metrics = {
  success_requests: 10, error_requests: 0, request_count: 10, token_count: 100,
  rpm: 1, tpm: 10, error_rate: 0, cache_rate: 0.5, cache_rate_numerator: 50, cache_rate_denominator: 100,
  ttft: { sample_count: 10, p50_ms: 100, p95_ms: 200, avg_ms: 150 },
  duration: { sample_count: 10, p50_ms: 200, p95_ms: 400, avg_ms: 300 },
}
const health = { overall: 'healthy', error_rate: 'healthy', ttft: 'healthy', minimum_sample: 1 }
const coverage = {
  requested_start: '2026-11-03T09:00:00Z', coverage_start: '2026-11-03T09:00:00Z',
  data_through: '2026-11-03T10:00:00Z', computed_at: '2026-11-03T10:00:00Z',
  aggregation_lag_seconds: 0, coverage_complete: true, bucket_seconds: 60,
}
let wrapper: ReturnType<typeof shallowMount>
beforeEach(() => {
  vi.useFakeTimers(); vi.clearAllMocks()
  api.getDimensions.mockResolvedValue({ platforms: [], groups: [], models: [] })
  api.getSnapshot.mockResolvedValue({ config: { enabled: true, refresh_interval_seconds: 60 }, metrics, health, coverage, trend: [] })
  api.getMatrix.mockResolvedValue({ coverage, group_by: 'platform_group', items: [] })
  api.getModels.mockResolvedValue({ items: [{ platform: 'openai', model: 'existing-model', metrics, health }] })
  api.getErrors.mockResolvedValue({ items: [] }); api.getUsers.mockResolvedValue({ items: [] })
})
afterEach(() => { wrapper?.unmount(); vi.restoreAllMocks(); vi.useRealTimers() })
function render() {
  wrapper = shallowMount(ChannelStatusV2View, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
  } } })
}
describe('channel monitor background refresh', () => {
  it('retains visible model rows without a loading placeholder during automatic refresh', async () => {
    render(); await flushPromises()
    const row = wrapper.findAll('tr').find(item => item.text().includes('existing-model'))!.element
    let finish!: (value: object) => void
    api.getModels.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    await vi.advanceTimersByTimeAsync(60000); await flushPromises()
    expect(api.getModels).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).not.toContain('common.loading')
    expect(wrapper.findAll('tr').find(item => item.text().includes('existing-model'))!.element).toBe(row)
    finish({ items: [{ platform: 'openai', model: 'updated-model', metrics, health }] }); await flushPromises()
    expect(wrapper.text()).toContain('updated-model')
  })
  it('does not poll in a hidden tab or refetch just because the window gains focus', async () => {
    render(); await flushPromises()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(true)
    await vi.advanceTimersByTimeAsync(180000); await flushPromises()
    expect(api.getSnapshot).toHaveBeenCalledOnce()
    vi.spyOn(document, 'hidden', 'get').mockReturnValue(false)
    window.dispatchEvent(new Event('focus')); document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(api.getSnapshot).toHaveBeenCalledOnce()
    await vi.advanceTimersByTimeAsync(60000); await flushPromises()
    expect(api.getSnapshot).toHaveBeenCalledTimes(2)
  })
})
