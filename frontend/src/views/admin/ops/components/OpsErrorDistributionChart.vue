<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsErrorDistributionResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  data: OpsErrorDistributionResponse | null
  loading: boolean
}

const props = defineProps<Props>()
const emit = defineEmits<{
  (e: 'openDetails'): void
}>()
const { t } = useI18n()

const totalSlaErrors = computed(() =>
  (props.data?.items ?? []).reduce((total, item) => total + Number(item.sla || 0), 0)
)

const hasData = computed(() => totalSlaErrors.value > 0)

const state = computed<ChartState>(() => {
  if (hasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

interface ErrorCategory {
  label: string
  count: number
}

const categories = computed<ErrorCategory[]>(() => {
  if (!props.data) return []

  let upstream = 0 // 502, 503, 504
  let client = 0 // 4xx
  let system = 0 // 500
  let other = 0

  for (const item of props.data.items || []) {
    const code = Number(item.status_code || 0)
    const count = Number(item.sla || 0)
    if (!Number.isFinite(code) || !Number.isFinite(count)) continue

    if ([502, 503, 504].includes(code)) upstream += count
    else if (code >= 400 && code < 500) client += count
    else if (code === 500) system += count
    else other += count
  }

  const out: ErrorCategory[] = []
  if (upstream > 0) out.push({ label: t('admin.ops.upstream'), count: upstream })
  if (client > 0) out.push({ label: t('admin.ops.client'), count: client })
  if (system > 0) out.push({ label: t('admin.ops.system'), count: system })
  if (other > 0) out.push({ label: t('admin.ops.other'), count: other })
  return out
})

const topReason = computed(() => {
  if (categories.value.length === 0) return null
  return categories.value.reduce((prev, cur) => (cur.count > prev.count ? cur : prev))
})

/**
 * 错误分布：环形图改横向条。原来靠颜色区分四类，压成墨色后颜色不再承载信息，
 * 改由长度（--p 占最大值的比例）和右侧百分比共同表达，类别名保留在左侧。
 */
const bars = computed(() => {
  const list = categories.value
  if (!list.length) return []
  const max = Math.max(...list.map((c) => c.count), 0)
  const total = list.reduce((a, b) => a + b.count, 0)
  return list.map((c) => ({
    label: c.label,
    count: c.count,
    share: max > 0 ? c.count / max : 0,
    pct: total > 0 ? `${((c.count / total) * 100).toFixed(1)}%` : '0%'
  }));
});
</script>

<template>
  <div class="ef-panel cut-tr flex h-full flex-col">
    <h2 class="flex items-center gap-2">
      <span>{{ t('admin.ops.errorDistribution') }}</span>
      <HelpTooltip :content="t('admin.ops.tooltips.errorDistribution')" />
      <button
        type="button"
        class="ef-chip"
        :disabled="state !== 'ready'"
        :title="t('admin.ops.errorTrend')"
        @click="emit('openDetails')"
      >
        {{ t('admin.ops.requestDetails.details') }}
      </button>
    </h2>

    <div class="relative min-h-0 flex-1">
      <div v-if="state === 'ready' && bars.length" class="flex h-full flex-col">
        <ol class="ef-dist">
          <li v-for="item in bars" :key="item.label">
            <span class="ef-dist-name">{{ item.label }}</span>
            <span class="ef-dist-pct">{{ item.pct }}</span>
            <span class="ef-dist-track" :style="{ '--p': item.share }"></span>
            <span class="ef-dist-val">{{ item.count }}</span>
          </li>
        </ol>
        <div v-if="topReason" class="mt-4 ef-field">
          {{ t('admin.ops.top') }}: {{ topReason.label }} · {{ topReason.count }}
        </div>
      </div>

      <div v-else class="flex h-full items-center justify-center">
        <div v-if="state === 'loading'" class="animate-pulse ef-field">{{ t('common.loading') }}</div>
        <EmptyState v-else :title="t('common.noData')" :description="t('admin.ops.charts.emptyError')" />
      </div>
    </div>
  </div>
</template>
