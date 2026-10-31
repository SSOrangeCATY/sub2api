<template>
  <div class="card p-4">
    <div class="mb-4 flex items-center justify-between gap-3">
      <h3 class="text-sm font-semibold text-gray-900 dark:text-white">
        {{ t('admin.dashboard.groupDistribution') }}
      </h3>
      <div
        v-if="showMetricToggle"
        class="inline-flex rounded-lg border border-gray-200 bg-gray-50 p-0.5 dark:border-dark-700 dark:bg-dark-800"
      >
        <button
          type="button"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="metric === 'tokens'
            ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
          @click="emit('update:metric', 'tokens')"
        >
          {{ t('admin.dashboard.metricTokens') }}
        </button>
        <button
          type="button"
          class="rounded-md px-2.5 py-1 text-xs font-medium transition-colors"
          :class="metric === 'actual_cost'
            ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-700 dark:text-white'
            : 'text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200'"
          @click="emit('update:metric', 'actual_cost')"
        >
          {{ t('admin.dashboard.metricActualCost') }}
        </button>
      </div>
    </div>
    <div v-if="loading" class="flex h-48 items-center justify-center">
      <LoadingSpinner />
    </div>
    <div v-else-if="displayGroupStats.length > 0" class="flex flex-col gap-4">
      <!-- 横向占比条（.ef-progress 契约：--p 驱动刻度填充）。
           长度编码取代颜色编码 —— 环形图压成灰后条目无法区分。 -->
      <ol class="ef-dist">
        <li v-for="group in displayGroupStats" :key="'bar-' + group.group_id">
          <span class="ef-dist-name" :title="group.group_name || String(group.group_id)">
            {{ group.group_name || group.group_id }}
          </span>
          <span class="ef-dist-pct">{{ percentOf(group) }}</span>
          <span class="ef-dist-track" :style="{ '--p': shareOf(group) }"></span>
          <span class="ef-dist-val">{{ metricLabel(group) }}</span>
        </li>
      </ol>
      <div class="max-h-48 w-full min-w-0 overflow-auto">
        <table class="w-full text-xs">
          <thead>
            <tr class="text-gray-500 dark:text-gray-400">
              <th class="pb-2 text-left">{{ t('admin.dashboard.group') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.requests') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.tokens') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.actual') }}</th>
              <th v-if="showAccountCost" class="pb-2 text-right">{{ t('admin.dashboard.accountCost') }}</th>
              <th class="pb-2 text-right">{{ t('admin.dashboard.standard') }}</th>
            </tr>
          </thead>
          <tbody>
            <template v-for="group in displayGroupStats" :key="group.group_id">
              <tr
                class="border-t border-gray-100 transition-colors dark:border-dark-700"
                :class="enableBreakdown && group.group_id > 0 ? 'cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-700/40' : ''"
                @click="enableBreakdown && group.group_id > 0 && toggleBreakdown('group', group.group_id)"
              >
                <td
                  class="max-w-[100px] truncate py-1.5 font-medium"
                  :class="enableBreakdown && group.group_id > 0 ? 'text-blue-600 hover:text-blue-800 dark:text-blue-400 dark:hover:text-blue-300' : 'text-gray-900 dark:text-white'"
                  :title="group.group_name || String(group.group_id)"
                >
                  <span class="inline-flex items-center gap-1">
                    <svg v-if="enableBreakdown && group.group_id > 0 && expandedKey === `group-${group.group_id}`" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/></svg>
                    <svg v-else-if="enableBreakdown && group.group_id > 0" class="h-3 w-3 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
                    {{ group.group_name || t('admin.dashboard.noGroup') }}
                  </span>
                </td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                  {{ formatNumber(group.requests) }}
                </td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">
                  {{ formatTokens(group.total_tokens) }}
                </td>
                <td class="py-1.5 text-right text-green-600 dark:text-green-400">
                  ${{ formatCost(group.actual_cost) }}
                </td>
                <td v-if="showAccountCost" class="py-1.5 text-right text-orange-500 dark:text-orange-400">
                  ${{ formatCost(group.account_cost) }}
                </td>
                <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">
                  ${{ formatCost(group.cost) }}
                </td>
              </tr>
              <!-- User breakdown sub-rows -->
              <tr v-if="expandedKey === `group-${group.group_id}`">
                <td :colspan="distributionColspan" class="p-0">
                  <UserBreakdownSubTable
                    :items="breakdownItems"
                    :loading="breakdownLoading"
                    :show-account-cost="showAccountCost"
                  />
                </td>
              </tr>
            </template>
          </tbody>
        </table>
      </div>
    </div>
    <div
      v-else
      class="flex h-48 items-center justify-center text-sm text-gray-500 dark:text-gray-400"
    >
      {{ t('admin.dashboard.noDataAvailable') }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserBreakdownSubTable from './UserBreakdownSubTable.vue'
import type { GroupStat, UserBreakdownItem } from '@/types'
import { getUserBreakdown } from '@/api/admin/dashboard'

const { t } = useI18n()

type DistributionMetric = 'tokens' | 'actual_cost'

const props = withDefaults(defineProps<{
  groupStats: GroupStat[]
  loading?: boolean
  metric?: DistributionMetric
  showMetricToggle?: boolean
  enableBreakdown?: boolean
  showAccountCost?: boolean
  startDate?: string
  endDate?: string
  filters?: Record<string, any>
}>(), {
  loading: false,
  metric: 'tokens',
  showMetricToggle: false,
  enableBreakdown: true,
  showAccountCost: true,
})

const emit = defineEmits<{
  'update:metric': [value: DistributionMetric]
}>()

const expandedKey = ref<string | null>(null)
const breakdownItems = ref<UserBreakdownItem[]>([])
const breakdownLoading = ref(false)
const showAccountCost = computed(() => props.showAccountCost)
const distributionColspan = computed(() => showAccountCost.value ? 6 : 5)

const toggleBreakdown = async (type: string, id: number | string) => {
  const key = `${type}-${id}`
  if (expandedKey.value === key) {
    expandedKey.value = null
    return
  }
  expandedKey.value = key
  breakdownLoading.value = true
  breakdownItems.value = []
  try {
    const res = await getUserBreakdown({
      ...props.filters,
      start_date: props.startDate,
      end_date: props.endDate,
      group_id: Number(id),
    })
    breakdownItems.value = res.users || []
  } catch {
    breakdownItems.value = []
  } finally {
    breakdownLoading.value = false
  }
}

const displayGroupStats = computed(() => {
  if (!props.groupStats?.length) return []

  const metricKey = props.metric === 'actual_cost' ? 'actual_cost' : 'total_tokens'
  return [...props.groupStats].sort((a, b) => toFiniteNumber(b[metricKey]) - toFiniteNumber(a[metricKey]))
})

/** 当前度量下的原始值 */
const metricValue = (group: GroupStat): number =>
  toFiniteNumber(props.metric === 'actual_cost' ? group.actual_cost : group.total_tokens)

/** 占最大值的比例，驱动 --v / --p（0–1） */
const shareOf = (group: GroupStat): number => {
  const max = Math.max(...displayGroupStats.value.map(metricValue), 0)
  if (max <= 0) return 0
  return metricValue(group) / max
}

/** 占整体总量的百分比文本 */
const percentOf = (group: GroupStat): string => {
  const total = displayGroupStats.value.reduce((a, b) => a + metricValue(b), 0)
  if (total <= 0) return '0%'
  return `${((metricValue(group) / total) * 100).toFixed(1)}%`
}

/** 条尾显示的数值文本 */
const metricLabel = (group: GroupStat): string => {
  const v = metricValue(group)
  return props.metric === 'actual_cost' ? `${formatCost(v)}` : formatTokens(v)
}

const formatTokens = (value: number): string => {
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const formatNumber = (value: number): string => {
  return toFiniteNumber(value).toLocaleString()
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatCost = (value: number | null | undefined): string => {
  const safeValue = toFiniteNumber(value)
  if (safeValue >= 1000) {
    return (safeValue / 1000).toFixed(2) + 'K'
  } else if (safeValue >= 1) {
    return safeValue.toFixed(2)
  } else if (safeValue >= 0.01) {
    return safeValue.toFixed(3)
  }
  return safeValue.toFixed(4)
}
</script>
