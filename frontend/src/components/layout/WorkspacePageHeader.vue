<template>
  <header class="pt-workspace-heading">
    <div>
      <p v-if="eyebrow" class="pt-workspace-eyebrow">// {{ eyebrow }}</p>
      <h1>{{ title }}</h1>
      <p v-if="description" class="pt-workspace-description">{{ description }}</p>
    </div>
  </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { PURCHASE_ROUTE_NAME, resolveRouteMetaKeys } from '@/router/title'
import { resolveSiteBillingMode } from '@/utils/siteBillingMode'

const route = useRoute()
const appStore = useAppStore()
const { t, te } = useI18n()
const billingMode = computed(() => resolveSiteBillingMode(appStore.cachedPublicSettings))
const routeMetaKeys = computed(() => resolveRouteMetaKeys(route, { billingMode: billingMode.value }))
const title = computed(() => {
  const key = routeMetaKeys.value.titleKey
  return key && te(key) ? t(key) : String(route.meta.title || '')
})
const eyebrow = computed(() => {
  if (route.name === PURCHASE_ROUTE_NAME) {
    if (billingMode.value === 'recharge_only') return 'RECHARGE'
    if (billingMode.value === 'subscription_only') return 'SUBSCRIPTIONS'
    return 'RECHARGE / SUBSCRIPTIONS'
  }
  return String(route.meta.title || '').replace(/\s+/g, ' ').toUpperCase()
})
const description = computed(() => {
  const key = routeMetaKeys.value.descriptionKey
  return key && te(key) ? t(key) : ''
})
</script>
