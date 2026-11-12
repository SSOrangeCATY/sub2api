import { afterAll, afterEach, beforeAll, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount, type VueWrapper } from '@vue/test-utils'
import type { UserRiskListParams, UserRiskRestriction, UserRiskRestrictionPage } from '@/api/admin/riskControl'
import UserRiskRecords from '../UserRiskRecords.vue'

const api = vi.hoisted(() => ({ list: vi.fn(), history: vi.fn(), release: vi.fn() }))
vi.mock('@/api/admin/riskControl', () => ({
  riskControlAPI: { listUserRestrictions: api.list, listUserRestrictionActions: api.history, releaseUserRestriction: api.release },
}))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))
const temporary: UserRiskRestriction = {
  user_id: 17, email: 'synthetic@example.invalid', username: 'Example', base_status: 'active',
  policy_key: 'cyber_policy', scope: 'api', generation: 1, version: 8, window_hit_count: 2,
  decision_hit_count: 2, threshold: 2, admin_release: false, restricted_at: '2026-11-12T00:00:00Z',
  expires_at: '2026-11-13T00:00:00Z', policy_revision: 2,
}
function page(row = temporary, position = 1): UserRiskRestrictionPage {
  return { items: [row], page: position, page_size: 20, total: 30, pages: 2,
    has_more: position === 1, next_cursor: position === 1 ? 'cursor-for-20' : '',
    as_of: '2026-11-12T00:00:00Z', window_days: 7, policy_revision: 2 }
}
let wrapper: VueWrapper | null = null
async function clickButton(suffix: string) {
  const button = wrapper!.findAll('button').find(item => item.text().endsWith(suffix))
  expect(button).toBeDefined()
  await button!.trigger('click')
  await flushPromises()
}
beforeAll(() => {
  Object.defineProperty(HTMLDialogElement.prototype, 'showModal', { configurable: true, value: function (this: HTMLDialogElement) { this.setAttribute('open', '') } })
  Object.defineProperty(HTMLDialogElement.prototype, 'close', { configurable: true, value: function (this: HTMLDialogElement) { this.removeAttribute('open') } })
})
afterAll(() => {
  Reflect.deleteProperty(HTMLDialogElement.prototype, 'showModal')
  Reflect.deleteProperty(HTMLDialogElement.prototype, 'close')
})
beforeEach(() => {
  vi.resetAllMocks()
  let sequence = 0
  vi.stubGlobal('crypto', { randomUUID: () => 'test-operation-' + (++sequence) })
})
afterEach(() => { wrapper?.unmount(); wrapper = null; vi.unstubAllGlobals() })

describe('user risk records interactions', () => {
  it('keeps the applied page size while a different page size remains a draft', async () => {
    api.list.mockImplementation(async (params: UserRiskListParams) => page(temporary, params.cursor ? 2 : 1))
    wrapper = mount(UserRiskRecords, { props: { active: true, mode: 'restrictions' } })
    await flushPromises()
    await wrapper.findAll('select')[1].setValue('50')
    await clickButton('.next')
    expect(api.list.mock.calls.at(-1)?.[0]).toMatchObject({ page_size: 20, cursor: 'cursor-for-20' })
    await clickButton('.previous')
    expect(api.list.mock.calls.at(-1)?.[0]).toMatchObject({ page_size: 20, cursor: undefined })
    await wrapper.get('form.records-filters').trigger('submit')
    await flushPromises()
    expect(api.list.mock.calls.at(-1)?.[0]).toMatchObject({ page_size: 50, cursor: undefined })
  })
  it('shows an escalated restriction after conflict and waits for an explicit release', async () => {
    let current = { ...temporary }
    api.list.mockImplementation(async () => page(current))
    api.release.mockRejectedValueOnce({ response: { status: 409 } }).mockResolvedValueOnce({
      released: true, user_id: 17, policy_key: 'cyber_policy', generation: 2, version: 11, base_status: 'active',
    })
    wrapper = mount(UserRiskRecords, { props: { active: true, mode: 'restrictions' } })
    await flushPromises()
    await clickButton('.release')
    await wrapper.get('textarea').setValue('Reviewed the appeal')
    await wrapper.get('dialog form').trigger('submit')
    await flushPromises()
    current = { ...temporary, version: 10, admin_release: true, expires_at: undefined, threshold: 3, decision_hit_count: 3 }
    await clickButton('.reload')
    const dialog = wrapper.get('dialog')
    expect(dialog.text()).toContain('admin.riskControl.userRecords.manual')
    expect(dialog.text()).toContain('3 / 3')
    expect(dialog.text()).toContain('admin.riskControl.userRelease.reviewChanged')
    expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('Reviewed the appeal')
    expect(api.release).toHaveBeenCalledTimes(1)
    await wrapper.get('dialog form').trigger('submit')
    await flushPromises()
    expect(api.release).toHaveBeenCalledTimes(2)
    expect(api.release.mock.calls[1][1]).toMatchObject({ expected_version: 10, reason: 'Reviewed the appeal' })
  })
})
