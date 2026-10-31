<template>
  <div class="pt-section-head pt-user-heading"><div><p class="pt-kicker">// OVERVIEW</p><h2>{{ t('dashboard.title') }}</h2></div><time class="pt-user-date">{{ new Date().toLocaleDateString() }}</time></div>
  <section class="pt-overview" :aria-label="t('dashboard.title')">
    <div class="pt-metric pt-metric--lead">
      <p class="pt-metric-label">{{ t('dashboard.todayRequests') }}<Icon name="chart" /></p>
      <strong class="pt-metric-value">{{ formatNumber(stats?.today_requests || 0) }}</strong>
      <p class="pt-metric-meta">{{ t('common.total') }} / {{ formatNumber(stats?.total_requests || 0) }}</p>
    </div>
    <div class="pt-metric pt-metric--teal">
      <p class="pt-metric-label">{{ t('dashboard.todayTokens') }}<Icon name="cube" /></p>
      <strong class="pt-metric-value">{{ formatTokens(stats?.today_tokens || 0) }}</strong>
      <p class="pt-metric-meta pt-token-breakdown"><span>{{ t('dashboard.input') }} {{ formatTokens(stats?.today_input_tokens || 0) }}</span><span>{{ t('dashboard.output') }} {{ formatTokens(stats?.today_output_tokens || 0) }}</span><span>{{ t('dashboard.cache') }} {{ formatTokens((stats?.today_cache_creation_tokens || 0) + (stats?.today_cache_read_tokens || 0)) }}</span></p>
    </div>
    <div class="pt-metric pt-metric--purple">
      <p class="pt-metric-label">{{ t('dashboard.todayCost') }} / {{ t('dashboard.actual') }}<Icon name="dollar" /></p>
      <strong class="pt-metric-value">${{ formatCost(stats?.today_actual_cost || 0) }}</strong>
      <p class="pt-metric-meta">{{ t('dashboard.standard') }} / ${{ formatCost(stats?.today_cost || 0) }}</p>
    </div>
    <TokenComposition :input="stats?.today_input_tokens || 0" :output="stats?.today_output_tokens || 0" :creation="stats?.today_cache_creation_tokens || 0" :read="stats?.today_cache_read_tokens || 0" />
  </section>
  <div class="pt-user-detail-grid" :class="{ 'pt-user-detail-grid--single': isSimple || !platformCards.length }">
  <div class="pt-account-summary">
  <div class="pt-subsection-title"><span class="pt-kicker">// ACCOUNT</span><h3>{{ t('dashboard.accountOverview') }}</h3></div>
  <section class="pt-health" :class="{ 'pt-health--simple': isSimple }" :aria-label="t('dashboard.performance')">
    <div v-if="!isSimple" class="pt-health-cell"><p class="pt-health-label"><Icon name="dollar" />{{ t('dashboard.balance') }}</p><p class="pt-health-value pt-teal">${{ formatBalance(balance) }}</p><p class="pt-health-foot"><span class="pt-status">{{ t('common.available') }}</span></p></div>
    <div class="pt-health-cell"><p class="pt-health-label"><Icon name="key" />{{ t('dashboard.apiKeys') }}</p><p class="pt-health-value">{{ stats?.total_api_keys || 0 }}</p><p class="pt-health-foot"><span class="pt-status pt-status--info">{{ stats?.active_api_keys || 0 }} {{ t('common.active') }}</span></p></div>
    <div class="pt-health-cell"><p class="pt-health-label"><Icon name="bolt" />{{ t('dashboard.performance') }}</p><p class="pt-health-value pt-blue">{{ formatTokens(stats?.rpm || 0) }} <small>RPM</small></p><p class="pt-health-foot">{{ formatTokens(stats?.tpm || 0) }} TPM</p></div>
    <div class="pt-health-cell"><p class="pt-health-label"><Icon name="clock" />{{ t('dashboard.avgResponse') }}</p><p class="pt-health-value">{{ formatDuration(stats?.average_duration_ms || 0) }}</p><p class="pt-health-foot">{{ t('dashboard.averageTime') }}</p></div>
  </section>
  <div class="pt-user-summary">
    <div><p>{{ t('dashboard.totalTokens') }}</p><strong>{{ formatTokens(stats?.total_tokens || 0) }}</strong><p>{{ t('dashboard.input') }} {{ formatTokens(stats?.total_input_tokens || 0) }} / {{ t('dashboard.output') }} {{ formatTokens(stats?.total_output_tokens || 0) }} / {{ t('dashboard.cache') }} {{ formatTokens((stats?.total_cache_creation_tokens || 0) + (stats?.total_cache_read_tokens || 0)) }}</p></div>
    <div><p>{{ t('common.total') }} / {{ t('dashboard.actual') }}</p><strong class="pt-purple">${{ formatCost(stats?.total_actual_cost || 0) }}</strong></div>
    <div><p>{{ t('common.total') }} / {{ t('dashboard.standard') }}</p><strong>${{ formatCost(stats?.total_cost || 0) }}</strong></div>
  </div>
  </div>

  <!-- Row 3: Per-platform breakdown -->
  <section v-if="!isSimple && platformCards.length > 0" class="pt-platforms">
    <div class="pt-subsection-title">
      <div><span class="pt-kicker">// PLATFORMS</span><h3>{{ t('dashboard.platformBreakdown') }}</h3></div>
      <span class="pt-platform-count">
        {{ t('dashboard.platformCount', { count: platformCount }) }}
      </span>
    </div>
    <div class="pt-platform-list">
      <div
        v-for="item in platformCards"
        :key="item.platform"
        data-testid="platform-card"
        :data-platform="item.platform"
        :class="[
          'pt-platform-record',
          item.isOther
            ? 'border-dashed border-gray-300 bg-gray-50 dark:border-dark-500 dark:bg-dark-700/30'
            : 'border-gray-200 dark:border-dark-600'
        ]"
      >
        <div class="flex items-center justify-between">
          <span class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ item.isOther ? t('dashboard.platformOther') : platformLabel(item.platform) }}
          </span>
          <span class="font-mono text-sm text-purple-600 dark:text-purple-400" :title="t('dashboard.actual')">
            ${{ formatCost(item.total_actual_cost) }}
          </span>
        </div>
        <div class="mt-2 space-y-1 text-xs">
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.todayCost') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">${{ formatCost(item.today_actual_cost) }}</span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.requests') }}</span>
            <span class="font-mono text-gray-700 dark:text-gray-300">
              {{ item.total_requests > 0 ? formatNumber(item.total_requests) : '-' }}
            </span>
          </div>
          <div class="flex items-center justify-between">
            <span class="text-gray-500 dark:text-gray-400">{{ t('dashboard.tokens') }}</span>
            <span class="font-mono text-gray-700 dark:text-gray-300">
              {{ item.total_tokens > 0 ? formatTokens(item.total_tokens) : '-' }}
            </span>
          </div>
        </div>

        <!-- Quota 区：仅当 quota 配置存在、非 __other__ 且至少有一个窗口配了 limit 时显示 -->
        <div v-if="hasAnyLimit(item.quota) && !item.isOther" class="mt-3 space-y-1.5 border-t border-gray-200 pt-2 dark:border-dark-700">
          <p class="text-[10px] uppercase tracking-wide text-gray-400">
            {{ t('dashboard.platformQuota.title') }}
          </p>
          <template v-for="w in (['daily', 'weekly', 'monthly'] as const)" :key="w">
            <div v-if="quotaVal(item.quota, `${w}_limit_usd`) != null" class="space-y-0.5">
              <!-- limit=0：完全禁用 -->
              <template v-if="(quotaVal(item.quota, `${w}_limit_usd`) as number) === 0">
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-600 dark:text-gray-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="font-mono pt-amber">{{ t('dashboard.platformQuota.disabled') }}</span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div class="h-full w-full rounded-full pt-quota-critical" />
                </div>
              </template>
              <!-- limit>0：正常用量进度条 -->
              <template v-else>
                <div class="flex items-center justify-between text-xs">
                  <span class="text-gray-600 dark:text-gray-300">{{ t(`dashboard.platformQuota.${w}`) }}</span>
                  <span class="font-mono text-gray-700 dark:text-gray-200">
                    ${{ formatUsd((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0) }} / ${{ formatUsd(quotaVal(item.quota, `${w}_limit_usd`) as number) }}
                  </span>
                </div>
                <div class="h-1.5 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
                  <div
                    class="h-full rounded-full transition-all"
                    :class="quotaBarClass(calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number))"
                    :style="{ width: calcPercent((quotaVal(item.quota, `${w}_usage_usd`) as number) ?? 0, quotaVal(item.quota, `${w}_limit_usd`) as number) + '%' }"
                  />
                </div>
                <p v-if="quotaVal(item.quota, `${w}_window_resets_at`)" class="text-[10px] text-gray-400">
                  {{ t('dashboard.platformQuota.resetsAt', { time: formatResetTime(quotaVal(item.quota, `${w}_window_resets_at`) as string) }) }}
                </p>
              </template>
            </div>
          </template>
        </div>
      </div>
    </div>
  </section>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import TokenComposition from '@/components/charts/TokenComposition.vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { PlatformDashboardStats, UserDashboardStats as UserStatsType } from '@/api/usage'
import type { PlatformQuotaItem } from '@/types'

interface FusedPlatformCard {
  platform: string
  total_actual_cost: number
  today_actual_cost: number
  total_requests: number
  total_tokens: number
  isOther?: boolean
  quota?: PlatformQuotaItem
}

const props = defineProps<{
  stats: UserStatsType
  balance: number
  isSimple: boolean
  platformQuotas?: PlatformQuotaItem[] | null
}>()
const { t } = useI18n()

const PLATFORM_LABELS: Record<string, string> = {
  anthropic: 'Claude',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  grok: 'Grok',
  kimi: 'Kimi',
  zhipu: 'Zhipu GLM',
  deepseek: 'DeepSeek',
  minimax: 'MiniMax',
  typesafe: 'TypeSafe / Jev',
}

const platformLabel = (p: string) => PLATFORM_LABELS[p] ?? p

// 处理"各平台之和 < 总值"的差值：后端按平台聚合时过滤了无法归属平台的行
// （group 与 account 都缺 platform）。这里把差值作为"其他"卡片显式展示，
// 避免 Row 1 总值与 Row 3 平台拆分加总对不上、用户困惑。
const OTHER_THRESHOLD = 0.0001
const platformCards = computed<FusedPlatformCard[]>(() => {
  // 建立 by_platform Map
  const byPlat = new Map<string, PlatformDashboardStats>()
  for (const item of props.stats?.by_platform ?? []) byPlat.set(item.platform, item)

  // 建立 quota Map。三档全空的记录不产生卡片，挂到卡片上也不渲染配额区。
  const byQuota = new Map<string, PlatformQuotaItem>()
  for (const q of props.platformQuotas ?? []) byQuota.set(q.platform, q)

  // 卡片集合 = 有用量的平台 ∪ 至少配置了一档限额的平台。
  // 三档全空的限额记录等价于不限额，不单独产生卡片。
  // 后端 by_platform / quota 接口均不会返回 platform='__other__'，
  // 无需显式排除；__other__ 由下方差值补差逻辑单独追加。
  const platforms = new Set<string>(byPlat.keys())
  for (const [platform, q] of byQuota) {
    if (hasAnyLimit(q)) platforms.add(platform)
  }

  const PLATFORM_ORDER = ['anthropic', 'openai', 'gemini', 'antigravity', 'grok', 'typesafe']
  const cards: FusedPlatformCard[] = []

  for (const p of platforms) {
    const stat = byPlat.get(p)
    cards.push({
      platform: p,
      total_actual_cost: stat?.total_actual_cost ?? 0,
      today_actual_cost: stat?.today_actual_cost ?? 0,
      total_requests: stat?.total_requests ?? 0,
      total_tokens: stat?.total_tokens ?? 0,
      quota: byQuota.get(p),
    })
  }

  // 排序：按 PLATFORM_ORDER，未知平台按名称排序
  cards.sort((a, b) => {
    const ai = PLATFORM_ORDER.indexOf(a.platform)
    const bi = PLATFORM_ORDER.indexOf(b.platform)
    if (ai === -1 && bi === -1) return a.platform.localeCompare(b.platform)
    if (ai === -1) return 1
    if (bi === -1) return -1
    return ai - bi
  })

  // __other__ 补差逻辑：只对 by_platform 有 usage 数据的总和计算
  const total = props.stats?.total_actual_cost ?? 0
  const today = props.stats?.today_actual_cost ?? 0
  const sumTotal = cards.reduce((s, c) => s + c.total_actual_cost, 0)
  const sumToday = cards.reduce((s, c) => s + c.today_actual_cost, 0)
  const diffTotal = Math.max(0, total - sumTotal)
  const diffToday = Math.max(0, today - sumToday)

  if (diffTotal > OTHER_THRESHOLD || diffToday > OTHER_THRESHOLD) {
    cards.push({
      platform: '__other__',
      total_actual_cost: diffTotal,
      today_actual_cost: diffToday,
      total_requests: 0,
      total_tokens: 0,
      isOther: true,
    })
  }

  return cards
})

// 标题右侧的平台计数 = 实际渲染的平台卡片数，不含"其他"差额卡。
const platformCount = computed(() => platformCards.value.filter((c) => !c.isOther).length)

// Quota helpers

type QuotaWindow = 'daily' | 'weekly' | 'monthly'
type QuotaField = `${QuotaWindow}_limit_usd` | `${QuotaWindow}_usage_usd` | `${QuotaWindow}_window_resets_at`

function quotaVal(q: PlatformQuotaItem | undefined, key: QuotaField): PlatformQuotaItem[QuotaField] {
  return q?.[key]
}

function hasAnyLimit(q: PlatformQuotaItem | undefined): boolean {
  if (!q) return false
  return q.daily_limit_usd != null || q.weekly_limit_usd != null || q.monthly_limit_usd != null
}

function calcPercent(usage: number, limit: number): number {
  if (!limit || limit <= 0) return 0
  return Math.min(100, Math.max(0, Math.round((usage / limit) * 100)))
}

function quotaBarClass(p: number): string {
  if (p >= 95) return 'pt-quota-critical'
  if (p >= 75) return 'pt-quota-warn'
  return 'pt-quota-ok'
}

// 与 formatBalance 一致使用 Intl.NumberFormat 做半偶舍入，避免 toFixed 在不同 JS 引擎
// 下偶发截断而非四舍五入（与后端展示精度不一致）。
const usdFormatter = new Intl.NumberFormat('en-US', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 2,
})
function formatUsd(n: number): string {
  if (!Number.isFinite(n)) return '0.00'
  return usdFormatter.format(n)
}

function formatResetTime(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleString(undefined, {
    month: 'numeric',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  })
}

const formatBalance = (b: number) =>
  new Intl.NumberFormat('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2
  }).format(b)

const formatNumber = (n: number) => n.toLocaleString()
const formatCost = (c: number) => c.toFixed(4)
const formatTokens = (t: number) => {
  if (t >= 1_000_000) return `${(t / 1_000_000).toFixed(1)}M`
  if (t >= 1000) return `${(t / 1000).toFixed(1)}K`
  return t.toString()
}
const formatDuration = (ms: number) => ms >= 1000 ? `${(ms / 1000).toFixed(2)}s` : `${ms.toFixed(0)}ms`
</script>
