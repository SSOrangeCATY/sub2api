<template>
  <span class="price-preview">
    <button type="button" class="price-button" :aria-label="label || t('modelPlaza.catalog.preview')" @click="show = true">{{ label || t('modelPlaza.catalog.preview') }}</button>
    <BaseDialog :show="show" :title="t('modelPlaza.catalog.previewTitle')" width="extra-wide" @close="show = false">
      <div class="price-preview">
        <label v-if="groups.length > 1" class="price-preview-select">{{ t('modelPlaza.filters.groupLabel') }}<select v-model="selected" class="input"><option v-for="group in groups" :key="group.id" :value="group.id">{{ group.name || '#' + group.id }}</option></select></label>
        <label class="price-preview-search">{{ t('modelPlaza.filters.modelLabel') }}<input v-model="search" type="search" :placeholder="t('modelPlaza.filters.searchPlaceholder')" /></label>
        <p v-if="loading" role="status" class="price-status">{{ t('modelPlaza.loading') }}</p>
        <p v-else-if="error" role="alert" class="price-status">{{ t('modelPlaza.loadFailed') }} <button type="button" class="price-button" @click="load">{{ t('modelPlaza.catalog.retry') }}</button></p>
        <template v-else-if="data">
          <template v-for="group in data.groups" :key="group.id">
            <h3 class="price-preview-group">{{ group.name }}</h3>
            <PlazaModelPricingTable v-if="filteredModels(group).length" :group="group" :models="filteredModels(group)" :purchase-pricing="data.billing_identity === 'purchase'" :rate-multiplier="group.rate_multiplier" />
            <p v-else class="price-empty">{{ t('modelPlaza.noSearchResult') }}</p>
          </template>
          <p v-if="!data.groups.length" class="price-empty">{{ t('modelPlaza.catalog.previewEmpty') }}</p>
        </template>
      </div>
    </BaseDialog>
  </span>
</template>
<script setup lang="ts">
import { onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getGroupModelPrices, type ModelPlazaResponse, type ModelPlazaGroup } from '@/api/modelPlaza'
import BaseDialog from '@/components/common/BaseDialog.vue'
import PlazaModelPricingTable from './PlazaModelPricingTable.vue'
import '@/styles/endfield/model-pricing.css'
const props = defineProps<{ groups: Array<{ id: number; name?: string }>; modelName?: string; label?: string }>()
const { t } = useI18n()
const show = ref(false), loading = ref(false), error = ref(false)
const data = ref<ModelPlazaResponse | null>(null)
const selected = ref(props.groups[0]?.id)
const search = ref(props.modelName || '')
let request: AbortController | undefined
let sequence = 0
let timer: ReturnType<typeof setInterval> | undefined
async function load() {
  request?.abort()
  const current = ++sequence
  data.value = null
  error.value = false
  if (!show.value || !selected.value) return
  request = new AbortController()
  loading.value = true
  try {
    const result = await getGroupModelPrices(selected.value, request.signal)
    if (current === sequence && show.value) data.value = result
  } catch {
    if (current === sequence && show.value && !request.signal.aborted) error.value = true
  } finally { if (current === sequence) loading.value = false }
}
watch([show, selected], () => {
  if (timer) { clearInterval(timer); timer = undefined }
  if (show.value) {
    void load()
    timer = setInterval(() => { if (!document.hidden && !loading.value) void load() }, 45000)
  } else { sequence++; request?.abort(); data.value = null; loading.value = false }
})
watch(() => props.groups, list => {
  if (!list.some(group => group.id === selected.value)) selected.value = list[0]?.id
})
watch(() => props.modelName, value => { search.value = value || '' })
onUnmounted(() => { sequence++; request?.abort(); if (timer) clearInterval(timer) })
function filteredModels(group: ModelPlazaGroup) { return group.models.filter(model => model.name.toLowerCase().includes(search.value.trim().toLowerCase())) }
</script>
