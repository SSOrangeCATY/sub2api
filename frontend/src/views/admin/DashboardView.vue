<template>
  <AppLayout>
    <div class="dashboard-accent space-y-6">
      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-12">
        <LoadingSpinner />
      </div>

      <template v-else-if="stats">
        <div class="pt-section-head">
          <div><p class="pt-kicker">// OVERVIEW</p><h2>{{ t('admin.dashboard.description') }}</h2></div>
          <span class="pt-kicker">{{ new Date().toLocaleDateString() }}</span>
        </div>
        <section class="pt-overview" :aria-label="t('admin.dashboard.description')">
          <div class="pt-metric pt-metric--lead">
            <p class="pt-metric-label">{{ t('admin.dashboard.todayRequests') }}<Icon name="chart" /></p>
            <strong class="pt-metric-value">{{ formatNumber(stats.today_requests) }}</strong>
            <p class="pt-metric-meta">{{ t('common.total') }} / {{ formatNumber(stats.total_requests) }}</p>
          </div>
          <div class="pt-metric pt-metric--teal">
            <p class="pt-metric-label">{{ t('admin.dashboard.todayTokens') }}<Icon name="cube" /></p>
            <strong class="pt-metric-value">{{ formatTokens(stats.today_tokens) }}</strong>
            <p class="pt-metric-meta">{{ t('admin.dashboard.totalTokens') }} / {{ formatTokens(stats.total_tokens) }}</p>
          </div>
          <div class="pt-metric pt-metric--purple">
            <p class="pt-metric-label">{{ t('admin.dashboard.todayCost') }} / {{ t('admin.dashboard.actual') }}<Icon name="dollar" /></p>
            <strong class="pt-metric-value">${{ formatCost(stats.today_actual_cost) }}</strong>
            <div class="pt-metric-meta pt-cost-detail">
              <span>{{ t('admin.dashboard.accountCost') }}<b>${{ formatCost(stats.today_account_cost) }}</b></span>
              <span>{{ t('admin.dashboard.standard') }}<b>${{ formatCost(stats.today_cost) }}</b></span>
            </div>
          </div>
          <TokenComposition :input="stats.today_input_tokens || 0" :output="stats.today_output_tokens || 0" :creation="stats.today_cache_creation_tokens || 0" :read="stats.today_cache_read_tokens || 0" />
        </section>
        <section class="pt-health" :aria-label="t('admin.dashboard.performance')">
          <div class="pt-health-cell">
            <p class="pt-health-label"><Icon name="server" />{{ t('admin.dashboard.accounts') }}</p>
            <p class="pt-health-value">{{ formatNumber(stats.total_accounts) }}</p>
            <div class="pt-health-foot">
              <span class="pt-status">{{ stats.normal_accounts }} {{ t('common.active') }}</span>
              <span v-if="stats.error_accounts > 0" class="pt-status pt-status--warn">{{ stats.error_accounts }} {{ t('common.error') }}</span>
            </div>
          </div>
          <div class="pt-health-cell">
            <p class="pt-health-label"><Icon name="key" />{{ t('admin.dashboard.apiKeys') }}</p>
            <p class="pt-health-value">{{ formatNumber(stats.total_api_keys) }}</p>
            <div class="pt-health-foot"><span class="pt-status pt-status--info">{{ stats.active_api_keys }} {{ t('common.active') }}</span></div>
          </div>
          <div class="pt-health-cell">
            <p class="pt-health-label"><Icon name="bolt" />{{ t('admin.dashboard.performance') }}</p>
            <p class="pt-health-value pt-blue">{{ formatTokens(stats.rpm) }} <small>RPM</small></p>
            <p class="pt-health-foot">{{ formatTokens(stats.tpm) }} TPM</p>
          </div>
          <div class="pt-health-cell">
            <p class="pt-health-label"><Icon name="clock" />{{ t('admin.dashboard.avgResponse') }}</p>
            <p class="pt-health-value">{{ formatDuration(stats.average_duration_ms) }}</p>
            <p class="pt-health-foot">{{ stats.active_users }} {{ t('admin.dashboard.activeUsers') }}</p>
          </div>
        </section>
        <div class="pt-user-summary">
          <div><p>{{ t('admin.dashboard.users') }}</p><strong>{{ formatNumber(stats.total_users) }} <small class="pt-teal text-sm">+{{ stats.today_new_users }} {{ t('admin.dashboard.newUsersToday') }}</small></strong></div>
          <div><p>{{ t('admin.dashboard.totalCost') }} / {{ t('admin.dashboard.actual') }}</p><strong class="pt-purple">${{ formatCost(stats.total_actual_cost) }}</strong></div>
          <div><p>{{ t('admin.dashboard.accountCost') }} / {{ t('admin.dashboard.standard') }}</p><strong class="text-base">${{ formatCost(stats.total_account_cost) }} / ${{ formatCost(stats.total_cost) }}</strong></div>
        </div>
        <div class="pt-actions">
          <h2>{{ t('admin.dashboard.quickActions') }}</h2>
          <button v-if="canUseBatchImage" type="button" class="pt-action" @click="router.push('/batch-image')">
            <Icon name="sparkles" /><span><b>{{ t('admin.dashboard.batchImage') }}</b><small>{{ t('admin.dashboard.batchImageDesc') }}</small></span><Icon name="chevronRight" size="sm" />
          </button>
          <button type="button" class="pt-action" @click="router.push('/admin/groups')">
            <Icon name="grid" /><span><b>{{ t('admin.dashboard.groupPricing') }}</b><small>{{ t('admin.dashboard.groupPricingDesc') }}</small></span><Icon name="chevronRight" size="sm" />
          </button>
        </div>

        <!-- Charts Section -->
        <div class="space-y-6">
          <!-- Date Range Filter -->
          <div class="pt-filter">
            <div class="flex flex-wrap items-center gap-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.timeRange') }}:</span
                >
                <DateRangePicker
                  v-model:start-date="startDate"
                  v-model:end-date="endDate"
                  @change="onDateRangeChange"
                />
              </div>
              <button type="button" @click="loadDashboardStats" :disabled="chartsLoading" class="ef-btn go">
                {{ t('common.refresh') }}
              </button>
              <div class="ml-auto flex items-center gap-2">
                <span class="text-sm font-medium text-gray-700 dark:text-gray-300"
                  >{{ t('admin.dashboard.granularity') }}:</span
                >
                <div class="flex gap-1" role="group" :aria-label="t('admin.dashboard.granularity')">
                  <button
                    v-for="option in granularityOptions"
                    :key="option.value"
                    type="button"
                    class="ef-state px-3 py-1.5"
                    :class="granularity === option.value ? 'is-alert' : 'is-draft'"
                    :aria-pressed="granularity === option.value"
                    @click="setGranularity(option.value)"
                  >{{ option.label }}</button>
                </div>
              </div>
            </div>
          </div>

          <!-- Charts Grid -->
          <div class="pt-chart-grid">
            <ModelDistributionChart
              :model-stats="modelStats"
              :enable-ranking-view="true"
              :ranking-items="rankingItems"
              :ranking-total-actual-cost="rankingTotalActualCost"
              :ranking-total-requests="rankingTotalRequests"
              :ranking-total-tokens="rankingTotalTokens"
              :loading="chartsLoading"
              :ranking-loading="rankingLoading"
              :ranking-error="rankingError"
              :start-date="startDate"
              :end-date="endDate"
              @ranking-click="goToUserUsage"
            />
            <TokenUsageTrend :trend-data="trendData" :loading="chartsLoading" />
          </div>

          <!-- User Usage Trend (Full Width) -->
          <div class="card pt-chart-panel pt-chart-panel--users">
            <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
              <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.dashboard.recentUsage') }} (Top 12)</h3>
              <div class="flex gap-1" role="group" :aria-label="t('admin.dashboard.recentUsage')">
                <button v-for="metric in (['tokens', 'actual_cost'] as const)" :key="metric" type="button"
                  class="ef-state px-2 py-1" :class="userTrendMetric === metric ? 'is-alert' : 'is-draft'"
                  :aria-pressed="userTrendMetric === metric" @click="setUserTrendMetric(metric)">
                  {{ t(metric === 'tokens' ? 'admin.dashboard.tokens' : 'admin.dashboard.actualSpending') }}
                </button>
              </div>
            </div>
            <div class="h-96 sm:h-80">
              <div v-if="userTrendLoading" class="flex h-full items-center justify-center">
                <LoadingSpinner size="md" />
              </div>
              <Line v-else-if="userTrendChartData" :data="userTrendChartData" :options="lineOptions" />
              <div
                v-else
                class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.dashboard.noDataAvailable') }}
              </div>
            </div>
          </div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import TokenComposition from '@/components/charts/TokenComposition.vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores/app'

const { t } = useI18n()
import { adminAPI } from '@/api/admin'
import type {
  DashboardStats,
  TrendDataPoint,
  ModelStat,
  UserUsageTrendPoint,
  UserSpendingRankingItem
} from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ModelDistributionChart from '@/components/charts/ModelDistributionChart.vue'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'

import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'
import { useChartPalette, efLineStyle, efLineLegendLabels } from '@/utils/chartTheme'


const p = useChartPalette()
// Register Chart.js components
ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Tooltip,
  Legend,
  Filler
)

const appStore = useAppStore()
const router = useRouter()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
const stats = ref<DashboardStats | null>(null)
const loading = ref(false)
const chartsLoading = ref(false)
const userTrendLoading = ref(false)
const rankingLoading = ref(false)
const rankingError = ref(false)

// Chart data
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const userTrend = ref<UserUsageTrendPoint[]>([])
const userTrendMetric = ref<'tokens' | 'actual_cost'>('tokens')
const rankingItems = ref<UserSpendingRankingItem[]>([])
const rankingTotalActualCost = ref(0)
const rankingTotalRequests = ref(0)
const rankingTotalTokens = ref(0)
let chartLoadSeq = 0
let usersTrendLoadSeq = 0
let rankingLoadSeq = 0
const rankingLimit = 12

// Helper function to format date in local timezone
const formatLocalDate = (date: Date): string => {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end)
  }
}

// Date range
const granularity = ref<'day' | 'hour'>('hour')
const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)

// Both options stay visible so the selected time unit is explicit.
const granularityOptions = computed(() => [
  { value: 'day' as const, label: t('admin.dashboard.day') },
  { value: 'hour' as const, label: t('admin.dashboard.hour') }
])

// Dark mode detection
const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

// Chart colors
const chartColors = computed(() => ({
  text: isDarkMode.value ? p.ink2 : p.ink2,
  grid: isDarkMode.value ? p.rule : p.rule
}))

// Line chart options (for user trend chart)
const lineOptions = computed(() => ({
  responsive: true,
  animation: { duration: p.motionDuration },
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: efLineLegendLabels(p)
    },
    tooltip: {
      itemSort: (a: any, b: any) => {
        const aValue = typeof a?.raw === 'number' ? a.raw : Number(a?.parsed?.y ?? 0)
        const bValue = typeof b?.raw === 'number' ? b.raw : Number(b?.parsed?.y ?? 0)
        return bValue - aValue
      },
      callbacks: {
        label: (context: any) => {
          return `${context.dataset.label}: ${formatUserTrendValue(Number(context.raw))}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        }
      }
    },
    y: {
      grid: {
        color: chartColors.value.grid
      },
      ticks: {
        color: chartColors.value.text,
        font: {
          size: 10
        },
        callback: (value: string | number) => formatUserTrendValue(Number(value))
      }
    }
  }
}))

// User trend chart data
const userTrendChartData = computed(() => {
  if (!userTrend.value?.length) return null

  const getDisplayName = (point: UserUsageTrendPoint): string => {
    const username = point.username?.trim()
    if (username) {
      return username
    }

    const email = point.email?.trim()
    if (email) {
      return email
    }

    return t('admin.redeem.userPrefix', { id: point.user_id })
  }

  // Group by user_id to avoid merging different users with the same display name
  const userGroups = new Map<number, { name: string; data: Map<string, number> }>()
  const allDates = new Set<string>()

  userTrend.value.forEach((point) => {
    allDates.add(point.date)
    const key = point.user_id
    if (!userGroups.has(key)) {
      userGroups.set(key, { name: getDisplayName(point), data: new Map() })
    }
    userGroups.get(key)!.data.set(point.date, userTrendMetric.value === 'tokens' ? point.tokens : point.actual_cost)
  })

  const sortedDates = Array.from(allDates).sort()

  const datasets = Array.from(userGroups.values()).map((group, idx) => ({
    label: group.name,
    data: sortedDates.map((date) => group.data.get(date) || 0),
    ...efLineStyle(idx, p)
  }))

  return {
    labels: sortedDates,
    datasets
  }
})

// Format helpers
const formatUserTrendValue = (value: number): string =>
  userTrendMetric.value === 'tokens' ? formatTokens(value) : `$${formatCost(value)}`

const formatTokens = (value: number | undefined): string => {
  if (value === undefined || value === null) return '0'
  if (value >= 1_000_000_000) {
    return `${(value / 1_000_000_000).toFixed(2)}B`
  } else if (value >= 1_000_000) {
    return `${(value / 1_000_000).toFixed(2)}M`
  } else if (value >= 1_000) {
    return `${(value / 1_000).toFixed(2)}K`
  }
  return value.toLocaleString()
}

const toFiniteNumber = (value: unknown): number => {
  const numberValue = Number(value)
  return Number.isFinite(numberValue) ? numberValue : 0
}

const formatNumber = (value: number | null | undefined): string => {
  return toFiniteNumber(value).toLocaleString()
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

const formatDuration = (ms: number): string => {
  if (ms >= 1000) {
    return `${(ms / 1000).toFixed(2)}s`
  }
  return `${Math.round(ms)}ms`
}

const goToUserUsage = (item: UserSpendingRankingItem) => {
  void router.push({
    path: '/admin/usage',
    query: {
      user_id: String(item.user_id),
      start_date: startDate.value,
      end_date: endDate.value
    }
  })
}

// Date range change handler
const onDateRangeChange = (range: {
  startDate: string
  endDate: string
  preset: string | null
}) => {
  // Auto-select granularity based on date range
  const start = new Date(range.startDate)
  const end = new Date(range.endDate)
  const daysDiff = Math.ceil((end.getTime() - start.getTime()) / (1000 * 60 * 60 * 24))

  // If range is 1 day, use hourly granularity
  if (daysDiff <= 1) {
    granularity.value = 'hour'
  } else {
    granularity.value = 'day'
  }

  loadChartData()
}

// Load data
const loadDashboardSnapshot = async (includeStats: boolean) => {
  const currentSeq = ++chartLoadSeq
  if (includeStats && !stats.value) {
    loading.value = true
  }
  chartsLoading.value = true
  try {
    const response = await adminAPI.dashboard.getSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      include_stats: includeStats,
      include_trend: true,
      include_model_stats: true,
      include_group_stats: false,
      include_users_trend: false
    })
    if (currentSeq !== chartLoadSeq) return
    if (includeStats && response.stats) {
      stats.value = response.stats
    }
    trendData.value = response.trend || []
    modelStats.value = response.models || []
  } catch (error) {
    if (currentSeq !== chartLoadSeq) return
    appStore.showError(t('admin.dashboard.failedToLoad'))
    console.error('Error loading dashboard snapshot:', error)
  } finally {
    if (currentSeq === chartLoadSeq) {
      loading.value = false
      chartsLoading.value = false
    }
  }
}

const setUserTrendMetric = (metric: 'tokens' | 'actual_cost') => {
  if (userTrendMetric.value === metric) return
  userTrendMetric.value = metric
  userTrend.value = []
  loadUsersTrend()
}

const loadUsersTrend = async () => {
  const currentSeq = ++usersTrendLoadSeq
  userTrendLoading.value = true
  try {
    const response = await adminAPI.dashboard.getUserUsageTrend({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      limit: 12,
      metric: userTrendMetric.value
    })
    if (currentSeq !== usersTrendLoadSeq) return
    userTrend.value = response.trend || []
  } catch (error) {
    if (currentSeq !== usersTrendLoadSeq) return
    console.error('Error loading users trend:', error)
    userTrend.value = []
  } finally {
    if (currentSeq === usersTrendLoadSeq) {
      userTrendLoading.value = false
    }
  }
}

const loadUserSpendingRanking = async () => {
  const currentSeq = ++rankingLoadSeq
  rankingLoading.value = true
  rankingError.value = false
  try {
    const response = await adminAPI.dashboard.getUserSpendingRanking({
      start_date: startDate.value,
      end_date: endDate.value,
      limit: rankingLimit
    })
    if (currentSeq !== rankingLoadSeq) return
    rankingItems.value = response.ranking || []
    rankingTotalActualCost.value = response.total_actual_cost || 0
    rankingTotalRequests.value = response.total_requests || 0
    rankingTotalTokens.value = response.total_tokens || 0
  } catch (error) {
    if (currentSeq !== rankingLoadSeq) return
    console.error('Error loading user spending ranking:', error)
    rankingItems.value = []
    rankingTotalActualCost.value = 0
    rankingTotalRequests.value = 0
    rankingTotalTokens.value = 0
    rankingError.value = true
  } finally {
    if (currentSeq === rankingLoadSeq) {
      rankingLoading.value = false
    }
  }
}

const loadDashboardStats = async () => {
  await Promise.all([
    loadDashboardSnapshot(true),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const loadChartData = async () => {
  await Promise.all([
    loadDashboardSnapshot(false),
    loadUsersTrend(),
    loadUserSpendingRanking()
  ])
}

const setGranularity = (value: 'day' | 'hour') => {
  if (granularity.value === value) return
  granularity.value = value
  void loadChartData()
}

onMounted(() => {
  void refreshBatchImageAccess()
  loadDashboardStats()
})
</script>

<style scoped>
.dashboard-accent .ef-state.is-alert {
  border-color: var(--ef-on-signal);
}
.dashboard-accent button.ef-state {
  cursor: pointer;
  padding: 0.375rem 0.75rem;
}
.dashboard-accent .ef-state[aria-hidden='true'] {
  display: flex;
  padding: 0;
}
.dashboard-accent .ef-btn.go {
  --fill: var(--ef-signal);
  color: var(--ef-on-signal);
}
.dashboard-accent .ef-btn.go:focus-visible {
  outline-color: var(--ef-on-signal);
}
</style>
