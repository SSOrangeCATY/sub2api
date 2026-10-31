<template>
  <AppLayout>
    <div class="pt-user-dashboard" :aria-busy="initialLoading || refreshing">
      <section v-if="initialLoading" class="pt-dashboard-loading" role="status" :aria-label="t('common.loading')">
        <div class="pt-subsection-title"><div><span class="pt-kicker">// OVERVIEW</span><h2>{{ t('dashboard.title') }}</h2></div><LoadingSpinner size="lg" /></div>
        <p>{{ t('common.loading') }}</p>
        <div class="pt-loading-layout" aria-hidden="true"><Skeleton height="260px" /><Skeleton height="260px" /><Skeleton height="260px" /></div>
        <Skeleton height="180px" />
      </section>
      <div v-else-if="!stats" class="pt-dashboard-error" role="alert">
        <span aria-hidden="true">!</span><p>{{ t('dashboard.loadFailed') }}</p>
        <button class="ef-btn go" type="button" @click="refreshAll">{{ t('common.tryAgain') }}</button>
      </div>
      <template v-else>
        <div v-if="hasError" class="pt-dashboard-notice" role="alert"><b aria-hidden="true">!</b><span>{{ t('dashboard.loadFailed') }}</span><button type="button" :disabled="refreshing" @click="refreshAll">{{ t('common.tryAgain') }}</button></div>
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas" />
        <div class="pt-dashboard-sync" role="status"><LoadingSpinner v-if="refreshing" size="sm" /><span v-if="refreshing">{{ t('dashboard.refreshing') }}</span><span v-else-if="updatedAt">{{ t('dashboard.updatedAt', { time: updatedAt }) }}</span></div>
        <UserDashboardCharts v-model:startDate="startDate" v-model:endDate="endDate" v-model:granularity="granularity" :loading="loadingCharts" :refreshing="refreshing" :trend="trendData" :models="modelStats" @dateRangeChange="refreshRange" @granularityChange="loadCharts" @refresh="refreshAll" />
        <div class="pt-user-bottom-grid">
          <UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" :start-date="recentRange.start" :end-date="recentRange.end" />
          <UserDashboardQuickActions />
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import '@/styles/endfield/user-dashboard.css'
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore } from '@/stores/auth'
import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'
import AppLayout from '@/components/layout/AppLayout.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Skeleton from '@/components/common/Skeleton.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'
import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'
import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'

const { t } = useI18n()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null)
const initialLoading = ref(true)
const loadingStats = ref(false)
const loadingUsage = ref(false)
const loadingCharts = ref(false)
const loadingQuotas = ref(false)
const errors = ref({ stats: false, charts: false, usage: false, quotas: false })
const hasError = computed(() => Object.values(errors.value).some(Boolean))
const refreshing = computed(() => loadingStats.value || loadingCharts.value || loadingUsage.value || loadingQuotas.value)
const updatedAt = ref('')
const trendData = ref<TrendDataPoint[]>([])
const modelStats = ref<ModelStat[]>([])
const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)
const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000)))
const endDate = ref(formatDateLocalInput(new Date()))
const recentRange = ref({ start: '', end: '' })
const granularity = ref('day')
let chartRequest = 0
let usageRequest = 0

async function loadStats() {
  loadingStats.value = true
  errors.value.stats = false
  try {
    await authStore.refreshUser()
    stats.value = await usageAPI.getDashboardStats()
    updatedAt.value = new Date().toLocaleTimeString([], { hour12: false })
  } catch { errors.value.stats = true }
  finally { loadingStats.value = false }
}

async function loadCharts() {
  const request = ++chartRequest
  loadingCharts.value = true
  errors.value.charts = false
  try {
    const params = { start_date: startDate.value, end_date: endDate.value }
    const [trend, models] = await Promise.all([
      usageAPI.getDashboardTrend({ ...params, granularity: granularity.value as 'day' | 'hour' }),
      usageAPI.getDashboardModels(params)
    ])
    if (request !== chartRequest) return
    trendData.value = trend.trend || []
    modelStats.value = models.models || []
  } catch { if (request === chartRequest) errors.value.charts = true }
  finally { if (request === chartRequest) loadingCharts.value = false }
}

async function loadRecent() {
  const request = ++usageRequest
  const range = { start: startDate.value, end: endDate.value }
  loadingUsage.value = true
  errors.value.usage = false
  try {
    const result = await usageAPI.getByDateRange(range.start, range.end)
    if (request === usageRequest) {
      recentUsage.value = result.items.slice(0, 5)
      recentRange.value = range
    }
  } catch { if (request === usageRequest) errors.value.usage = true }
  finally { if (request === usageRequest) loadingUsage.value = false }
}

async function loadPlatformQuotas() {
  loadingQuotas.value = true
  errors.value.quotas = false
  try { platformQuotas.value = (await getMyPlatformQuotas()).platform_quotas ?? [] }
  catch { errors.value.quotas = true }
  finally { loadingQuotas.value = false }
}

function refreshRange() { return Promise.all([loadCharts(), loadRecent()]) }
async function refreshAll() {
  if (refreshing.value) return
  await Promise.all([loadStats(), loadCharts(), loadRecent(), loadPlatformQuotas()])
  initialLoading.value = false
}
onMounted(refreshAll)
</script>
