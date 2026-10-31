<template>
  <section class="pt-recent-usage" :aria-busy="loading">
    <div class="pt-subsection-title"><div><span class="pt-kicker">// RECENT USAGE</span><h2>{{ t('dashboard.recentUsage') }}</h2></div><span class="pt-recent-count">{{ data.length }}</span></div>
    <p v-if="startDate && endDate" class="pt-record-range">{{ startDate }} / {{ endDate }}</p>
    <div v-if="loading && !data.length" class="pt-record-loading"><LoadingSpinner size="lg" /></div>
    <div v-else-if="!data.length" class="pt-record-empty"><Icon name="beaker" size="lg" /><h3>{{ t('dashboard.noUsageRecords') }}</h3><p>{{ t('dashboard.startUsingApi') }}</p></div>
    <ol v-else class="pt-record-list" :class="{ 'is-updating': loading }">
      <li v-for="log in data" :key="log.id" class="pt-usage-record">
        <span class="pt-record-id">#{{ log.id }}</span>
        <div class="pt-record-model"><strong :title="log.model">{{ log.model }}</strong><time :datetime="log.created_at">{{ formatDateTime(log.created_at) }}</time></div>
        <div class="pt-record-cost"><strong :title="t('dashboard.actual')">${{ formatCost(log.actual_cost) }}</strong><span :title="t('dashboard.standard')">/ ${{ formatCost(log.total_cost) }}</span><small>{{ (log.input_tokens + log.output_tokens).toLocaleString() }} tokens</small></div>
      </li>
    </ol>
    <router-link to="/usage" class="pt-record-link">{{ t('dashboard.viewAllUsage') }}<Icon name="arrowRight" size="sm" /></router-link>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'
import type { UsageLog } from '@/types'
defineProps<{ data: UsageLog[]; loading: boolean; startDate?: string; endDate?: string }>()
const { t } = useI18n()
const formatCost = (cost: number) => cost.toFixed(4)
</script>
