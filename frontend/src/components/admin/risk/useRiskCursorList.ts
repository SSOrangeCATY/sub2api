import { computed, onScopeDispose, readonly, ref, shallowRef } from 'vue'
import type { UserRiskCursorPage, UserRiskListParams } from '@/api/admin/riskControl'
import { extractApiErrorMessage } from '@/utils/apiError'

// One controller owns one independent list. Failed or superseded reads never
// replace visible records or advance the committed cursor path.
export function useRiskCursorList<T extends UserRiskCursorPage>(
  fetchPage: (params: UserRiskListParams) => Promise<T>,
  filters: () => UserRiskListParams,
  fallbackMessage: () => string,
) {
  const data = shallowRef<T | null>(null)
  const loading = ref(false)
  const error = ref('')
  const cursors = ref<string[]>([''])
  const changedDuringTraversal = ref(false)
  let sequence = 0
  let disposed = false

  async function read(target: string[]): Promise<boolean> {
    if (disposed) return false
    const ticket = ++sequence
    loading.value = true
    error.value = ''
    try {
      const result = await fetchPage({
        ...filters(),
        cursor: target.at(-1) || undefined,
      })
      if (disposed || ticket !== sequence) return false
      const previous = data.value
      changedDuringTraversal.value = target.length === 1
        ? false
        : changedDuringTraversal.value || Boolean(previous && previous.total !== result.total)
      data.value = result
      cursors.value = [...target]
      return true
    } catch (cause: unknown) {
      if (!disposed && ticket === sequence) {
        error.value = extractApiErrorMessage(cause, fallbackMessage())
      }
      return false
    } finally {
      if (!disposed && ticket === sequence) loading.value = false
    }
  }

  const canPrevious = computed(() => !loading.value && cursors.value.length > 1)
  const canNext = computed(() => {
    const page = data.value
    return !loading.value && Boolean(page?.has_more && page.next_cursor)
  })
  const refresh = () => read([''])
  const next = () => canNext.value && data.value
    ? read([...cursors.value, data.value.next_cursor])
    : Promise.resolve(false)
  const previous = () => canPrevious.value
    ? read(cursors.value.slice(0, -1))
    : Promise.resolve(false)

  function invalidate() {
    sequence++
    loading.value = false
  }
  onScopeDispose(() => {
    disposed = true
    invalidate()
  })

  return {
    data, loading, error, canPrevious, canNext, refresh, next, previous, invalidate,
    cursors: readonly(cursors),
    changedDuringTraversal: readonly(changedDuringTraversal),
  }
}
