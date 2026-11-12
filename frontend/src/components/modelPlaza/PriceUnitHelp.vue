<template>
  <div class="price-unit-help" @mouseenter="open = true" @mouseleave="onLeave" @focusout="onFocusOut" @keydown.esc.stop="close">
    <span>{{ currencies.join(' / ') }} │ {{ t('modelPlaza.catalog.unit') }}</span>
    <button type="button" class="price-help-button" :aria-label="t('modelPlaza.catalog.currencyHelp')" :aria-expanded="open" :aria-describedby="open ? helpId : undefined" @focus="open = true" @click="toggle">
      <svg aria-hidden="true" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6"><circle cx="12" cy="12" r="9"/><path d="M9.5 8.5a2.5 2.5 0 0 1 5 0c0 2-2.5 2-2.5 4M12 16h.01"/></svg>
    </button>
    <div v-if="open" :id="helpId" role="tooltip" class="price-help-content">
      <strong>{{ t('modelPlaza.catalog.currencyHelp') }}</strong>
      <p v-if="currencies.includes('CNY')">{{ t('modelPlaza.catalog.cnyHelp') }}</p>
      <p>{{ t('modelPlaza.catalog.usdHelp') }}</p>
      <p>{{ t('modelPlaza.catalog.measureHelp') }}</p>
      <p>{{ t(purchase ? 'modelPlaza.catalog.purchaseHelp' : 'modelPlaza.catalog.discountHelp') }}</p>
    </div>
  </div>
</template>
<script setup lang="ts">
import { ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
defineProps<{ currencies: string[]; purchase?: boolean }>()
const { t } = useI18n()
const open = ref(false)
const pinned = ref(false)
const helpId = useId()
function close() { open.value = false; pinned.value = false }
function toggle() { pinned.value = !pinned.value; open.value = pinned.value }
function onLeave() { if (!pinned.value) open.value = false }
function onFocusOut(event: FocusEvent) {
  if (!(event.relatedTarget instanceof Node) || !(event.currentTarget as HTMLElement).contains(event.relatedTarget)) close()
}
</script>
