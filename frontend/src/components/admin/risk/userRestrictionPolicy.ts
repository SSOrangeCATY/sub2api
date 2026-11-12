export interface UserRestrictionActionDraft {
  threshold: number | ''
  duration_hours: number | ''
  admin_release: boolean
}
export interface UserRestrictionPolicyDraft {
  enabled: boolean
  window_days: number | ''
  actions: UserRestrictionActionDraft[]
}
export type UserRestrictionPolicyError = 'window' | 'count' | 'threshold' | 'duration' | 'manualLast'
export interface UserRestrictionPolicyIssue {
  code: UserRestrictionPolicyError
  rowIndex?: number
  field: 'window_days' | 'actions' | 'threshold' | 'duration_hours' | 'admin_release'
}
export function getUserRestrictionPolicyIssue(policy: UserRestrictionPolicyDraft): UserRestrictionPolicyIssue | null {
  const window = Number(policy.window_days)
  if (!Number.isInteger(window) || window === 0 || Math.sign(window) !== 1 || window > 30) return { code: 'window', field: 'window_days' }
  if (policy.actions.length === 0 || policy.actions.length > 8) return { code: 'count', field: 'actions' }
  let previous = 0
  for (const [index, action] of policy.actions.entries()) {
    const threshold = Number(action.threshold)
    if (!Number.isInteger(threshold) || threshold === 0 || Math.sign(threshold - previous) !== 1 || threshold > 1000) return { code: 'threshold', rowIndex: index, field: 'threshold' }
    if (action.admin_release) {
      if (index !== policy.actions.length - 1 || Number(action.duration_hours) !== 0) return { code: 'manualLast', rowIndex: index, field: 'admin_release' }
    } else {
      const duration = Number(action.duration_hours)
      if (!Number.isInteger(duration) || duration === 0 || Math.sign(duration) !== 1 || duration > 720) return { code: 'duration', rowIndex: index, field: 'duration_hours' }
    }
    previous = threshold
  }
  return null
}
export function validateUserRestrictionPolicy(policy: UserRestrictionPolicyDraft): UserRestrictionPolicyError | null {
  return getUserRestrictionPolicyIssue(policy)?.code ?? null
}
export function cloneUserRestrictionPolicy(policy: UserRestrictionPolicyDraft): UserRestrictionPolicyDraft {
  return { ...policy, actions: policy.actions.map(action => ({ ...action })) }
}
