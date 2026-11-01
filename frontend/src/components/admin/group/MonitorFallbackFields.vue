<template>
  <section class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-dark-600">
    <label class="input-label" for="monitor-group-type">{{ t('admin.groups.monitorFallback.type') }}</label>
    <select id="monitor-group-type" class="input" :value="groupType" @change="changeType">
      <option value="business">{{ t('admin.groups.monitorFallback.business') }}</option>
      <option value="fallback" :disabled="platform === 'composite' || subscriptionType === 'subscription'">{{ t('admin.groups.monitorFallback.fallback') }}</option>
    </select>
    <template v-if="groupType === 'fallback'">
      <p class="input-hint">{{ t('admin.groups.monitorFallback.hint') }}</p>
      <ol class="space-y-2">
        <li v-for="(id, index) in groupIds" :key="id" class="flex items-center gap-2 text-sm">
          <span class="min-w-0 flex-1">{{ index + 1 }}. {{ candidates.find(g => g.id === id)?.name ?? '#' + id }}</span>
          <button type="button" class="btn btn-secondary" :disabled="index === 0" :aria-label="t('admin.groups.monitorFallback.up')" @click="move(index, -1)">↑</button>
          <button type="button" class="btn btn-secondary" :disabled="index === groupIds.length - 1" :aria-label="t('admin.groups.monitorFallback.down')" @click="move(index, 1)">↓</button>
          <button type="button" class="btn btn-secondary" :aria-label="t('admin.groups.monitorFallback.remove')" @click="emit('update:groupIds', groupIds.filter(value => value !== id))">×</button>
        </li>
      </ol>
      <select class="input" :disabled="loading || groupIds.length >= 20" :aria-label="t('admin.groups.monitorFallback.add')" @change="add">
        <option value="">{{ t('admin.groups.monitorFallback.add') }}</option>
        <option v-for="group in eligible" :key="group.id" :value="group.id" :disabled="groupIds.includes(group.id)">{{ group.name }} ({{ group.rate_multiplier }}x)</option>
      </select>
      <p v-if="error" role="alert" class="text-sm text-red-500">{{ error }}</p>
      <p class="input-hint">{{ t('admin.groups.monitorFallback.billing') }}</p>
    </template>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminGroup, GroupPlatform } from '@/types'
const props = defineProps<{ groupType: 'business' | 'fallback'; groupIds: number[]; platform: GroupPlatform; currentId?: number; subscriptionType?: string }>()
const emit = defineEmits<{ 'update:groupType': [value: 'business' | 'fallback']; 'update:groupIds': [value: number[]] }>()
const { t } = useI18n()
const candidates = ref<AdminGroup[]>([])
const loading = ref(false)
const error = ref('')
const eligible = computed(() => candidates.value.filter(g => g.id !== props.currentId && g.status === 'active' && g.platform === props.platform && (g.group_type ?? 'business') === 'business' && g.subscription_type !== 'subscription' && !g.claude_code_only && !g.fallback_group_id_on_invalid_request && !(g.fallback_group_ids?.length)))
async function load() {
  loading.value = true
  try { candidates.value = await adminAPI.groups.getAll(); error.value = '' }
  catch { error.value = t('admin.groups.monitorFallback.loadFailed') }
  finally { loading.value = false }
}
function changeType(event: Event) {
  const value = (event.target as HTMLSelectElement).value as 'business' | 'fallback'
  emit('update:groupType', value)
  if (value === 'business') emit('update:groupIds', [])
}
function add(event: Event) {
  const input = event.target as HTMLSelectElement
  const id = Number(input.value)
  if (eligible.value.some(g => g.id === id) && !props.groupIds.includes(id)) emit('update:groupIds', [...props.groupIds, id])
  input.value = ''
}
function move(index: number, direction: number) {
  const next = [...props.groupIds]
  const target = index + direction
  if (target < 0 || target >= next.length) return
  ;[next[index], next[target]] = [next[target]!, next[index]!]
  emit('update:groupIds', next)
}
watch(() => props.platform, () => {
  if (props.platform === 'composite') {
    emit('update:groupIds', [])
    emit('update:groupType', 'business')
  } else if (candidates.value.length) {
    // Opening another group's editor updates platform and chain together.
    // Preserve hydrated targets belonging to that platform.
    const next = props.groupIds.filter(id => eligible.value.some(group => group.id === id))
    if (next.length !== props.groupIds.length) emit('update:groupIds', next)
  }
})
onMounted(load)
</script>
