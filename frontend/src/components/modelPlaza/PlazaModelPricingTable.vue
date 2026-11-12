<template>
  <div class="price-catalog-table">
    <div class="price-table-caption">
      <div><span class="price-eyebrow">// YOUR PRICE</span><strong>{{ t('modelPlaza.catalog.actualPrice') }}</strong></div>
      <PriceUnitHelp :currencies="currencies" :purchase="purchasePricing" />
    </div>
    <p class="price-table-note">{{ t('modelPlaza.catalog.currentPriceHint') }} <span v-if="contextTokens">{{ t('modelPlaza.catalog.contextValue', { count: contextTokens.toLocaleString() }) }}</span></p>
    <table>
      <thead><tr>
        <th scope="col">{{ t('modelPlaza.table.model') }}</th>
        <th scope="col">{{ t('modelPlaza.table.input') }}</th>
        <th scope="col">{{ t('modelPlaza.table.output') }}</th>
        <th scope="col">{{ t('modelPlaza.table.cacheRead') }}</th>
        <th scope="col"><span class="sr-only">{{ t('modelPlaza.catalog.details') }}</span></th>
      </tr></thead>
      <tbody v-for="entry in rows" :key="entry.key">
        <tr class="price-summary">
          <td class="price-model-cell">
            <strong>{{ entry.model.name }}</strong>
            <span class="price-model-meta"><span v-if="entry.group.name">{{ entry.group.name }} │ </span>{{ entry.model.platform }} │ {{ modeLabel(entry.model) }}</span>
            <span v-if="purchasePricing && entry.model.price_quote?.variants.length && entry.model.price_quote.variants.length > 1" class="price-range-note">{{ t('modelPlaza.catalog.routeRange') }}</span>
            <span v-if="quoteTime(entry.model)" class="price-model-meta">{{ t('modelPlaza.catalog.quotedAt', { time: new Date(quoteTime(entry.model)!).toLocaleTimeString() }) }}</span>
          </td>
          <template v-if="mode(entry.model) === 'token' || mode(entry.model) === 'mixed'">
            <td v-for="field in mainFields" :key="field" :data-label="fieldLabel(field)" class="price-number-cell">
              <span v-for="range in ranges(entry, field)" :key="range.currency" class="price-number">{{ range.text }}</span>
              <span v-if="!ranges(entry, field).length" class="price-unavailable">—</span>
            </td>
          </template>
          <td v-else colspan="3" class="price-request-cell" :data-label="modeLabel(entry.model)">
            <template v-if="requestPrices(entry).length">
              <span v-for="price in requestPrices(entry)" :key="price.key" class="price-request-tier"><span>{{ price.label }}</span><strong>{{ price.text }}</strong><span>{{ perUnit(entry.model) }}</span></span>
            </template>
            <span v-else class="price-unavailable">{{ t('modelPlaza.catalog.unavailable') }}</span>
          </td>
          <td class="price-action-cell">
            <button type="button" class="price-button price-details-button" :aria-expanded="expanded.has(entry.key)" :aria-controls="detailId(entry.key)" @click="toggle(entry.key)">
              {{ t(expanded.has(entry.key) ? 'modelPlaza.catalog.collapse' : 'modelPlaza.catalog.details') }}
              <span aria-hidden="true">{{ expanded.has(entry.key) ? '−' : '+' }}</span>
            </button>
          </td>
        </tr>
        <tr v-if="mode(entry.model) === 'mixed'" class="price-mixed-row"><td colspan="5"><span>{{ t('modelPlaza.catalog.mode.mixed') }} │ </span><span v-for="price in requestPrices(entry)" :key="price.key">{{ price.label }} {{ price.text }} {{ price.unit }} │ </span></td></tr>
        <tr v-if="!entry.variants.length" class="price-unavailable-row"><td colspan="5">{{ t('modelPlaza.catalog.unavailableHint') }}</td></tr>
        <tr v-if="expanded.has(entry.key)" :id="detailId(entry.key)" class="price-detail-row">
          <td colspan="5">
            <div class="price-details">
              <p v-if="purchasePricing">{{ t('modelPlaza.catalog.purchaseHelp') }}</p>
              <section v-for="(variant, index) in entry.baseVariants" :key="index" class="price-variant">
                <h4>{{ t('modelPlaza.catalog.priceSheet') }} <span v-if="entry.baseVariants.length > 1">/ {{ index + 1 }}</span> <span>{{ variant.pricing.source_currency || 'USD' }} │ {{ t('modelPlaza.catalog.mode.' + variant.pricing.billing_mode) }} │ {{ t((purchasePricing || entry.model.pricing_as_of) && !variant.time_pricing ? 'modelPlaza.catalog.snapshotPeriod' : 'modelPlaza.catalog.standardPeriod') }}</span></h4>
                <p v-if="variant.pricing.intervals.length && variant.pricing.billing_mode === 'token'">{{ t(variant.context_metric === 'total' ? 'modelPlaza.catalog.totalContext' : 'modelPlaza.table.tierHint') }}</p>
                <div class="price-tier-list">
                  <section v-for="(tier, tierIndex) in tiers(variant)" :key="tierIndex" class="price-tier">
                    <strong v-if="variant.pricing.intervals.length">{{ tierLabel(tier) }}</strong>
                    <p v-if="tier.unconfirmed">{{ t('modelPlaza.catalog.unavailable') }}</p>
                    <dl>
                      <template v-for="field in fieldsForVariant(variant)" :key="field">
                        <template v-if="tier[field] != null">
                          <dt>{{ fieldLabel(field) }}</dt>
                          <dd>{{ formatScaled(tier[field] ?? null, variant.pricing.billing_mode === 'token' ? 1000000 : 1, 2, variant.pricing.source_currency || 'USD') }}</dd>
                        </template>
                      </template>
                    </dl>
                  </section>
                </div>
                <p v-for="(period, periodIndex) in variant.time_pricing?.periods || []" :key="periodIndex" class="price-rule">
                  {{ period.start_time }}–{{ period.end_time }} │ {{ variant.time_pricing?.timezone }} │ {{ t('modelPlaza.catalog.periodFactor', { multiplier: period.multiplier }) }}<span v-if="variant.time_pricing?.weekdays_only"> │ {{ t('modelPlaza.table.timePricingWeekdays') }}</span>
                </p>
                <p v-for="(factor, effort) in reasoningFactors(variant)" :key="effort" class="price-rule">{{ t('modelPlaza.table.reasoningMultiplierHint', { effort, multiplier: factor }) }}</p>
              </section>
              <p v-if="!purchasePricing && entry.group.peak_rate_enabled" class="price-rule">{{ t('modelPlaza.detail.peakNote', { window: entry.group.peak_start + '–' + entry.group.peak_end + ' UTC' + (serverOffset || ''), multiplier: entry.group.peak_rate_multiplier }) }}</p>
              <section v-if="entry.model.official_pricing" class="price-official">
                <h4>{{ t('modelPlaza.catalog.reference') }} │ USD / 1M token</h4>
                <p>{{ t('modelPlaza.catalog.referenceHint') }}</p>
                <div class="price-tier-list">
                  <section v-for="(tier, index) in officialTiers(entry.model)" :key="index" class="price-tier">
                    <strong v-if="entry.model.official_pricing.intervals?.length">{{ tierLabel(tier) }}</strong>
                    <dl><template v-for="field in tokenFields" :key="field"><template v-if="tier[field] != null"><dt>{{ fieldLabel(field) }}</dt><dd>{{ formatScaled(tier[field] ?? null, 1000000, 2) }}</dd></template></template></dl>
                  </section>
                </div>
              </section>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ModelPlazaGroup, ModelPriceVariant, PlazaModel } from '@/api/modelPlaza'
import { currentVariants, hasUncoveredContext, priceRanges, pricingVariants, type PriceField, type QuoteGroup } from '@/utils/modelPriceQuote'
import { formatScaled, resolveIntervalPrices } from '@/utils/pricing'
import { useAppStore } from '@/stores/app'
import { REASONING_EFFORT_LEVELS } from '@/constants/channel'
import PriceUnitHelp from './PriceUnitHelp.vue'

const props = withDefaults(defineProps<{
  models: PlazaModel[]
  entries?: Array<{ model: PlazaModel; group: ModelPlazaGroup }>
  purchasePricing?: boolean
  rateMultiplier: number
  userRateMultiplier?: number | null
  imageRateIndependent?: boolean
  imageRateMultiplier?: number | null
  videoRateIndependent?: boolean
  videoRateMultiplier?: number | null
  peakWindow?: string
  peakRateMultiplier?: number | null
  platform?: string
  group?: ModelPlazaGroup
  contextTokens?: number
}>(), { contextTokens: 0 })
const { t } = useI18n()
const app = useAppStore()
const serverOffset = computed(() => app.cachedPublicSettings?.server_utc_offset)
const expanded = ref(new Set<string>())
const id = useId()
const now = ref(new Date())
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => { timer = setInterval(() => { now.value = new Date() }, 15000) })
onUnmounted(() => { if (timer) clearInterval(timer) })
const mainFields: PriceField[] = ['input_price', 'output_price', 'cache_read_price']
const tokenFields: PriceField[] = [...mainFields, 'cache_write_price', 'cache_write_1h_price', 'cache_write_12h_price']
const detailFields: PriceField[] = [...tokenFields, 'image_input_price', 'image_output_price', 'per_request_price']
const fallbackGroup = computed<QuoteGroup>(() => props.group ?? {
  rate_multiplier: props.rateMultiplier, user_rate_multiplier: props.userRateMultiplier ?? undefined,
  image_rate_independent: props.imageRateIndependent, image_rate_multiplier: props.imageRateMultiplier ?? undefined,
  video_rate_independent: props.videoRateIndependent, video_rate_multiplier: props.videoRateMultiplier ?? undefined
})
const rows = computed(() => (props.entries ?? props.models.map(model => ({ model, group: fallbackGroup.value }))).map(entry => ({
  ...entry,
  key: [entry.group.id ?? 0, entry.model.platform, entry.model.name].join(':'),
  variants: quoteTime(entry.model) && (now.value.getTime() - new Date(quoteTime(entry.model)!).getTime() > 90000)
    ? [] : currentVariants(entry.model, entry.group, !!props.purchasePricing, now.value, serverOffset.value),
  baseVariants: fresh(entry.model) ? pricingVariants(entry.model, entry.group, !!props.purchasePricing) : []
})))
type Row = (typeof rows.value)[number]
function quoteTime(model: PlazaModel) { return props.purchasePricing ? model.price_quote?.quoted_at : model.pricing_as_of }
function fresh(model: PlazaModel) { const at = quoteTime(model); return !at || now.value.getTime() - new Date(at).getTime() <= 90000 }
function reasoningFactors(variant: ModelPriceVariant) {
  return Object.fromEntries(REASONING_EFFORT_LEVELS.flatMap(effort => {
    const multiplier = variant.pricing.reasoning_effort_multipliers?.[effort]
    return typeof multiplier === 'number' && Number.isFinite(multiplier) && multiplier > 0 ? [[effort, multiplier]] : []
  }))
}
const currencies = computed(() => [...new Set(rows.value.flatMap(row => row.variants.map(v => v.pricing.source_currency || 'USD')))].sort())
function toggle(key: string) { const next = new Set(expanded.value); if (!next.delete(key)) next.add(key); expanded.value = next }
function detailId(key: string) { return id + '-' + encodeURIComponent(key) }
function mode(model: PlazaModel) {
  const modes = new Set(props.purchasePricing ? model.price_quote?.variants.map(v => v.pricing.billing_mode) : [model.pricing?.billing_mode])
  return modes.size > 1 ? 'mixed' : (props.purchasePricing ? model.price_quote?.variants[0]?.pricing.billing_mode : model.pricing?.billing_mode) || model.pricing?.billing_mode || 'token'
}
function modeLabel(model: PlazaModel) { return t('modelPlaza.catalog.mode.' + mode(model)) }
function perUnit(model: PlazaModel) { return t('modelPlaza.catalog.per.' + mode(model)) }
function ranges(row: Row, field: PriceField) { return priceRanges(row.variants.filter(v => v.pricing.billing_mode === 'token'), field, props.contextTokens) }
function fieldLabel(field: PriceField) { return t('modelPlaza.catalog.fields.' + field) }
type DisplayTier = Partial<Record<PriceField, number | null>> & { tier_label?: string; min_tokens?: number; max_tokens?: number | null; unconfirmed?: boolean }
function fieldsForVariant(variant: ModelPriceVariant) { return variant.pricing.billing_mode === 'token' ? detailFields.filter(field => field !== 'per_request_price') : ['per_request_price'] as PriceField[] }
function tiers(variant: ModelPriceVariant): DisplayTier[] {
  if (!variant.pricing.intervals.length) return [variant.pricing]
  const result: DisplayTier[] = variant.pricing.intervals.map(iv => variant.interval_price_mode === 'absolute' ? iv : resolveIntervalPrices(iv, variant.pricing))
  if (hasUncoveredContext(variant.pricing)) result.push({ ...variant.pricing, tier_label: t('modelPlaza.catalog.unmatchedTier') })
  if (variant.pricing.billing_mode !== 'token' && variant.interval_price_mode !== 'absolute') {
    for (let index = 0; index < variant.pricing.intervals.length; index++) {
      if (result[index].per_request_price === 0) result[index] = { ...result[index], per_request_price: null, unconfirmed: true }
    }
  }
  return result
}
function officialTiers(model: PlazaModel): DisplayTier[] {
  const p = model.official_pricing!
  return p.intervals?.length ? p.intervals.map(iv => resolveIntervalPrices(iv, p)) : [p]
}
function tierLabel(tier: { tier_label?: string; min_tokens?: number; max_tokens?: number | null }) {
  if (tier.tier_label) return tier.tier_label
  return tier.max_tokens == null ? '>' + (tier.min_tokens ?? 0).toLocaleString() : (tier.min_tokens ?? 0).toLocaleString() + '–' + tier.max_tokens.toLocaleString()
}
function requestPrices(row: Row) {
  const nonToken = row.variants.filter(v => v.pricing.billing_mode !== 'token')
  return [...new Set(nonToken.map(v => v.pricing.billing_mode))].flatMap(billingMode => {
  const modeVariants = nonToken.filter(v => v.pricing.billing_mode === billingMode)
  const unit = t('modelPlaza.catalog.per.' + billingMode)
  const labels = [...new Set(modeVariants.flatMap(v => v.pricing.intervals.filter(iv => iv.per_request_price != null).map(iv => iv.tier_label || tierLabel(iv))))]
  if (!labels.length) return priceRanges(modeVariants, 'per_request_price').map(range => ({ ...range, key: billingMode + range.currency, label: '', unit }))
  return labels.flatMap(label => {
    const variants = modeVariants.map(v => {
      const iv = v.pricing.intervals.find(iv => (iv.tier_label || tierLabel(iv)) === label)
      const value = iv?.per_request_price ?? v.pricing.per_request_price
      const unconfirmed = (iv?.per_request_price === 0 && v.interval_price_mode !== 'absolute') || (!iv && v.pricing.intervals.length > 0)
      return { ...v, pricing: { ...v.pricing, per_request_price: unconfirmed ? null : value } }
    })
    const ranges = priceRanges(variants, 'per_request_price')
    if (!ranges.length) return [{ key: billingMode + label, label, unit, text: t('modelPlaza.catalog.unavailable') }]
    return ranges.map(range => ({ ...range, key: billingMode + label + range.currency, label, unit }))
  })
  })
}
</script>
