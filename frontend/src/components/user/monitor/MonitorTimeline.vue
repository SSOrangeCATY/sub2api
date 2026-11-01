<template>
  <div class="monitor-timeline">
    <div
      class="monitor-timeline__heading"
    >
      <span>{{ t('monitorCommon.history60pts', { n: length }) }}</span>
      <span class="tabular-nums">{{ t('monitorCommon.nextUpdateIn', { n: countdownSeconds }) }}</span>
    </div>

    <div
      v-if="maintenance"
      class="monitor-timeline__maintenance"
    >
      {{ t('monitorCommon.maintenancePaused') }}
    </div>
    <div v-else class="monitor-timeline__bars" :aria-label="t('monitorCommon.timelineLegend')" role="group">
      <div
        v-for="(bar, idx) in displayBars"
        :key="idx"
        class="monitor-timeline__slot"
        :aria-hidden="bar.status === 'empty' ? true : undefined"
      >
        <div
          v-if="bar.status !== 'empty'"
          class="monitor-timeline__bar"
          :data-status="bar.status"
          :style="{ height: bar.heightPct + '%' }"
          :title="bar.title"
          :aria-label="bar.title"
          role="img"
        ></div>
      </div>
    </div>

    <div
      class="monitor-timeline__axis"
    >
      <span>{{ t('monitorCommon.past') }}</span>
      <span>{{ t('monitorCommon.now') }}</span>
    </div>
    <ul v-if="!maintenance" class="monitor-timeline__legend" :aria-label="t('monitorCommon.timelineLegend')">
      <li v-for="item in legend" :key="item.status">
        <span class="monitor-timeline__swatch" :data-status="item.status" aria-hidden="true"></span>
        <span>{{ item.label }}</span><b>{{ item.count }}</b>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import { useChannelMonitorFormat } from '@/composables/useChannelMonitorFormat'

const props = withDefaults(defineProps<{
  buckets?: MonitorTimelinePoint[]
  countdownSeconds: number
  length?: number
  maintenance?: boolean
}>(), {
  buckets: () => [],
  length: 60,
  maintenance: false,
})

const { t } = useI18n()
const { statusLabel, formatLatency, formatRelativeTime } = useChannelMonitorFormat()

interface Bar {
  status: string
  heightPct: number
  title: string
}

// History uses solid green/yellow/deep red, with height and labels as extra cues.
// Missing records retain their time slots without rendering a bar or legend item.
const STATUS_HEIGHT: Record<string, number> = {
  operational: 100,
  degraded: 65,
  failed: 35,
  error: 35,
  empty: 0,
}

const normalizedStatus = (status: string) => Object.prototype.hasOwnProperty.call(STATUS_HEIGHT, status) ? status : 'empty'
const legend = computed(() => ['operational', 'degraded', 'failed', 'error'].map(status => ({
  status,
  label: statusLabel(status as MonitorTimelinePoint['status']),
  count: displayBars.value.filter(bar => bar.status === status).length,
})))

const displayBars = computed<Bar[]>(() => {
  // Real points come newest-first; convert to oldest-first so the rightmost
  // bar represents "now". Pad the left with empty placeholders to keep the
  // bar count stable at `length`.
  const real = [...(props.buckets ?? [])]
    .slice(0, props.length)
    .reverse()

  const padCount = Math.max(0, props.length - real.length)
  const bars: Bar[] = []

  for (let i = 0; i < padCount; i += 1) {
    bars.push({
      status: 'empty',
      heightPct: STATUS_HEIGHT.empty,
      title: t('monitorCommon.timelineNoData'),
    })
  }

  for (const point of real) {
    const status = normalizedStatus(point.status)
    const heightPct = STATUS_HEIGHT[status] ?? STATUS_HEIGHT.empty
    const latency = formatLatency(point.latency_ms)
    const relative = formatRelativeTime(point.checked_at)
    const label = status === 'empty' ? t('monitorCommon.timelineNoData') : statusLabel(point.status)
    bars.push({
      status,
      heightPct,
      title: `${relative} / ${label} / ${latency}${point.latency_ms == null ? '' : ' ms'}`,
    })
  }

  return bars
})
</script>
