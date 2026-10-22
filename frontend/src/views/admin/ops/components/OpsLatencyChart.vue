<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OpsLatencyHistogramResponse } from '@/api/admin/ops'
import type { ChartState } from '../types'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import EmptyState from '@/components/common/EmptyState.vue'

interface Props {
  latencyData: OpsLatencyHistogramResponse | null
  loading: boolean
}

const props = defineProps<Props>()
const { t } = useI18n()

const hasData = computed(() => (props.latencyData?.total_requests ?? 0) > 0)

const state = computed<ChartState>(() => {
  if (hasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

/**
 * 延迟直方图：唯一一个真正的柱状图，改用 skill 的 .ef-bars —— 纯 DOM/CSS，
 * 没有 canvas，也不再有 chart.js 依赖。--v（0–1）是每根柱的高度，
 * <span> 是柱下方的桶区间标签（.ef-bars 契约：必须是 <ol>）。
 */
const bars = computed(() => {
  const buckets = props.latencyData?.buckets ?? []
  const counts = buckets.map((b) => b.count)
  const max = Math.max(...counts, 0)
  return buckets.map((b) => ({
    range: b.range,
    count: b.count,
    v: max > 0 ? b.count / max : 0
  }));
})
</script>

<template>
  <div class="ef-panel cut-tr flex h-full flex-col">
    <h2 class="flex items-center gap-2">
      <span>{{ t('admin.ops.latencyHistogram') }}</span>
      <HelpTooltip :content="t('admin.ops.tooltips.latencyHistogram')" />
    </h2>

    <div class="min-h-0 flex-1">
      <template v-if="state === 'ready' && bars.length">
        <ol class="ef-bars" :aria-label="t('admin.ops.latencyHistogram')">
          <li
            v-for="(bar, i) in bars"
            :key="i"
            :style="{ '--v': bar.v }"
            :title="bar.range + ': ' + bar.count"
          >
            <span>{{ bar.range }}</span>
          </li>
        </ol>
        <p class="ef-field mt-6 text-center">
          {{ t('admin.ops.requests') }} · {{ props.latencyData?.total_requests ?? 0 }}
        </p>
      </template>
      <div v-else class="flex h-full items-center justify-center">
        <div v-if="state === 'loading'" class="animate-pulse ef-field">{{ t('common.loading') }}</div>
        <EmptyState v-else :title="t('common.noData')" :description="t('admin.ops.charts.emptyRequest')" />
      </div>
    </div>
  </div>
</template>
