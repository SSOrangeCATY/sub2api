<template>
  <section class="risk-policy-section" :aria-labelledby="headingId">
    <header class="policy-overview">
      <div>
        <p class="ef-field">// SAVED POLICY</p>
        <h2 :id="headingId">{{ t('admin.riskControl.userPolicy.savedTitle') }}</h2>
      </div>
      <span v-if="serverConfig?.revision" class="ef-field">{{ t('admin.riskControl.userPolicy.revision') }} / {{ serverConfig.revision }}</span>
    </header>
    <p v-if="errorMessage" ref="notice" class="ef-notice" role="alert" tabindex="-1">{{ errorMessage }}</p>
    <div class="policy-context">
      <p>{{ scopeSummary }}</p>
      <div class="policy-context-actions">
        <button type="button" class="ef-btn small" :disabled="saving || reading || blocked" @click="emit('editScope')">{{ t('admin.riskControl.userPolicy.editScope') }}</button>
        <button data-test="reload-user-policy" type="button" class="ef-btn small" :disabled="saving || reading || blocked" @click="reloadPolicy">{{ reading ? t('common.loading') : t('admin.riskControl.userPolicy.reloadKeepingDraft') }}</button>
      </div>
    </div>
    <p class="policy-scope-hint">{{ t('admin.riskControl.userPolicy.sharedScopeHint') }}</p>
    <ol v-if="available && savedPolicy" class="saved-rule-list">
      <li v-for="(rule, index) in savedPolicy.actions" :key="index">
        {{ rule.admin_release
          ? t('admin.riskControl.userPolicy.previewManual', { days: savedPolicy.window_days, count: rule.threshold })
          : t('admin.riskControl.userPolicy.previewTemporary', { days: savedPolicy.window_days, count: rule.threshold, hours: rule.duration_hours }) }}
      </li>
    </ol>
    <UserRestrictionPolicyEditor
      v-if="draft && revision !== null"
      :model-value="draft"
      :revision="revision"
      :scope-summary="scopeSummary"
      :master-enabled="masterEnabled"
      :dirty="dirty"
      :saving="saving || Boolean(blocked)"
      :save-blocked="!available || conflict || reading"
      @update:model-value="controller.updateDraft"
      @reset="controller.reset"
      @save="savePolicy"
    />
    <p v-else class="ef-notice" role="status">{{ t('admin.riskControl.userPolicy.unsupported') }}</p>
  </section>
</template>

<script setup lang="ts">
import { computed, nextTick, onScopeDispose, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ContentModerationConfig } from '@/api/admin/riskControl'
import { extractApiErrorMessage } from '@/utils/apiError'
import UserRestrictionPolicyEditor from './UserRestrictionPolicyEditor.vue'
import { useUserRiskPolicyDraft } from './useUserRiskPolicyDraft'

const props = defineProps<{
  config: ContentModerationConfig
  masterEnabled: boolean
  blocked?: boolean
}>()
const emit = defineEmits<{
  saved: [config: ContentModerationConfig, previousRevision: number]
  latest: [config: ContentModerationConfig]
  editScope: []
  dirty: [value: boolean]
  busy: [value: boolean]
}>()
const { t } = useI18n()
const headingId = useId()
const notice = ref<HTMLElement | null>(null)
const controller = useUserRiskPolicyDraft()
const { serverConfig, draft, revision, available, saving, reading, conflict, errorKind, cause, dirty } = controller
watch(() => props.config, value => controller.acceptConfig(value), { immediate: true })
watch(dirty, value => emit('dirty', value), { immediate: true })
watch(() => saving.value || reading.value, value => emit('busy', value), { immediate: true })
onScopeDispose(() => emit('busy', false))
const savedPolicy = computed(() => serverConfig.value?.cyber_policy_user_restriction)
const scopeSummary = computed(() => {
  const config = serverConfig.value
  const groups = config?.all_groups === true
    ? t('admin.riskControl.userPolicy.scopeAll')
    : config?.all_groups === false && Array.isArray(config.group_ids)
      ? t('admin.riskControl.userPolicy.scopeGroups', { count: config.group_ids.length })
      : t('admin.riskControl.userPolicy.scopeUnknown')
  const models = config?.model_filter
  const modelScope = models?.type === 'all'
    ? t('admin.riskControl.userPolicy.modelsAll')
    : models && (models.type === 'include' || models.type === 'exclude') && Array.isArray(models.models)
      ? t('admin.riskControl.userPolicy.' + (models.type === 'include' ? 'modelsInclude' : 'modelsExclude'), { count: models.models.length })
      : t('admin.riskControl.userPolicy.scopeUnknown')
  return groups + ' / ' + modelScope
})
const errorMessage = computed(() => {
  if (conflict.value || errorKind.value === 'version') return t('admin.riskControl.userPolicy.conflict')
  if (errorKind.value === 'unsupported') return t('admin.riskControl.userPolicy.unsupported')
  if (errorKind.value === 'load') return extractApiErrorMessage(cause.value, t('admin.riskControl.userPolicy.editorLoadFailed'))
  if (errorKind.value === 'save') return extractApiErrorMessage(cause.value, t('admin.riskControl.userPolicy.editorSaveFailed'))
  return ''
})
async function savePolicy() {
  if (props.blocked) return
  const result = await controller.save()
  if (result) emit('saved', result.config, result.previousRevision)
  else if (errorMessage.value) await nextTick(() => notice.value?.focus())
}
async function reloadPolicy() {
  if (props.blocked) return
  if (await controller.reload()) {
    if (serverConfig.value) emit('latest', serverConfig.value)
  } else if (errorMessage.value) await nextTick(() => notice.value?.focus())
}
</script>

<style scoped>
.policy-scope-hint { margin-block: 0 1rem; color: var(--ef-ink-2); font-size: .8125rem; line-height: 1.7; }
.risk-policy-section { min-width: 0; }
.policy-overview { display: flex; flex-wrap: wrap; align-items: baseline; justify-content: space-between; gap: 1rem; padding-block: 1rem; border-bottom: 1px solid var(--ef-rule); }
.policy-overview p { margin: 0 0 .4rem; letter-spacing: .14em; }
.policy-overview h2 { margin: 0; font-family: var(--ef-font-ui); font-size: 1.125rem; font-weight: 750; }
.policy-context { display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 1rem; padding-block: 1rem; }
.policy-context p { margin: 0; color: var(--ef-ink-2); overflow-wrap: anywhere; }
.policy-context-actions { display: flex; flex-wrap: wrap; gap: .75rem; }
.policy-context-actions button { min-height: 44px; }
.saved-rule-list { margin: 0; padding: 0 0 1rem 1.25rem; border-bottom: 1px solid var(--ef-rule); line-height: 1.8; }
.risk-policy-section > .ef-notice { margin-block: 1rem; }
</style>
