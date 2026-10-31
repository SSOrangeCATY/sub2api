<template>
  <div class="user-chart-actions space-y-6">
    <div class="pt-subsection-title"><div><span class="pt-kicker">// USAGE ANALYSIS</span><h2>{{ t('dashboard.usageAnalysis') }}</h2></div></div>
    <!-- Date Range Filter -->
    <div class="pt-filter">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.timeRange') }}:</span>
          <DateRangePicker :start-date="startDate" :end-date="endDate" @update:startDate="$emit('update:startDate', $event)" @update:endDate="$emit('update:endDate', $event)" @change="$emit('dateRangeChange', $event)" />
        </div>
        <button type="button" @click="$emit('refresh')" :disabled="loading || refreshing" :aria-busy="loading || refreshing" class="ef-btn go">
          <LoadingSpinner v-if="loading || refreshing" size="sm" color="white" />
          {{ t('common.refresh') }}
        </button>
        <div class="ml-auto flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.granularity') }}:</span>
          <div class="flex gap-1" role="group" :aria-label="t('dashboard.granularity')">
            <button v-for="unit in (['day', 'hour'] as const)" :key="unit" type="button"
              class="ef-state" :class="granularity === unit ? 'is-alert' : 'is-draft'"
              :aria-pressed="granularity === unit" @click="setGranularity(unit)">
              {{ t(unit === 'day' ? 'dashboard.day' : 'dashboard.hour') }}
            </button>
          </div>
        </div>
      </div>
    </div>

    <!-- Charts Grid -->
    <div class="pt-chart-grid">
      <!-- Model Distribution Chart -->
      <div class="card pt-chart-panel pt-chart-panel--distribution relative overflow-hidden" :aria-busy="loading">
        <div v-if="loading" class="pt-chart-loading">
          <LoadingSpinner size="sm" /><span>{{ t('common.loading') }}</span>
        </div>
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white"><span class="pt-kicker mb-1 block">// MODEL MIX</span>{{ t('dashboard.modelDistribution') }}</h3>
        <!-- 横向占比条（.ef-progress 契约）。长度编码取代颜色编码。 -->
        <ol v-if="modelBars.length" class="ef-dist">
          <li v-for="(m, index) in modelBars" :key="'bar-' + m.model" :style="{ '--series-color': p.steps[index % p.steps.length] }">
            <span class="ef-dist-name" :title="m.model">{{ m.model }}</span>
            <span class="ef-dist-pct">{{ m.pct }}</span>
            <span class="ef-dist-track" :style="{ '--p': m.share }"></span>
            <span class="ef-dist-val">{{ m.label }}</span>
          </li>
        </ol>
        <div v-else class="flex h-32 items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
        <div v-if="modelBars.length" class="mt-4 max-h-48 w-full min-w-0 overflow-auto">
          <table class="w-full text-xs">
            <thead>
              <tr class="text-gray-500 dark:text-gray-400">
                <th class="pb-2 text-left">{{ t('dashboard.model') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.requests') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.tokens') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.actual') }}</th>
                <th class="pb-2 text-right">{{ t('dashboard.standard') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="model in models"
                :key="model.model"
                class="border-t border-gray-100 dark:border-dark-700"
              >
                <td class="max-w-[100px] truncate py-1.5 font-medium text-gray-900 dark:text-white" :title="model.model">
                  {{ model.model }}
                </td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatNumber(model.requests) }}</td>
                <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatTokens(model.total_tokens) }}</td>
                <td class="py-1.5 text-right text-green-600 dark:text-green-400">${{ formatCost(model.actual_cost) }}</td>
                <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">${{ formatCost(model.cost) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <!-- Token Usage Trend Chart -->
      <TokenUsageTrend :trend-data="trend" :loading="loading" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useChartPalette } from '@/utils/chartTheme'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { TrendDataPoint, ModelStat } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler } from 'chart.js'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{ loading: boolean, refreshing?: boolean, startDate: string, endDate: string, granularity: string, trend: TrendDataPoint[], models: ModelStat[] }>()
const emit = defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'dateRangeChange', 'granularityChange', 'refresh'])
const p = useChartPalette()
const { t } = useI18n()

const setGranularity = (unit: 'day' | 'hour') => {
  if (props.granularity === unit) return
  emit('update:granularity', unit)
  emit('granularityChange')
}

/** 模型分布横条：按 token 量排序，--p 用占最大值的比例 */
const modelBars = computed(() => {
  const list = props.models ?? []
  if (!list.length) return []
  const sorted = [...list].sort((a: ModelStat, b: ModelStat) => (b.total_tokens || 0) - (a.total_tokens || 0))
  const max = Math.max(...sorted.map((m: ModelStat) => m.total_tokens || 0), 0)
  const total = sorted.reduce((a: number, b: ModelStat) => a + (b.total_tokens || 0), 0)
  return sorted.map((m: ModelStat) => ({
    model: m.model,
    share: max > 0 ? (m.total_tokens || 0) / max : 0,
    pct: total > 0 ? `${(((m.total_tokens || 0) / total) * 100).toFixed(1)}%` : '0%',
    label: formatTokens(m.total_tokens || 0)
  }))
})
</script>

<style scoped>
.user-chart-actions .ef-state.is-alert {
  border-color: var(--ef-on-signal);
}
.user-chart-actions button.ef-state {
  padding: 0.375rem 0.75rem;
  cursor: pointer;
}
.user-chart-actions .ef-btn.go {
  --fill: var(--ef-signal);
  color: var(--ef-on-signal);
}
.user-chart-actions .ef-btn.go:focus-visible {
  outline-color: var(--ef-on-signal);
}
</style>
