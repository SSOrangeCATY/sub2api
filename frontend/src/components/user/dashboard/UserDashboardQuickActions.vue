<template>
  <section class="pt-user-quick-actions">
    <div class="pt-subsection-title"><div><span class="pt-kicker">// QUICK ACTIONS</span><h2>{{ t('dashboard.quickActions') }}</h2></div></div>
    <nav :aria-label="t('dashboard.quickActions')">
      <router-link to="/keys" class="pt-quick-action pt-quick-action--primary"><Icon name="key" size="lg" /><span><strong>{{ t('dashboard.createApiKey') }}</strong><small>{{ t('dashboard.generateNewKey') }}</small></span><Icon name="arrowRight" size="md" /></router-link>
      <router-link to="/usage" class="pt-quick-action"><Icon name="chart" size="lg" /><span><strong>{{ t('dashboard.viewUsage') }}</strong><small>{{ t('dashboard.checkDetailedLogs') }}</small></span><Icon name="arrowRight" size="md" /></router-link>
      <router-link v-if="canUseBatchImage" to="/batch-image" class="pt-quick-action"><Icon name="sparkles" size="lg" /><span><strong>{{ t('dashboard.batchImageAgent') }}</strong><small>{{ t('dashboard.batchImageAgentDesc') }}</small></span><Icon name="arrowRight" size="md" /></router-link>
      <router-link to="/redeem" class="pt-quick-action"><Icon name="gift" size="lg" /><span><strong>{{ t('dashboard.redeemCode') }}</strong><small>{{ t('dashboard.addBalanceWithCode') }}</small></span><Icon name="arrowRight" size="md" /></router-link>
    </nav>
  </section>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()
onMounted(() => { void refreshBatchImageAccess() })
</script>
