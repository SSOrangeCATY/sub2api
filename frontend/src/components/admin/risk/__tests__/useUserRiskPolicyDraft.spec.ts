import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { effectScope, type EffectScope } from 'vue'
import type { ContentModerationConfig } from '@/api/admin/riskControl'
import { cloneUserRestrictionPolicy } from '../userRestrictionPolicy'
import { useUserRiskPolicyDraft } from '../useUserRiskPolicyDraft'

const config = (revision = 3, days = 7): ContentModerationConfig => ({
  revision,
  cyber_policy_user_restriction: {
    enabled: true, window_days: days,
    actions: [{ threshold: 2, duration_hours: 24, admin_release: false },
      { threshold: 3, duration_hours: 0, admin_release: true }],
  },
} as ContentModerationConfig)

const createAPI = () => ({
  getConfig: vi.fn(async () => config()),
  updateConfig: vi.fn(async () => config(4)),
})
type Controller = ReturnType<typeof useUserRiskPolicyDraft>
function edit(controller: Controller, days: number) {
  const draft = cloneUserRestrictionPolicy(controller.draft.value!)
  draft.window_days = days
  return controller.updateDraft(draft)
}

describe('versioned user restriction policy drafts', () => {
  let scope: EffectScope
  beforeEach(() => { scope = effectScope() })
  afterEach(() => { scope.stop() })
  const create = (api: ReturnType<typeof createAPI>) =>
    scope.run(() => useUserRiskPolicyDraft(api))!

  it('saves only the policy and its read revision without mutating the source configuration', async () => {
    const api = createAPI()
    api.updateConfig.mockResolvedValueOnce(config(4, 14))
    const controller = create(api)
    const original = config()
    expect(controller.acceptConfig(original)).toBe(true)
    edit(controller, 14)
    const result = await controller.save()
    expect(api.updateConfig).toHaveBeenCalledWith({
      expected_revision: 3,
      cyber_policy_user_restriction: config(4, 14).cyber_policy_user_restriction,
    })
    expect(result?.previousRevision).toBe(3)
    expect(controller.revision.value).toBe(4)
    expect(controller.dirty.value).toBe(false)
    expect(original.cyber_policy_user_restriction?.window_days).toBe(7)
  })

  it('preserves a conflicted draft and explicitly reloads a new baseline without saving it', async () => {
    const api = createAPI()
    api.updateConfig.mockRejectedValueOnce({ response: { status: 409 } })
    api.getConfig.mockResolvedValueOnce(config(4, 10))
    const controller = create(api)
    controller.acceptConfig(config())
    edit(controller, 14)
    expect(await controller.save()).toBeNull()
    expect(controller.conflict.value).toBe(true)
    expect(controller.revision.value).toBe(3)
    expect(controller.draft.value?.window_days).toBe(14)
    expect(await controller.reload()).toBe(true)
    expect(controller.revision.value).toBe(4)
    expect(controller.draft.value?.window_days).toBe(14)
    expect(controller.dirty.value).toBe(true)
    expect(controller.conflict.value).toBe(false)
    expect(api.updateConfig).toHaveBeenCalledTimes(1)
    controller.reset()
    expect(controller.draft.value?.window_days).toBe(10)
    expect(controller.dirty.value).toBe(false)
  })

  it('does not silently rebase a dirty draft when the parent receives another version', async () => {
    const api = createAPI()
    const controller = create(api)
    controller.acceptConfig(config())
    edit(controller, 14)
    controller.acceptConfig(config(4, 9))
    expect(controller.revision.value).toBe(3)
    expect(controller.draft.value?.window_days).toBe(14)
    expect(controller.conflict.value).toBe(true)
    expect(await controller.save()).toBeNull()
    expect(api.updateConfig).not.toHaveBeenCalled()
    controller.reset()
    expect(controller.revision.value).toBe(4)
    expect(controller.draft.value?.window_days).toBe(9)
  })

  it('never regresses a newer parent snapshot when an older successful save returns late', async () => {
    const api = createAPI()
    let finish!: (value: ContentModerationConfig) => void
    api.updateConfig.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const controller = create(api)
    controller.acceptConfig(config())
    edit(controller, 14)
    const pending = controller.save()
    expect(controller.saving.value).toBe(true)
    expect(edit(controller, 21)).toBe(false)
    controller.acceptConfig(config(5, 20))
    finish(config(4, 14))
    expect(await pending).toBeNull()
    expect(controller.serverConfig.value?.revision).toBe(5)
    expect(controller.draft.value?.window_days).toBe(14)
    expect(controller.conflict.value).toBe(true)
    expect(controller.saving.value).toBe(false)
    controller.reset()
    expect(controller.revision.value).toBe(5)
    expect(controller.draft.value?.window_days).toBe(20)
  })

  it('does not invent a policy for missing configuration or send an invalid draft', async () => {
    const api = createAPI()
    const controller = create(api)
    expect(controller.acceptConfig({ revision: 3 } as ContentModerationConfig)).toBe(false)
    expect(controller.available.value).toBe(false)
    expect(controller.draft.value).toBeNull()
    expect(await controller.save()).toBeNull()
    controller.acceptConfig(config())
    edit(controller, 0)
    expect(await controller.save()).toBeNull()
    expect(api.updateConfig).not.toHaveBeenCalled()
  })

  it('retains the baseline and edits when rereading fails', async () => {
    const api = createAPI()
    api.getConfig.mockRejectedValueOnce(new Error('offline'))
    const controller = create(api)
    controller.acceptConfig(config())
    edit(controller, 14)
    expect(await controller.reload()).toBe(false)
    expect(controller.errorKind.value).toBe('load')
    expect(controller.revision.value).toBe(3)
    expect(controller.draft.value?.window_days).toBe(14)
    expect(controller.dirty.value).toBe(true)
  })

  it('preserves edits made while the latest policy is being read', async () => {
    const api = createAPI()
    let finish!: (value: ContentModerationConfig) => void
    api.getConfig.mockImplementationOnce(() => new Promise(resolve => { finish = resolve }))
    const controller = create(api)
    controller.acceptConfig(config())
    edit(controller, 14)
    const pending = controller.reload()
    expect(edit(controller, 21)).toBe(true)
    finish(config(4, 10))
    expect(await pending).toBe(true)
    expect(controller.revision.value).toBe(4)
    expect(controller.draft.value?.window_days).toBe(21)
    expect(controller.dirty.value).toBe(true)
  })
})
