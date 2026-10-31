<template>
  <figure class="pt-token-mix" :aria-label="t('usage.tokenDetails')">
    <figcaption><span class="pt-kicker">// TOKEN MIX</span><h3>{{ t('usage.tokenDetails') }}</h3></figcaption>
    <p class="sr-only">{{ t('usage.cacheHitRate') }}: {{ hitRate }}</p>
    <svg viewBox="0 0 240 240" aria-hidden="true">
      <circle cx="120" cy="120" r="108" fill="none" stroke="var(--pt-rule)" stroke-width="1" stroke-dasharray="1 6" />
      <circle cx="120" cy="120" r="92" fill="none" stroke="var(--pt-chip)" stroke-width="13" />
      <circle v-for="part in arcs" :key="part.label" cx="120" cy="120" r="92" fill="none"
        :stroke="part.color" stroke-width="13" pathLength="100" :stroke-dasharray="part.dash"
        :stroke-dashoffset="part.offset" transform="rotate(-90 120 120)" />
      <circle cx="120" cy="120" r="76" fill="none" stroke="var(--pt-rule)" stroke-width="1" />
      <text x="120" y="116" text-anchor="middle" class="pt-mix-value">{{ hitRate }}</text>
      <text x="120" y="142" text-anchor="middle" class="pt-mix-label">{{ t('usage.cacheHitRate') }}</text>
    </svg>
    <ul class="pt-mix-legend">
      <li v-for="part in parts" :key="part.label"><i :style="{ background: part.color }" aria-hidden="true"></i><span>{{ part.label }}</span><b>{{ formatTokens(part.value) }}</b></li>
    </ul>
  </figure>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useChartPalette } from '@/utils/chartTheme'
const props = defineProps<{ input: number; output: number; creation: number; read: number }>()
const { t } = useI18n()
const p = useChartPalette()
const finite = (n: number) => Number.isFinite(n) ? Math.max(0, n) : 0
const parts = computed(() => [
  { label: t('dashboard.input'), value: finite(props.input), color: p.steps[0] },
  { label: t('dashboard.output'), value: finite(props.output), color: p.steps[1] },
  { label: t('usage.cacheCreationTokensLabel'), value: finite(props.creation), color: p.steps[2] },
  { label: t('usage.cacheReadTokensLabel'), value: finite(props.read), color: p.steps[3] }
])
const hitRate = computed(() => {
  const prompt = finite(props.input) + finite(props.creation) + finite(props.read)
  return prompt > 0 ? `${(finite(props.read) / prompt * 100).toFixed(1)}%` : '—'
})
const arcs = computed(() => {
  const total = parts.value.reduce((sum, part) => sum + part.value, 0)
  let used = 0
  return parts.value.map(part => {
    const share = total > 0 ? part.value / total * 100 : 0
    const result = { ...part, dash: `${Math.max(0, share - Math.min(1, share / 5))} 100`, offset: -used }
    used += share
    return result
  })
})
const formatTokens = (n: number) => n >= 1e6 ? `${(n / 1e6).toFixed(2)}M` : n >= 1e3 ? `${(n / 1e3).toFixed(1)}K` : n.toLocaleString()
</script>
