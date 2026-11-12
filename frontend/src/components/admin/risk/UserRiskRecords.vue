<template>
  <section v-if="active" class="risk-records" :aria-labelledby="headingId">
    <header class="records-heading">
      <div>
        <p class="ef-field">// {{ mode === 'history' ? 'ACTION HISTORY' : 'CURRENT RESTRICTIONS' }}</p>
        <h2 :id="headingId">{{ t('admin.riskControl.userRecords.' + (mode === 'history' ? 'historyTitle' : 'currentTitle')) }}</h2>
      </div>
      <button type="button" class="ef-btn small" @click="refreshView">{{ t('admin.riskControl.userRecords.refresh') }}</button>
    </header>
    <form class="records-filters" @submit.prevent="applyFilters">
      <label :for="searchId">
        <span>{{ t('admin.riskControl.userRecords.search') }}</span>
        <input :id="searchId" v-model.trim="search" class="input" type="search" :placeholder="t('admin.riskControl.userRecords.searchHint')" />
      </label>
      <label :for="filterId">
        <span>{{ t('admin.riskControl.userRecords.' + (mode === 'history' ? 'action' : 'releaseMode')) }}</span>
        <select v-if="mode === 'history'" :id="filterId" v-model="action" class="input">
          <option value="">{{ t('admin.riskControl.userRecords.all') }}</option>
          <option v-for="kind in actionKinds" :key="kind" :value="kind">{{ actionLabel(kind) }}</option>
        </select>
        <select v-else :id="filterId" v-model="releaseMode" class="input">
          <option value="">{{ t('admin.riskControl.userRecords.all') }}</option>
          <option value="temporary">{{ t('admin.riskControl.userRecords.temporary') }}</option>
          <option value="manual">{{ t('admin.riskControl.userRecords.manual') }}</option>
        </select>
      </label>
      <label :for="sizeId">
        <span>{{ t('admin.riskControl.userRecords.pageSize') }}</span>
        <select :id="sizeId" v-model.number="pageSize" class="input">
          <option v-for="size in [20, 50, 100]" :key="size" :value="size">{{ size }}</option>
        </select>
      </label>
      <button type="submit" class="ef-btn small">{{ t('admin.riskControl.userRecords.filter') }}</button>
    </form>
    <p v-if="error" class="ef-notice" role="alert">{{ error }}</p>
    <p v-if="releaseNotice" class="ef-notice" role="status">{{ releaseNotice }}</p>
    <p v-if="changedDuringTraversal" class="ef-notice" role="status">{{ t('admin.riskControl.userRecords.updated') }}</p>
    <p v-if="loading" class="records-message" role="status">{{ t('admin.riskControl.userRecords.loading') }}</p>
    <div v-if="metadata" class="records-meta" role="status">
      <span>{{ t('admin.riskControl.userRecords.visibleCount', { total: metadata.total }) }}</span>
      <span>{{ t('admin.riskControl.userRecords.queriedAt', { time: formatTime(metadata.as_of) }) }}</span>
      <span v-if="mode === 'restrictions' && restrictionData">{{ t('admin.riskControl.userRecords.window', { days: restrictionData.window_days }) }}</span>
    </div>
    <ol v-if="mode === 'restrictions' && restrictionData" class="records-list">
      <li v-for="row in restrictionData.items" :key="row.user_id + '/' + row.policy_key" class="records-row">
        <div class="record-identity">
          <span class="ef-field">#{{ row.user_id }}</span>
          <strong>{{ row.username || row.email }}</strong>
          <span v-if="row.username">{{ row.email }}</span>
          <span>{{ t('admin.riskControl.userRecords.baseStatus', { status: baseStatusLabel(row.base_status) }) }}</span>
          <button type="button" class="ef-btn small" @click="openRelease(row)">{{ t('admin.riskControl.userRelease.release') }}</button>
        </div>
        <dl class="record-facts">
          <div><dt>{{ t('admin.riskControl.userRecords.windowCount') }}</dt><dd>{{ row.window_hit_count }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.decisionCount') }}</dt><dd>{{ row.decision_hit_count }} / {{ t('admin.riskControl.userRecords.revision', { revision: row.policy_revision }) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.startedAt') }}</dt><dd>{{ formatTime(row.restricted_at) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.expiry') }}</dt><dd>{{ row.admin_release ? t('admin.riskControl.userRecords.manual') : formatTime(row.expires_at) }}</dd></div>
        </dl>
      </li>
    </ol>
    <ol v-if="mode === 'history' && actionData" class="records-list">
      <li v-for="row in actionData.items" :key="row.id" class="records-row">
        <div class="record-identity">
          <span class="ef-field">#{{ row.user_id }} / {{ row.id }}</span>
          <strong>{{ row.username || row.email }}</strong>
          <span v-if="row.username">{{ row.email }}</span>
        </div>
        <dl class="record-facts">
          <div><dt>{{ t('admin.riskControl.userRecords.time') }}</dt><dd>{{ formatTime(row.created_at) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.action') }}</dt><dd>{{ actionLabel(row.action) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.decisionCount') }}</dt><dd>{{ row.decision.hit_count }} / {{ t('admin.riskControl.userRecords.revision', { revision: row.decision.policy_revision }) }}</dd></div>
          <div><dt>{{ t('admin.riskControl.userRecords.actor') }}</dt><dd>{{ row.decision.actor_label || (row.actor_id ? '#' + row.actor_id : t('admin.riskControl.userRecords.system')) }}</dd></div>
        </dl>
        <p class="record-reason">{{ reasonLabel(row) }}</p>
      </li>
    </ol>
    <p v-if="!loading && !error && metadata && visibleRows === 0" class="records-message">
      {{ t('admin.riskControl.userRecords.' + (mode === 'history' ? 'noActions' : 'noRestrictions')) }}
    </p>
    <footer v-if="metadata" class="records-pagination">
      <span class="ef-field">{{ t('admin.riskControl.userRecords.page', { page: metadata.page }) }}</span>
      <button type="button" class="ef-btn small" :disabled="!canPrevious" @click="previousPage">{{ t('admin.riskControl.userRecords.previous') }}</button>
      <button type="button" class="ef-btn small" :disabled="!canNext" @click="nextPage">{{ t('admin.riskControl.userRecords.next') }}</button>
    </footer>
    <UserRiskReleaseDialog :target="releaseTarget" :reason="releaseReason" :error="releaseError"
      :locked="releaseLocked" :busy="releaseBusy" :can-submit="releaseCanSubmit"
      :conflict="releasePhase === 'conflict'" :uncertain="releasePhase === 'uncertain'" :changed="releaseReviewChanged"
      @update:reason="releaseReason = $event" @close="release.close()" @submit="submitRelease" @reload="reloadRelease" />
  </section>
</template>

<script setup lang="ts">
import { computed, reactive, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { riskControlAPI, type UserRiskAction, type UserRiskRestriction } from '@/api/admin/riskControl'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'
import { useRiskCursorList } from './useRiskCursorList'
import { useUserRiskRelease } from './useUserRiskRelease'
import UserRiskReleaseDialog from './UserRiskReleaseDialog.vue'

const props = defineProps<{ active: boolean; mode: 'restrictions' | 'history'; policyRevision?: number }>()
const { t } = useI18n()
const release = useUserRiskRelease(key => t('admin.riskControl.userRelease.' + key))
const { target: releaseTarget, reason: releaseReason, phase: releasePhase, error: releaseError,
  locked: releaseLocked, busy: releaseBusy, canSubmit: releaseCanSubmit, reviewChanged: releaseReviewChanged } = release
const releaseNotice = ref('')
function openRelease(row: UserRiskRestriction) {
  releaseNotice.value = ''
  release.open(row)
}
async function submitRelease() {
  const result = await release.submit()
  if (!result) return
  releaseNotice.value = t('admin.riskControl.userRelease.' + (result.released ? 'success' : 'gone'))
    + (result.base_status !== 'active' ? ' ' + t('admin.riskControl.userRelease.baseDisabled') : '')
  await refreshView()
}
async function reloadRelease() {
  if (await release.reload()) {
    if (!releaseTarget.value) releaseNotice.value = t('admin.riskControl.userRelease.gone')
    await refreshView()
  }
}
const headingId = useId()
const searchId = useId()
const filterId = useId()
const sizeId = useId()
const search = ref('')
const releaseMode = ref<'' | 'manual' | 'temporary'>('')
const action = ref('')
const pageSize = ref(20)
const applied = reactive({ search: '', releaseMode: '' as '' | 'manual' | 'temporary', action: '', pageSize: 20 })
const actionKinds = ['hit', 'restricted', 'escalated', 'released', 'expired', 'ignored']
const restrictionList = useRiskCursorList(riskControlAPI.listUserRestrictions,
  () => ({ search: applied.search, release_mode: applied.releaseMode || undefined, page_size: applied.pageSize }),
  () => t('admin.riskControl.userRecords.loadFailed'))
const historyList = useRiskCursorList(riskControlAPI.listUserRestrictionActions,
  () => ({ search: applied.search, action: applied.action || undefined, page_size: applied.pageSize }),
  () => t('admin.riskControl.userRecords.loadFailed'))
const { data: restrictionData } = restrictionList
const { data: actionData } = historyList
function activeList() { return props.mode === 'history' ? historyList : restrictionList }
const metadata = computed(() => activeList().data.value)
const loading = computed(() => activeList().loading.value)
const error = computed(() => activeList().error.value)
const changedDuringTraversal = computed(() => activeList().changedDuringTraversal.value)
const canPrevious = computed(() => activeList().canPrevious.value)
const canNext = computed(() => activeList().canNext.value)
const visibleRows = computed(() => props.mode === 'history' ? actionData.value?.items.length ?? 0 : restrictionData.value?.items.length ?? 0)
function refreshView() { return activeList().refresh() }
function previousPage() { return activeList().previous() }
function nextPage() { return activeList().next() }
function applyFilters() {
  applied.search = search.value
  applied.releaseMode = releaseMode.value
  applied.action = action.value
  applied.pageSize = pageSize.value
  void refreshView()
}
watch(() => [props.active, props.mode, props.policyRevision], () => {
  restrictionList.invalidate()
  historyList.invalidate()
  if (props.active) void refreshView()
}, { immediate: true })
function formatTime(value?: string) {
  return value ? formatDateTimeValue(value) || t('admin.riskControl.userRecords.timeUnavailable') : t('admin.riskControl.userRecords.timeUnavailable')
}
function actionLabel(value: string) {
  return actionKinds.includes(value) ? t('admin.riskControl.userRecords.actions.' + value) : value
}
function baseStatusLabel(value: string) {
  return value === 'active' || value === 'disabled' ? t('admin.riskControl.userRecords.' + value) : value
}
function reasonLabel(row: UserRiskAction) {
  if (row.action === 'expired') return t('admin.riskControl.userRecords.automaticExpiry')
  if (row.action === 'ignored') return t('admin.riskControl.userRecords.' + (row.decision.ignored_reason === 'cycle_changed' ? 'lateIgnored' : 'notCounted'))
  return row.reason || t('admin.riskControl.userRecords.systemRecord')
}
</script>

<style scoped>
.risk-records { min-width: 0; }
.records-heading { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 1rem; padding-block: 1.25rem; border-bottom: 1px solid var(--ef-rule); }
.records-heading p { margin: 0 0 .4rem; letter-spacing: .14em; }
.records-heading h2 { margin: 0; font-family: var(--ef-font-ui); font-size: 1.5rem; font-weight: 750; }
.records-filters { display: grid; grid-template-columns: minmax(0, 2fr) minmax(0, 1fr) minmax(0, 1fr) auto; gap: 1rem; align-items: end; padding-block: 1.25rem; }
.records-filters label { display: grid; gap: .4rem; min-width: 0; font-size: .8125rem; }
.records-filters .input { width: 100%; min-width: 0; min-height: 44px; border: 1px solid var(--ef-rule); border-radius: 0; background: var(--ef-panel); color: var(--ef-ink); box-shadow: none; }
.risk-records button { min-height: 44px; }
.records-meta { display: flex; flex-wrap: wrap; gap: .5rem 1.25rem; padding-block: .75rem; color: var(--ef-ink-2); font-size: .8125rem; }
.records-list { margin: 0; padding: 0; list-style: none; }
.records-row { display: grid; grid-template-columns: minmax(0, 1.5fr) minmax(0, 4fr); gap: 1rem 1.5rem; padding-block: 1.25rem; border-bottom: 1px solid var(--ef-rule); min-width: 0; }
.record-identity { display: grid; gap: .35rem; min-width: 0; overflow-wrap: anywhere; }
.record-identity > span { color: var(--ef-ink-2); font-size: .8125rem; }
.record-facts { display: grid; grid-template-columns: repeat(4, minmax(0, 1fr)); gap: 1rem; margin: 0; min-width: 0; }
.record-facts dt { color: var(--ef-ink-2); font-size: .75rem; margin-bottom: .4rem; }
.record-facts dd { margin: 0; font-variant-numeric: tabular-nums; overflow-wrap: anywhere; line-height: 1.6; }
.record-reason { grid-column: 1 / -1; margin: 0; color: var(--ef-ink-2); line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere; }
.records-message { padding-block: 1.5rem; color: var(--ef-ink-2); }
.records-pagination { display: flex; flex-wrap: wrap; align-items: center; justify-content: flex-end; gap: .75rem; padding-block: 1.25rem; }
.records-pagination > span { margin-right: auto; }
.risk-records > .ef-notice { margin-block: 1rem; }
@media (max-width: 767px) {
  .records-filters { grid-template-columns: minmax(0, 1fr); }
  .records-row { grid-template-columns: minmax(0, 1fr); }
  .record-facts { grid-template-columns: repeat(2, minmax(0, 1fr)); }
}
</style>
