<template>
  <section class="user-restriction-policy" :aria-labelledby="titleId">
    <header class="policy-heading">
      <div>
        <p class="ef-field policy-kicker">// USER RESTRICTIONS</p>
        <h2 :id="titleId">{{ t('admin.riskControl.userPolicy.title') }}</h2>
      </div>
      <span class="ef-field">{{ t('admin.riskControl.userPolicy.revision') }} / {{ revision }}</span>
    </header>

    <p v-if="!masterEnabled" class="ef-notice" role="status">
      {{ t('admin.riskControl.userPolicy.masterOff') }}
    </p>
    <p class="policy-scope">{{ scopeSummary }}</p>

    <form @submit.prevent="save">
      <fieldset :disabled="saving" class="policy-fields">
        <legend class="sr-only">{{ t('admin.riskControl.userPolicy.title') }}</legend>
        <div class="policy-setting-row">
          <label class="policy-toggle">
            <input data-test="user-policy-enabled" type="checkbox" :checked="modelValue.enabled" @change="setEnabled(($event.target as HTMLInputElement).checked)" />
            <span>{{ t('admin.riskControl.userPolicy.enabled') }}</span>
          </label>
          <p class="policy-hint">{{ t('admin.riskControl.userPolicy.disableHint') }}</p>
        </div>
        <div class="policy-setting-row">
          <label :for="windowId">{{ t('admin.riskControl.userPolicy.window') }}</label>
          <div class="policy-number-unit">
            <input :id="windowId" data-test="user-policy-window" class="input" type="number" min="1" max="30" step="1" :value="modelValue.window_days" :aria-invalid="validationError === 'window'" :aria-describedby="validationError === 'window' ? errorId : undefined" @input="setWindow(($event.target as HTMLInputElement).value)" />
            <span>{{ t('admin.riskControl.userPolicy.days') }}</span>
          </div>
          <p class="policy-hint">{{ t('admin.riskControl.userPolicy.windowHint') }}</p>
        </div>

        <ol class="policy-action-list" :aria-describedby="validationError === 'count' ? errorId : undefined">
          <li v-for="(action, index) in modelValue.actions" :key="index" class="policy-action-row">
            <span class="ef-field policy-row-number">{{ String(index + 1).padStart(2, '0') }}</span>
            <label class="policy-action-field">
              <span>{{ t('admin.riskControl.userPolicy.threshold') }}</span>
              <input :data-test="'user-policy-threshold-' + index" class="input" type="number" min="1" max="1000" step="1" :value="action.threshold" :aria-invalid="rowInvalid(index, 'threshold')" :aria-describedby="rowInvalid(index, 'threshold') ? errorId : undefined" @input="setActionNumber(index, 'threshold', ($event.target as HTMLInputElement).value)" />
            </label>
            <label class="policy-action-field">
              <span>{{ t('admin.riskControl.userPolicy.releaseMode') }}</span>
              <select :data-test="'user-policy-release-' + index" class="input" :value="action.admin_release ? 'manual' : 'automatic'" :aria-invalid="rowInvalid(index, 'admin_release')" :aria-describedby="rowInvalid(index, 'admin_release') ? errorId : undefined" @change="setReleaseMode(index, ($event.target as HTMLSelectElement).value === 'manual')">
                <option value="automatic">{{ t('admin.riskControl.userPolicy.automatic') }}</option>
                <option value="manual">{{ t('admin.riskControl.userPolicy.manual') }}</option>
              </select>
            </label>
            <label v-if="!action.admin_release" class="policy-action-field">
              <span>{{ t('admin.riskControl.userPolicy.duration') }}</span>
              <div class="policy-number-unit">
                <input :data-test="'user-policy-duration-' + index" class="input" type="number" min="1" max="720" step="1" :value="action.duration_hours" :aria-invalid="rowInvalid(index, 'duration_hours')" :aria-describedby="rowInvalid(index, 'duration_hours') ? errorId : undefined" @input="setActionNumber(index, 'duration_hours', ($event.target as HTMLInputElement).value)" />
                <span>{{ t('admin.riskControl.userPolicy.hours') }}</span>
              </div>
            </label>
            <p v-else class="policy-hint">{{ t('admin.riskControl.userPolicy.manualHint') }}</p>
            <button :data-test="'user-policy-remove-' + index" type="button" class="ef-btn small" @click="removeAction(index)">{{ t('admin.riskControl.userPolicy.remove') }}</button>
          </li>
        </ol>
        <button data-test="user-policy-add" type="button" class="ef-btn small" :disabled="!canAddRule || saving" :aria-describedby="hasTerminalRule ? terminalHintId : (thresholdLimitReached ? thresholdLimitHintId : (validationError === 'count' ? errorId : undefined))" @click="addAction">{{ t('admin.riskControl.userPolicy.add') }}</button>
        <p v-if="hasTerminalRule" :id="terminalHintId" class="policy-hint">{{ t('admin.riskControl.userPolicy.terminalHint') }}</p>
        <p v-else-if="thresholdLimitReached" :id="thresholdLimitHintId" class="policy-hint">{{ t('admin.riskControl.userPolicy.thresholdLimitHint') }}</p>
      </fieldset>

      <p v-if="validationError" :id="errorId" data-test="user-policy-error" class="ef-notice" role="alert">{{ validationMessage }}</p>

      <div class="policy-preview">
        <h3>{{ t('admin.riskControl.userPolicy.preview') }}</h3>
        <p v-if="validationError">{{ t('admin.riskControl.userPolicy.correctDraft') }}</p>
        <template v-else>
          <p v-if="!masterEnabled || !modelValue.enabled" class="policy-hint">{{ t('admin.riskControl.userPolicy.inactivePreview') }}</p>
          <p v-for="(action, index) in modelValue.actions" :key="index">
            {{ action.admin_release
              ? t('admin.riskControl.userPolicy.previewManual', { days: modelValue.window_days, count: action.threshold })
              : t('admin.riskControl.userPolicy.previewTemporary', { days: modelValue.window_days, count: action.threshold, hours: action.duration_hours }) }}
          </p>
        </template>
      </div>

      <div class="ef-actions policy-footer">
        <span v-if="dirty" class="ef-field" role="status">{{ t('admin.riskControl.userPolicy.unsaved') }}</span>
        <button data-test="user-policy-reset" type="button" class="ef-btn" :disabled="saving || !dirty" @click="emit('reset')">{{ t('admin.riskControl.userPolicy.reset') }}</button>
        <button data-test="user-policy-save" type="submit" class="ef-btn go" :aria-busy="saving" :disabled="saving || saveBlocked || !dirty || validationError !== null">{{ saving ? t('common.saving') : t('admin.riskControl.userPolicy.save') }}</button>
      </div>
    </form>
  </section>
</template>

<script setup lang="ts">
import { computed, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { cloneUserRestrictionPolicy, getUserRestrictionPolicyIssue, type UserRestrictionActionDraft, type UserRestrictionPolicyDraft } from './userRestrictionPolicy'

const props = defineProps<{
  modelValue: UserRestrictionPolicyDraft
  revision: string | number
  scopeSummary: string
  masterEnabled: boolean
  dirty: boolean
  saving: boolean
  saveBlocked?: boolean
}>()
const emit = defineEmits<{
  'update:modelValue': [policy: UserRestrictionPolicyDraft]
  save: []
  reset: []
}>()
const { t } = useI18n()
const titleId = useId()
const windowId = useId()
const errorId = useId()
const terminalHintId = useId()
const thresholdLimitHintId = useId()
const validationIssue = computed(() => getUserRestrictionPolicyIssue(props.modelValue))
const validationError = computed(() => validationIssue.value?.code ?? null)
const hasTerminalRule = computed(() => props.modelValue.actions.some(action => action.admin_release))
const thresholdLimitReached = computed(() => Number(props.modelValue.actions.at(-1)?.threshold) >= 1000)
const canAddRule = computed(() => !(props.modelValue.actions.length >= 8) && !hasTerminalRule.value && !thresholdLimitReached.value)
const validationMessage = computed(() => {
  const issue = validationIssue.value
  if (!issue) return ''
  const message = t('admin.riskControl.userPolicy.errors.' + issue.code)
  return issue.rowIndex === undefined ? message : t('admin.riskControl.userPolicy.rowError', { row: issue.rowIndex + 1, message })
})
function rowInvalid(index: number, field: 'threshold' | 'duration_hours' | 'admin_release') {
  return validationIssue.value?.rowIndex === index && validationIssue.value?.field === field
}
const numericDraft = (value: string): number | '' => value === '' ? '' : Number(value)

function update(change: (draft: UserRestrictionPolicyDraft) => void) {
  if (props.saving) return
  const draft = cloneUserRestrictionPolicy(props.modelValue)
  change(draft)
  emit('update:modelValue', draft)
}
function setEnabled(enabled: boolean) { update(draft => { draft.enabled = enabled }) }
function setWindow(value: string) { update(draft => { draft.window_days = numericDraft(value) }) }
function setActionNumber(index: number, field: 'threshold' | 'duration_hours', value: string) {
  update(draft => { draft.actions[index][field] = numericDraft(value) })
}
function setReleaseMode(index: number, manual: boolean) {
  update(draft => {
    draft.actions[index].admin_release = manual
    draft.actions[index].duration_hours = manual ? 0 : 24
  })
}
function addAction() {
  if (!canAddRule.value || props.saving) return
  update(draft => {
    const threshold = Number(draft.actions.at(-1)?.threshold || 0) + 1
    const action: UserRestrictionActionDraft = { threshold, duration_hours: 24, admin_release: false }
    draft.actions.push(action)
  })
}
function removeAction(index: number) { update(draft => { draft.actions.splice(index, 1) }) }
function save() {
  if (!props.saving && !props.saveBlocked && props.dirty && validationError.value === null) emit('save')
}
</script>

<style scoped>
.user-restriction-policy { min-width: 0; }
.policy-heading { display: flex; align-items: baseline; justify-content: space-between; flex-wrap: wrap; gap: 1rem; padding-block: 1.25rem; border-bottom: 1px solid var(--ef-rule); }
.policy-kicker { margin: 0 0 .4rem; letter-spacing: .14em; }
.policy-heading h2 { margin: 0; font-family: var(--ef-font-ui); font-size: 1.5rem; font-weight: 750; }
.policy-scope { margin-block: 1rem; color: var(--ef-ink-2); }
.policy-fields { margin: 0; padding: 0; border: 0; min-width: 0; }
.policy-setting-row { display: flex; flex-wrap: wrap; align-items: center; gap: .75rem 1.5rem; padding-block: 1rem; border-bottom: 1px solid var(--ef-rule); }
.policy-toggle { display: flex; align-items: center; gap: .6rem; min-height: 44px; }
.policy-hint { margin: 0; color: var(--ef-ink-2); font-size: .8125rem; line-height: 1.7; overflow-wrap: anywhere; }
.policy-setting-row > .policy-hint { flex-basis: 100%; }
.policy-number-unit { display: flex; align-items: center; gap: .5rem; min-width: 0; }
.policy-number-unit input { min-width: 0; max-width: 7rem; }
.policy-action-list { list-style: none; padding: 0; margin: 1rem 0; }
.policy-action-row { display: grid; grid-template-columns: 2rem minmax(0, 1fr) minmax(0, 1.5fr) minmax(0, 1.5fr) auto; align-items: center; gap: 1rem; border-bottom: 1px solid var(--ef-rule); padding-block: 1rem; }
.policy-action-field { display: grid; gap: .4rem; min-width: 0; font-size: .8125rem; }
.policy-action-field .input { min-height: 44px; width: 100%; min-width: 0; }
.policy-preview { margin-block: 1.5rem; padding-block: 1rem; border-block: 1px solid var(--ef-rule); }
.policy-preview h3 { font-size: .9375rem; margin: 0 0 .75rem; }
.policy-preview p { margin: .5rem 0; line-height: 1.7; }
.policy-footer { display: flex; align-items: center; flex-wrap: wrap; justify-content: flex-end; gap: .75rem; }
.policy-footer > span { margin-right: auto; }
.policy-action-row > button, .policy-footer button, .policy-fields > button { min-height: 44px; }
@media (max-width: 767px) {
  .policy-action-row { grid-template-columns: 2rem minmax(0, 1fr); gap: .75rem; }
  .policy-row-number { align-self: start; padding-top: .4rem; }
  .policy-action-field, .policy-action-row > .policy-hint, .policy-action-row > button { grid-column: 2; }
  .policy-action-row > button { justify-self: start; }
  .policy-footer > span { width: 100%; }
}
</style>
