<template>
  <section class="pt-community" :aria-label="t('home.community.title')">
    <span class="pt-community-mark" aria-hidden="true">QQ</span>
    <div class="pt-community-info">
      <p class="pt-community-kicker">// COMMUNITY</p>
      <h2>{{ t('home.community.title') }}</h2>
      <button type="button" class="pt-community-copy" :aria-label="t('home.community.copyGroup')" @click="copyToClipboard(groupNumber)">
        <span>{{ groupNumber }}</span><Icon :name="copied ? 'check' : 'copy'" size="xs" aria-hidden="true" />
      </button>
    </div>
    <details ref="details" class="pt-community-qr" @keydown.esc.stop.prevent="closePreview" @toggle="expanded = !!details?.open">
      <summary :aria-label="t('home.community.showQr')" :aria-expanded="expanded" aria-controls="home-community-qr">
        <img src="/community-qq.png" alt="" width="32" height="32" aria-hidden="true" />
      </summary>
      <div id="home-community-qr" class="pt-community-preview">
        <div class="pt-community-preview-head"><span>{{ t('home.community.scanToJoin') }}</span><button type="button" :aria-label="t('common.close')" @click="closePreview"><Icon name="x" size="sm" /></button></div>
        <img src="/community-qq.png" :alt="t('home.community.qrAlt', { number: groupNumber })" width="200" height="200" />
        <p>QQ / {{ groupNumber }}</p>
      </div>
    </details>
  </section>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import Icon from '@/components/icons/Icon.vue'

const groupNumber = '487375322'
const details = ref<HTMLDetailsElement>()
const expanded = ref(false)
const { t } = useI18n()
const { copied, copyToClipboard } = useClipboard()
function closePreview() {
  if (!details.value?.open) return
  const restoreFocus = details.value.contains(document.activeElement)
  details.value.open = false
  if (restoreFocus) details.value.querySelector('summary')?.focus()
}
onClickOutside(details, closePreview)
</script>

<style scoped>
.pt-community { display: flex; align-items: center; gap: 14px; position: relative; width: 304px; max-width: 100%; margin-top: 24px; padding: 13px 14px; border-top: 2px solid var(--pt-ink); background: var(--pt-panel); color: var(--pt-ink); }
.pt-community-mark { display: grid; place-items: center; width: 42px; height: 42px; flex: 0 0 auto; background: var(--ef-signal); color: var(--ef-on-signal); font-size: 20px; font-weight: 800; letter-spacing: -.07em; }
.pt-community-info { flex: 1; min-width: 0; }
.pt-community-kicker { font-size: 8px; font-weight: 700; letter-spacing: .14em; color: var(--pt-muted); line-height: 1.3; }
.pt-community h2 { font-size: 14px; font-weight: 750; margin-top: 3px; line-height: 1.4; }
.pt-community-copy { display: flex; align-items: center; gap: 8px; min-height: 28px; font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums; color: var(--pt-muted); }
.pt-community-copy:hover { color: var(--pt-ink); text-decoration: underline; text-underline-offset: 3px; }
.pt-community-qr { flex: 0 0 auto; }
.pt-community-qr summary { display: grid; place-items: center; width: 44px; height: 44px; cursor: pointer; list-style: none; background: #fff; padding: 5px; clip-path: polygon(0 0,100% 0,100% calc(100% - 6px),calc(100% - 6px) 100%,0 100%); }
.pt-community-qr summary::-webkit-details-marker { display: none; }
.pt-community-qr summary img { display: block; width: 32px; height: 32px; }
.pt-community-qr[open] summary { background: var(--ef-signal); }
.pt-community-copy:focus-visible, .pt-community-qr summary:focus-visible, .pt-community-preview button:focus-visible { outline: 2px solid var(--pt-ink); outline-offset: -2px; }
.pt-community-preview { position: absolute; z-index: 30; bottom: calc(100% + 12px); inset-inline-end: 0; width: 226px; padding: 12px; border: 1px solid var(--pt-ink); border-top-width: 3px; background: var(--pt-panel); }
.pt-community-preview::after { content: ''; position: absolute; top: 100%; inset-inline-end: 27px; height: 12px; width: 1px; background: var(--pt-ink); }
.pt-community-preview-head { display: flex; align-items: center; justify-content: space-between; gap: 10px; margin-bottom: 10px; font-size: 12px; font-weight: 700; }
.pt-community-preview button { display: grid; place-items: center; width: 28px; height: 28px; background: var(--pt-chip); }
.pt-community-preview > img { display: block; width: 200px; height: 200px; background: #fff; }
.pt-community-preview > p { margin-top: 10px; padding: 5px 7px; background: var(--ef-signal); color: var(--ef-on-signal); font-size: 12px; font-weight: 700; font-variant-numeric: tabular-nums; }
@media (pointer: coarse) { .pt-community-copy { min-height: 44px; } .pt-community-preview button { width: 44px; height: 44px; } }
</style>
