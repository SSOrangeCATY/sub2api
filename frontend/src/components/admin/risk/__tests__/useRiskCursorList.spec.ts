import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import type { UserRiskActionPage, UserRiskListParams } from '@/api/admin/riskControl'
import { useRiskCursorList } from '../useRiskCursorList'

const page = (position = 1, total = 2, next = 'next-token'): UserRiskActionPage => ({
  items: [], page: position, total, page_size: 20, pages: 2,
  has_more: Boolean(next), next_cursor: next, as_of: '2026-11-12T00:00:00Z',
})

describe('risk cursor list controller', () => {
  let scope: EffectScope
  beforeEach(() => { scope = effectScope() })
  afterEach(() => { scope.stop() })
  const create = (
    fetch: (params: UserRiskListParams) => Promise<UserRiskActionPage>,
    filters: () => UserRiskListParams = () => ({}),
  ) => scope.run(() => useRiskCursorList(fetch, filters, () => 'Unable to load records'))!

  it('keeps the newest response when overlapping filters resolve out of order', async () => {
    let older!: (value: UserRiskActionPage) => void
    let newer!: (value: UserRiskActionPage) => void
    const fetch = vi.fn()
      .mockImplementationOnce(() => new Promise(resolve => { older = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { newer = resolve }))
    let search = 'old'
    const list = create(fetch, () => ({ search }))
    const first = list.refresh()
    search = 'new'
    const second = list.refresh()
    newer(page(1, 9))
    expect(await second).toBe(true)
    older(page(1, 3))
    expect(await first).toBe(false)
    expect(list.data.value?.total).toBe(9)
    expect(list.loading.value).toBe(false)
  })

  it('retains records and the cursor path when loading the next page fails', async () => {
    const original = page()
    const fetch = vi.fn().mockResolvedValueOnce(original).mockRejectedValueOnce(new Error('network failed'))
    const list = create(fetch)
    expect(await list.refresh()).toBe(true)
    expect(await list.next()).toBe(false)
    expect(list.data.value).toBe(original)
    expect(list.cursors.value).toEqual([''])
    expect(list.error.value).not.toBe('')
    expect(list.canNext.value).toBe(true)
  })

  it('uses server cursors for next and previous, and resets traversal on refresh', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(page())
      .mockResolvedValueOnce(page(2, 2, ''))
      .mockResolvedValueOnce(page())
      .mockResolvedValueOnce(page(1, 5))
    let search = 'first'
    const list = create(fetch, () => ({ search, page_size: 20 }))
    await list.refresh()
    await list.next()
    expect(fetch.mock.calls[1][0].cursor).toBe('next-token')
    expect(list.cursors.value).toEqual(['', 'next-token'])
    await list.previous()
    expect(fetch.mock.calls[2][0].cursor).toBeUndefined()
    search = 'changed'
    await list.refresh()
    expect(fetch.mock.calls[3][0]).toEqual({ search: 'changed', page_size: 20, cursor: undefined })
    expect(list.cursors.value).toEqual([''])
  })

  it('marks changed live counts until the user refreshes', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(page())
      .mockResolvedValueOnce(page(2, 3, ''))
      .mockResolvedValueOnce(page(1, 3))
    const list = create(fetch)
    await list.refresh()
    await list.next()
    expect(list.changedDuringTraversal.value).toBe(true)
    await list.refresh()
    expect(list.changedDuringTraversal.value).toBe(false)
  })

  it('discards a late response after its owning component scope is disposed', async () => {
    let finish!: (value: UserRiskActionPage) => void
    const fetch = vi.fn(() => new Promise<UserRiskActionPage>(resolve => { finish = resolve }))
    const list = create(fetch)
    const pending = list.refresh()
    scope.stop()
    finish(page())
    expect(await pending).toBe(false)
    expect(list.data.value).toBeNull()
  })
})
