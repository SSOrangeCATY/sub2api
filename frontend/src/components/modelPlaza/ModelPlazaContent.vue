<template>
  <div class="price-catalog">
    <header class="price-catalog-heading">
      <div>
        <p class="price-eyebrow">// MODEL PRICING</p>
        <h1 v-if="!embedded">{{ t('modelPlaza.title') }}</h1>
        <h2 v-else>{{ t('modelPlaza.catalog.findPrice') }}</h2>
        <p>{{ t('modelPlaza.catalog.intro') }}</p>
      </div>
      <div class="price-identity"><span>{{ t('modelPlaza.catalog.pricingFor') }}</span><strong>{{ t(purchasePricing ? 'modelPlaza.catalog.purchaseIdentity' : 'modelPlaza.catalog.standardIdentity') }}</strong></div>
    </header>
    <p v-if="!isAuthenticated" class="price-status">{{ t('modelPlaza.catalog.loginHint') }}</p>
    <div v-if="loading" class="price-loading" role="status"><LoadingSpinner />{{ t('modelPlaza.loading') }}</div>
    <div v-else-if="error" class="price-status" role="alert">{{ t('modelPlaza.loadFailed') }} <button type="button" class="price-button" @click="$emit('retry')">{{ t('modelPlaza.catalog.retry') }}</button></div>
    <template v-else>
      <section class="price-search-panel" :class="{ 'price-search-panel--expanded': advanced }" :aria-label="t('modelPlaza.catalog.findPrice')">
        <label class="price-search-field">
          <span>{{ t('modelPlaza.filters.modelLabel') }}</span>
          <div><Icon name="search" /><input v-model="query" type="search" :placeholder="t('modelPlaza.filters.searchPlaceholder')" /></div>
        </label>
        <button type="button" class="price-button price-mobile-more" :aria-expanded="advanced" :aria-controls="filterId" @click="advanced = !advanced">{{ t('modelPlaza.catalog.moreFilters') }}<span v-if="filterCount">/ {{ filterCount }}</span><span aria-hidden="true">{{ advanced ? '−' : '+' }}</span></button>
        <div :id="filterId" class="price-filter-options">
        <label><span>{{ t('modelPlaza.filters.platformLabel') }}</span><select v-model="platform"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="p in platforms" :key="p" :value="p">{{ p }}</option></select></label>
        <label><span>{{ t('modelPlaza.filters.groupLabel') }}</span><select v-model="group"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="g in groups" :key="g.id" :value="String(g.id)">{{ g.name }}</option></select></label>
        <label><span>{{ t('modelPlaza.catalog.currency') }}</span><select v-model="currency"><option value="all">{{ t('modelPlaza.filters.all') }}</option><option v-for="c in currencies" :key="c" :value="c">{{ c }}</option></select></label>
        <div class="price-search-secondary">
          <label><span>{{ t('modelPlaza.catalog.context') }}</span><input v-model.number="contextTokens" type="number" min="0" max="100000000" step="1" :placeholder="t('modelPlaza.catalog.baseTier')" /></label>
          <label><span>{{ t('modelPlaza.catalog.sort') }}</span><select v-model="sort"><option value="name">{{ t('modelPlaza.catalog.sortName') }}</option><option value="input">{{ t('modelPlaza.catalog.sortInput') }}</option><option value="output">{{ t('modelPlaza.catalog.sortOutput') }}</option></select></label>
        </div>
        </div>
      </section>
      <div class="price-result-bar" aria-live="polite">
        <p>{{ t('modelPlaza.catalog.results', { models: uniqueModels, groups: uniqueGroups, prices: entries.length }) }}</p>
        <button v-if="hasFilters" type="button" class="price-button" @click="reset">{{ t('modelPlaza.catalog.reset') }}</button>
        <span v-if="sort !== 'name'">{{ t('modelPlaza.catalog.sortHint') }}</span>
      </div>
      <PlazaModelPricingTable v-if="entries.length" :models="[]" :entries="pageEntries" :rate-multiplier="1" :purchase-pricing="purchasePricing" :context-tokens="validContext" />
      <p v-else class="price-empty">{{ t(hasFilters ? 'modelPlaza.noSearchResult' : 'modelPlaza.empty') }}</p>
      <nav v-if="entries.length" class="price-pagination" :aria-label="t('modelPlaza.catalog.pagination')">
        <label>{{ t('modelPlaza.catalog.pageSize') }} <select v-model.number="pageSize"><option :value="25">25</option><option :value="50">50</option><option :value="100">100</option></select></label>
        <span aria-live="polite">{{ t('modelPlaza.catalog.page', { page, pages }) }}</span>
        <button type="button" class="price-button" :disabled="page <= 1" @click="page--">{{ t('modelPlaza.catalog.previous') }}</button>
        <button type="button" class="price-button" :disabled="page >= pages" @click="page++">{{ t('modelPlaza.catalog.next') }}</button>
      </nav>
      <details v-if="descriptionHtml" class="price-site-notes"><summary>{{ t('modelPlaza.catalog.siteNotes') }}</summary><div class="price-markdown" v-html="descriptionHtml"></div></details>
    </template>
  </div>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DOMPurify from 'dompurify'
import { marked } from 'marked'
import Icon from '@/components/icons/Icon.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import PlazaModelPricingTable from './PlazaModelPricingTable.vue'
import type { ModelPlazaResponse } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { currentVariants, pricingVariants, priceRanges } from '@/utils/modelPriceQuote'
import { useAppStore } from '@/stores/app'
import '@/styles/endfield/model-pricing.css'

const props = defineProps<{ response: ModelPlazaResponse | null; loading: boolean; error?: boolean; embedded?: boolean; initialGroup?: string; initialSearch?: string }>()
defineEmits<{ retry: [] }>()
const { t } = useI18n()
const auth = useAuthStore()
const app = useAppStore()
const now = ref(new Date())
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { now.value = new Date() }, 15000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
const isAuthenticated = computed(() => auth.isAuthenticated)
const purchasePricing = computed(() => props.response?.billing_identity === 'purchase')
const query = ref(props.initialSearch || '')
const group = ref(props.initialGroup || 'all')
const platform = ref('all')
const currency = ref('all')
const sort = ref('name')
const advanced = ref(false)
const filterId = useId()
const contextTokens = ref<number | string>(0)
const page = ref(1), pageSize = ref(25)
const validContext = computed(() => typeof contextTokens.value === 'number' && Number.isFinite(contextTokens.value) ? Math.min(100000000, Math.max(0, Math.floor(contextTokens.value))) : 0)
const groups = computed(() => props.response?.groups ?? [])
const platforms = computed(() => [...new Set(groups.value.flatMap(g => g.models.map(m => m.platform)))].sort())
const allEntries = computed(() => groups.value.flatMap(g => g.models.map(model => ({ group: g, model }))))
const currencies = computed(() => [...new Set(allEntries.value.flatMap(e => pricingVariants(e.model, e.group, purchasePricing.value).map(v => v.pricing.source_currency || 'USD')))].sort())
const entries = computed(() => {
  const words = query.value.toLowerCase().trim().split(/\s+/).filter(Boolean)
  return allEntries.value.filter(e =>
    (group.value === 'all' || String(e.group.id) === group.value) &&
    (platform.value === 'all' || e.model.platform === platform.value) &&
    words.every(word => (e.model.name + ' ' + e.group.name + ' ' + e.model.platform).toLowerCase().includes(word)) &&
    (currency.value === 'all' || pricingVariants(e.model, e.group, purchasePricing.value).some(v => (v.pricing.source_currency || 'USD') === currency.value))
  ).sort((a, b) => {
    if (sort.value !== 'name') {
      const field = sort.value === 'input' ? 'input_price' : 'output_price'
      const qa = priceRanges(currentVariants(a.model, a.group, purchasePricing.value, now.value, app.cachedPublicSettings?.server_utc_offset).filter(v => v.pricing.billing_mode === 'token'), field, validContext.value)[0]
      const qb = priceRanges(currentVariants(b.model, b.group, purchasePricing.value, now.value, app.cachedPublicSettings?.server_utc_offset).filter(v => v.pricing.billing_mode === 'token'), field, validContext.value)[0]
      if (!!qa !== !!qb) return qa ? -1 : 1
      if (qa && qb) {
        const denomination = qa.currency.localeCompare(qb.currency)
        if (denomination) return denomination
        if (qa.min !== qb.min) return qa.min - qb.min
      }
    }
    return a.model.name.localeCompare(b.model.name) || a.group.name.localeCompare(b.group.name)
  })
})
const uniqueModels = computed(() => new Set(entries.value.map(e => e.model.platform + ':' + e.model.name)).size)
const pages = computed(() => Math.max(1, Math.ceil(entries.value.length / pageSize.value)))
const pageEntries = computed(() => entries.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
watch([query, group, platform, currency, sort, validContext, pageSize], () => { page.value = 1 })
watch(pages, count => { page.value = Math.min(page.value, count) })
const uniqueGroups = computed(() => new Set(entries.value.map(e => e.group.id)).size)
const hasFilters = computed(() => !!query.value || group.value !== 'all' || platform.value !== 'all' || currency.value !== 'all' || sort.value !== 'name' || !!validContext.value)
const filterCount = computed(() => [group.value !== 'all', platform.value !== 'all', currency.value !== 'all', sort.value !== 'name', !!validContext.value].filter(Boolean).length)
function reset() { query.value = ''; group.value = 'all'; platform.value = 'all'; currency.value = 'all'; sort.value = 'name'; contextTokens.value = 0 }
watch(groups, list => { if (group.value !== 'all' && !list.some(g => String(g.id) === group.value)) group.value = 'all' })
const descriptionHtml = computed(() => props.response?.description?.trim() ? DOMPurify.sanitize(marked.parse(props.response.description) as string) : '')
</script>
