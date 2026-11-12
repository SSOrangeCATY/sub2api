import { describe, expect, it, vi } from 'vitest'
import type { UserRiskRestriction } from '@/api/admin/riskControl'
import { useUserRiskRelease } from '../useUserRiskRelease'

const row: UserRiskRestriction = {
  user_id: 17, email: 'synthetic@example.invalid', username: 'Example', base_status: 'disabled',
  policy_key: 'cyber_policy', scope: 'api', generation: 1, version: 8, window_hit_count: 3,
  decision_hit_count: 3, threshold: 3, admin_release: true, restricted_at: '2026-11-12T00:00:00Z', policy_revision: 2,
}
const result = { released: true, user_id: 17, policy_key: 'cyber_policy', generation: 2, version: 9, base_status: 'disabled' }
function setup(selected = row) {
  const api = { release: vi.fn(), read: vi.fn() }
  const id = vi.fn().mockReturnValueOnce('operation-one').mockReturnValueOnce('operation-two')
  const state = useUserRiskRelease(key => key, api, id)
  state.open(selected)
  state.reason.value = 'Reviewed the appeal'
  return { api, id, state }
}
describe('versioned user restriction release', () => {
  it('captures the selected version and reports the independent disabled base account', async () => {
    const selected = { ...row }
    const { api, state } = setup(selected)
    selected.version = 12
    api.release.mockResolvedValue(result)
    expect(await state.submit()).toEqual(result)
    expect(api.release).toHaveBeenCalledWith(17, {
      operation_id: 'operation-one', expected_version: 8, reason: 'Reviewed the appeal',
    })
    expect(state.target.value).toBeNull()
  })
  it('replays exactly the captured payload after an ambiguous failure', async () => {
    const { api, id, state } = setup()
    api.release.mockRejectedValueOnce(new Error('connection lost')).mockResolvedValueOnce(result)
    await state.submit()
    expect(state.phase.value).toBe('uncertain')
    expect(state.close()).toBe(false)
    expect(state.open({ ...row, user_id: 18 })).toBe(false)
    state.reason.value = 'Changed after sending'
    await state.submit()
    expect(api.release.mock.calls[1]).toEqual(api.release.mock.calls[0])
    expect(id).toHaveBeenCalledTimes(1)
  })
  it('retains the reason and requires a refreshed version after a conflict', async () => {
    const { api, state } = setup()
    api.release.mockRejectedValueOnce({ response: { status: 409 } }).mockResolvedValueOnce({ ...result, version: 11 })
    await state.submit()
    expect(await state.submit()).toBeNull()
    api.read.mockResolvedValue({ items: [{ ...row, version: 10 }] })
    expect(await state.reload()).toBe(true)
    expect(state.reason.value).toBe('Reviewed the appeal')
    expect(state.reviewChanged.value).toBe(true)
    await state.submit()
    expect(api.release.mock.calls[1][1]).toEqual({
      operation_id: 'operation-two', expected_version: 10, reason: 'Reviewed the appeal',
    })
  })
  it('rejects a stale conflict refresh without retiring the selected version', async () => {
    const { api, state } = setup()
    api.release.mockRejectedValue({ response: { status: 409 } })
    await state.submit()
    api.read.mockResolvedValue({ items: [{ ...row, version: 7 }] })
    expect(await state.reload()).toBe(false)
    expect(state.phase.value).toBe('conflict')
    expect(state.target.value?.version).toBe(8)
  })
  it('replays the captured operation if a success response has an inconsistent version', async () => {
    const { api, state } = setup()
    api.release.mockResolvedValueOnce({ ...result, version: 8 }).mockResolvedValueOnce(result)
    expect(await state.submit()).toBeNull()
    expect(state.phase.value).toBe('uncertain')
    expect(state.target.value?.version).toBe(8)
    await state.submit()
    expect(api.release.mock.calls[1]).toEqual(api.release.mock.calls[0])
  })
  it('does not replace an in-flight operation or issue concurrent submissions', async () => {
    const { api, state } = setup()
    let finish!: (value: typeof result) => void
    api.release.mockImplementation(() => new Promise(resolve => { finish = resolve }))
    const pending = state.submit()
    expect(state.close()).toBe(false)
    expect(await state.submit()).toBeNull()
    finish(result)
    await pending
    expect(api.release).toHaveBeenCalledTimes(1)
  })
})
