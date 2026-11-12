import { computed, onScopeDispose, ref, shallowRef } from 'vue'
import { riskControlAPI, type ContentModerationConfig, type CyberPolicyUserRestrictionPolicy } from '@/api/admin/riskControl'
import { cloneUserRestrictionPolicy, validateUserRestrictionPolicy, type UserRestrictionPolicyDraft } from './userRestrictionPolicy'

type PolicyAPI = Pick<typeof riskControlAPI, 'getConfig' | 'updateConfig'>
export type UserRiskPolicyErrorKind = 'load' | 'save' | 'version' | 'unsupported' | null

function parsePolicy(config: ContentModerationConfig) {
  const policy = config.cyber_policy_user_restriction
  if (!Number.isSafeInteger(config.revision) || !(Number(config.revision) > 0) ||
      !policy || typeof policy.enabled !== 'boolean' || typeof policy.window_days !== 'number' ||
      !Array.isArray(policy.actions) || !policy.actions.every(rule => rule &&
        typeof rule.threshold === 'number' && typeof rule.duration_hours === 'number' &&
        typeof rule.admin_release === 'boolean') ||
      validateUserRestrictionPolicy(policy) !== null) return null
  return { revision: Number(config.revision), policy: cloneUserRestrictionPolicy(policy) }
}

export function useUserRiskPolicyDraft(api: PolicyAPI = riskControlAPI) {
  const serverConfig = shallowRef<ContentModerationConfig | null>(null)
  const draft = shallowRef<UserRestrictionPolicyDraft | null>(null)
  const baseline = shallowRef<UserRestrictionPolicyDraft | null>(null)
  const revision = ref<number | null>(null)
  const available = ref(false)
  const saving = ref(false)
  const reading = ref(false)
  const conflict = ref(false)
  const errorKind = ref<UserRiskPolicyErrorKind>(null)
  const cause = shallowRef<unknown>(null)
  const dirty = computed(() => Boolean(draft.value && baseline.value &&
    JSON.stringify(draft.value) !== JSON.stringify(baseline.value)))
  let sequence = 0
  let disposed = false

  function clearError() {
    errorKind.value = null
    cause.value = null
  }

  function acknowledge(config: ContentModerationConfig, preserveDraft: boolean) {
    const parsed = parsePolicy(config)
    if (!parsed) {
      available.value = false
      errorKind.value = 'unsupported'
      return false
    }
    const retained = preserveDraft && draft.value ? cloneUserRestrictionPolicy(draft.value) : null
    serverConfig.value = config
    baseline.value = cloneUserRestrictionPolicy(parsed.policy)
    draft.value = retained ?? cloneUserRestrictionPolicy(parsed.policy)
    revision.value = parsed.revision
    available.value = true
    conflict.value = false
    clearError()
    return true
  }

  // Automatic parent updates cannot silently rebase a dirty draft.
  function acceptConfig(config: ContentModerationConfig) {
    const parsed = parsePolicy(config)
    if (!parsed) {
      available.value = false
      errorKind.value = 'unsupported'
      return false
    }
    if (Number(serverConfig.value?.revision ?? 0) > parsed.revision) return false
    serverConfig.value = config
    available.value = true
    if (!dirty.value) return acknowledge(config, false)
    if (revision.value !== parsed.revision ||
        JSON.stringify(baseline.value) !== JSON.stringify(parsed.policy)) {
      conflict.value = true
      errorKind.value = 'version'
    }
    return true
  }

  function updateDraft(value: UserRestrictionPolicyDraft) {
    if (saving.value || disposed) return false
    draft.value = cloneUserRestrictionPolicy(value)
    return true
  }

  function reset() {
    if (saving.value || disposed || !serverConfig.value) return false
    return acknowledge(serverConfig.value, false)
  }

  async function reload() {
    if (saving.value || disposed) return false
    const ticket = ++sequence
    reading.value = true
    clearError()
    try {
      const config = await api.getConfig()
      if (disposed || ticket !== sequence) return false
      if (Number(serverConfig.value?.revision ?? 0) > Number(config.revision ?? 0)) return false
      // Capture the current draft after the await, preserving edits made while reading.
      return acknowledge(config, true)
    } catch (error: unknown) {
      if (!disposed && ticket === sequence) { cause.value = error; errorKind.value = 'load' }
      return false
    } finally {
      if (!disposed && ticket === sequence) reading.value = false
    }
  }

  async function save() {
    if (disposed || saving.value || reading.value || !available.value || conflict.value ||
        !dirty.value || !draft.value || revision.value === null ||
        validateUserRestrictionPolicy(draft.value) !== null) return null
    const previousRevision = revision.value
    const captured = cloneUserRestrictionPolicy(draft.value)
    const policy: CyberPolicyUserRestrictionPolicy = {
      enabled: captured.enabled, window_days: Number(captured.window_days),
      actions: captured.actions.map(rule => ({
        threshold: Number(rule.threshold), duration_hours: Number(rule.duration_hours),
        admin_release: rule.admin_release,
      })),
    }
    const ticket = ++sequence
    saving.value = true
    clearError()
    try {
      const config = await api.updateConfig({
        expected_revision: previousRevision, cyber_policy_user_restriction: policy,
      })
      if (disposed || ticket !== sequence) return null
      if (Number(serverConfig.value?.revision ?? 0) > Number(config.revision ?? 0)) {
        conflict.value = true
        errorKind.value = 'version'
        return null
      }
      if (!acknowledge(config, false)) return null
      return { config, previousRevision }
    } catch (error: unknown) {
      if (!disposed && ticket === sequence) {
        cause.value = error
        const status = (error as { response?: { status?: number } })?.response?.status
        conflict.value = status === 409
        errorKind.value = conflict.value ? 'version' : 'save'
      }
      return null
    } finally {
      if (!disposed && ticket === sequence) saving.value = false
    }
  }

  onScopeDispose(() => {
    disposed = true
    sequence++
    saving.value = false
    reading.value = false
  })
  return { serverConfig, draft, revision, available, saving, reading, conflict, errorKind, cause, dirty,
    acceptConfig, updateDraft, reset, reload, save }
}
