<template>
  <span class="pt-loading" :class="[`pt-loading--${size}`, `pt-loading--${color}`]" role="status" :aria-label="t('common.loading')">
    <span class="pt-loading-track" aria-hidden="true"><i v-for="segment in 4" :key="segment"></i><b></b></span>
  </span>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

type SpinnerSize = 'sm' | 'md' | 'lg' | 'xl'
type SpinnerColor = 'primary' | 'secondary' | 'white' | 'gray'

interface Props {
  size?: SpinnerSize
  color?: SpinnerColor
}

withDefaults(defineProps<Props>(), {
  size: 'md',
  color: 'primary'
})

</script>

<style scoped>
.pt-loading { --loading-size: 32px; display: inline-flex; align-items: center; justify-content: center; flex: 0 0 auto; width: var(--loading-size); height: var(--loading-size); vertical-align: middle; color: var(--ef-signal, #fffa00); }
.pt-loading--sm { --loading-size: 16px; }
.pt-loading--lg { --loading-size: 48px; }
.pt-loading--xl { --loading-size: 64px; }
.pt-loading--secondary, .pt-loading--gray { color: #fff; }
.pt-loading--white { color: #fff; }
.pt-loading-track { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8%; position: relative; width: 100%; height: 56%; padding: 8%; background: #161616; }
.pt-loading-track i { background: currentColor; opacity: .28; }
.pt-loading-track b { position: absolute; inset: 14% auto 14% 8%; width: 15%; background: currentColor; }
@media (prefers-reduced-motion: no-preference) {
  .pt-loading-track b { animation: pt-loading-step 960ms steps(3, end) infinite; }
}
@keyframes pt-loading-step { from { transform: translateX(0); } to { transform: translateX(460%); } }
@media (prefers-reduced-motion: reduce) {
  .pt-loading-track i { opacity: 1; }
  .pt-loading-track b { display: none; }
}
</style>
