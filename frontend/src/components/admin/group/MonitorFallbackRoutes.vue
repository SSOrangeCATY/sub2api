<template>
  <section class="mb-4 space-y-3 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-600 dark:bg-dark-800">
    <div class="flex flex-wrap items-center gap-3">
      <h3 class="font-semibold">{{ t('admin.groups.monitorFallback.routes') }}</h3>
      <input v-model="model" class="input max-w-xs" :placeholder="t('admin.groups.monitorFallback.model')" :aria-label="t('admin.groups.monitorFallback.model')" @keyup.enter="refresh" />
      <button type="button" class="btn btn-secondary" :disabled="loading" @click="refresh">{{ t('admin.groups.monitorFallback.refresh') }}</button>
    </div>
    <p class="input-hint">{{ t('admin.groups.monitorFallback.statusHint') }}</p>
    <p v-if="error" role="alert" class="monitor-status monitor-status--degraded">{{ error }}</p>
    <p v-else-if="!routes.length" class="text-sm text-gray-500">{{ t('admin.groups.monitorFallback.empty') }}</p>
    <div v-for="route in routes" :key="route.entry_group_id" class="rounded-lg bg-gray-50 p-3 text-sm dark:bg-dark-700">
      <strong>{{ route.entry_group_name }}</strong>
      <span class="ml-2 monitor-status" :class="route.target_group_id && route.target_health === 'operational' ? 'monitor-status--operational' : 'monitor-status--degraded'"> → {{ route.target_group_name || t('admin.groups.monitorFallback.unavailable') }}</span>
      <span v-if="route.target_group_id" class="ml-2 text-xs text-gray-500">{{ t('admin.groups.monitorFallback.health.' + (route.target_health || 'unknown')) }}</span>
      <div v-if="audience === 'admin'" class="mt-1 text-xs text-gray-500">{{ route.candidates?.map(c => (c.group_name || '#' + c.group_id) + ': ' + c.status + ' (' + c.reason + ')').join(' → ') }}</div>
    </div>
  </section>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { apiClient } from '@/api/client'
interface Route { entry_group_id: number; entry_group_name: string; target_group_id: number | null; target_group_name: string; target_health?: string; candidates: Array<{ group_id: number; group_name: string; status: string; reason: string }> | null }
const props = withDefaults(defineProps<{ audience?: 'admin' | 'user' }>(), { audience: 'admin' })
const { t } = useI18n()
const model = ref('')
const routes = ref<Route[]>([])
const error = ref('')
const loading = ref(false)
let timer: ReturnType<typeof setInterval> | undefined
let active = true
async function refresh() {
  if (loading.value) return
  loading.value = true
  try {
    const endpoint = props.audience === 'admin' ? '/admin/groups/fallback-routes' : '/channel-monitors/fallback-routes'
    const { data } = await apiClient.get<Route[]>(endpoint, { params: { model: model.value.trim() || undefined } })
    if (active) { routes.value = data; error.value = '' }
  } catch { if (active) { routes.value = []; error.value = t('admin.groups.monitorFallback.loadFailed') } }
  finally { loading.value = false }
}
onMounted(() => { void refresh(); timer = setInterval(() => { if (!document.hidden) void refresh() }, 15000) })
onUnmounted(() => { active = false; if (timer) clearInterval(timer) })
</script>
