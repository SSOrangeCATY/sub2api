import { computed, ref, shallowRef } from 'vue'
import { riskControlAPI, type UserRiskReleasePayload, type UserRiskReleaseResult, type UserRiskRestriction } from '@/api/admin/riskControl'
import { extractApiErrorMessage } from '@/utils/apiError'

type Phase = 'idle' | 'submitting' | 'uncertain' | 'conflict' | 'refreshing' | 'failed'
interface ReleaseAPI {
  release: typeof riskControlAPI.releaseUserRestriction
  read: typeof riskControlAPI.listUserRestrictions
}

// Capture a restriction version and an immutable operation before sending.
// An ambiguous outcome can only replay that exact operation, never a new one.
export function useUserRiskRelease(
  message: (key: string) => string,
  api: ReleaseAPI = { release: riskControlAPI.releaseUserRestriction, read: riskControlAPI.listUserRestrictions },
  operationID: () => string = () => crypto.randomUUID(),
) {
  const target = shallowRef<UserRiskRestriction | null>(null)
  const reason = ref('')
  const phase = ref<Phase>('idle')
  const error = ref('')
  const reviewChanged = ref(false)
  let submitted: Readonly<UserRiskReleasePayload> | null = null
  const busy = computed(() => phase.value === 'submitting' || phase.value === 'refreshing')
  const locked = computed(() => busy.value || phase.value === 'uncertain')
  const canSubmit = computed(() => Boolean(target.value) && !busy.value && phase.value !== 'conflict'
    && (phase.value === 'uncertain' || Boolean(reason.value.trim())))

  function open(row: UserRiskRestriction) {
    if (locked.value) return false
    if (!Number.isSafeInteger(row.version) || row.version <= 0) return false
    target.value = { ...row }
    reason.value = ''
    reviewChanged.value = false
    submitted = null
    phase.value = 'idle'
    error.value = ''
    return true
  }
  function close() {
    if (locked.value) return false
    target.value = null
    submitted = null
    error.value = ''
    return true
  }
  async function submit(): Promise<UserRiskReleaseResult | null> {
    if (!canSubmit.value || !target.value) return null
    const row = target.value
    error.value = ''
    try {
      submitted ??= Object.freeze({
        operation_id: operationID(), expected_version: row.version, reason: reason.value.trim(),
      })
    } catch {
      error.value = message('unavailable')
      return null
    }
    phase.value = 'submitting'
    try {
      const result = await api.release(row.user_id, { ...submitted })
      if (result.user_id !== row.user_id || result.policy_key !== row.policy_key
        || (result.released && (!Number.isSafeInteger(result.version) || result.version <= row.version))) {
        phase.value = 'uncertain'
        error.value = message('uncertain')
        return null
      }
      target.value = null
      submitted = null
      phase.value = 'idle'
      return result
    } catch (cause: unknown) {
      const status = (cause as { response?: { status?: number } } | null)?.response?.status
      if (status === 409) {
        phase.value = 'conflict'
        error.value = message('conflict')
      } else if (status && status >= 400 && status < 500 && status !== 408 && status !== 429) {
        phase.value = 'failed'
        submitted = null
        error.value = extractApiErrorMessage(cause, message('failed'))
      } else {
        phase.value = 'uncertain'
        error.value = message('uncertain')
      }
      return null
    }
  }
  async function reload() {
    if (!target.value || busy.value || phase.value !== 'conflict') return false
    const previous = target.value
    phase.value = 'refreshing'
    try {
      const page = await api.read({ user_id: previous.user_id, page_size: 100 })
      const current = page.items.find(row => row.user_id === previous.user_id && row.policy_key === previous.policy_key)
      if (!current) {
        target.value = null
        submitted = null
        phase.value = 'idle'
        return true
      }
      if (!Number.isSafeInteger(current.version) || current.version <= previous.version) {
        phase.value = 'conflict'
        error.value = message('unchanged')
        return false
      }
      target.value = { ...current }
      reviewChanged.value = true
      submitted = null
      phase.value = 'idle'
      error.value = ''
      return true
    } catch (cause: unknown) {
      phase.value = 'conflict'
      error.value = extractApiErrorMessage(cause, message('failed'))
      return false
    }
  }
  return { target, reason, phase, error, reviewChanged, busy, locked, canSubmit, open, close, submit, reload }
}
