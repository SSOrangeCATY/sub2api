<template>
  <dialog ref="dialog" class="risk-release" :aria-labelledby="titleId" :aria-describedby="helpId" @cancel.prevent="cancel">
    <form @submit.prevent="emit('submit')">
      <header>
        <p class="ef-field">// RELEASE RESTRICTION</p>
        <h2 :id="titleId">{{ t('admin.riskControl.userRelease.title') }}</h2>
      </header>
      <template v-if="target">
        <dl class="release-facts">
          <div><dt>{{ t('admin.riskControl.userRelease.user') }}</dt><dd>#{{ target.user_id }} / {{ target.email }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRelease.version') }}</dt><dd>{{ target.version }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRelease.scope') }}</dt><dd>{{ target.scope === 'api' ? t('admin.riskControl.userRelease.apiAccess') : target.scope }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRelease.level') }}</dt><dd>{{ target.decision_hit_count }} / {{ target.threshold }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRelease.penalty') }}</dt><dd>{{ t('admin.riskControl.userRecords.' + (target.admin_release ? 'manual' : 'temporary')) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.startedAt') }}</dt><dd>{{ formatTime(target.restricted_at) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.expiry') }}</dt><dd>{{ target.admin_release ? t('admin.riskControl.userRecords.manual') : formatTime(target.expires_at) }}</dd></div>
        </dl>
        <p v-if="changed" class="ef-notice" role="status">{{ t('admin.riskControl.userRelease.reviewChanged') }}</p>
        <p :id="helpId" class="release-help">{{ t('admin.riskControl.userRelease.help') }}</p>
        <p v-if="target.base_status !== 'active'" class="ef-notice">{{ t('admin.riskControl.userRelease.baseDisabled') }}</p>
        <label :for="reasonId">{{ t('admin.riskControl.userRelease.reason') }}</label>
        <textarea :id="reasonId" :value="reason" :readonly="locked" required maxlength="500" rows="4"
          @input="emit('update:reason', ($event.target as HTMLTextAreaElement).value)" />
        <p v-if="error" class="ef-notice" role="alert">{{ error }}</p>
        <footer>
          <button type="button" class="ef-btn small" :disabled="locked" @click="emit('close')">{{ t('admin.riskControl.userRelease.cancel') }}</button>
          <button v-if="conflict" type="button" class="ef-btn go" :disabled="busy" @click="emit('reload')">{{ t('admin.riskControl.userRelease.reload') }}</button>
          <button v-else type="submit" class="ef-btn go" :disabled="!canSubmit">{{ t('admin.riskControl.userRelease.' + (busy ? 'submitting' : uncertain ? 'retry' : 'release')) }}</button>
        </footer>
      </template>
    </form>
  </dialog>
</template>

<script setup lang="ts">
import { nextTick, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UserRiskRestriction } from '@/api/admin/riskControl'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{
  target: UserRiskRestriction | null; reason: string; error: string
  locked: boolean; busy: boolean; conflict: boolean; uncertain: boolean; canSubmit: boolean
  changed: boolean
}>()
const emit = defineEmits<{
  'update:reason': [value: string]; submit: []; reload: []; close: []
}>()
const { t } = useI18n()
const dialog = ref<HTMLDialogElement | null>(null)
const titleId = useId()
const helpId = useId()
const reasonId = useId()
function cancel() { if (!props.locked) emit('close') }
function formatTime(value?: string) {
  return value ? formatDateTime(value) : t('admin.riskControl.userRecords.timeUnavailable')
}
watch(() => props.target, async (target) => {
  await nextTick()
  const element = dialog.value
  if (!element) return
  if (target && !element.open) element.showModal()
  else if (!target && element.open) element.close()
}, { immediate: true })
</script>

<style scoped>
.risk-release { width: min(36rem, calc(100vw - 2rem)); max-height: calc(100dvh - 2rem); padding: 1.5rem; border: 1px solid var(--ef-rule); border-radius: 0; background: var(--ef-panel); color: var(--ef-ink); box-shadow: none; overflow: auto; }
.risk-release::backdrop { background: rgb(0 0 0 / 55%); }
.risk-release header { padding-bottom: 1rem; border-bottom: 1px solid var(--ef-rule); }
.risk-release header p { margin: 0 0 .5rem; }
.risk-release h2 { margin: 0; font-family: var(--ef-font-ui); font-weight: 750; }
.release-facts { display: grid; grid-template-columns: minmax(0, 3fr) minmax(0, 1fr); gap: 1rem; padding-block: 1rem; margin: 0; border-bottom: 1px solid var(--ef-rule); }
.release-facts dt { margin-bottom: .4rem; color: var(--ef-ink-2); font-size: .8125rem; }
.release-facts dd { margin: 0; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
.release-help { line-height: 1.7; color: var(--ef-ink-2); }
.risk-release label { display: block; margin-block: 1rem .5rem; }
.risk-release textarea { display: block; width: 100%; min-height: 7rem; padding: .75rem; border: 1px solid var(--ef-rule); border-radius: 0; background: var(--ef-paper); color: var(--ef-ink); font: inherit; font-size: 1rem; resize: vertical; }
.risk-release textarea:focus-visible { outline: 2px solid var(--ef-trace); outline-offset: 2px; }
.risk-release footer { display: flex; flex-wrap: wrap; justify-content: flex-end; gap: .75rem; margin-top: 1.5rem; }
.risk-release button { min-height: 44px; }
@media (max-width: 479px) { .risk-release { padding: 1rem; } .release-facts { grid-template-columns: minmax(0, 1fr); } }
</style>
