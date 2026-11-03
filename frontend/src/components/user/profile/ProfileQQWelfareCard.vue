<template>
  <section class="welfare-panel" aria-labelledby="qq-welfare-title">
    <div class="welfare-section-head"><div><span class="welfare-eyebrow">// QQ VERIFICATION</span><h2 id="qq-welfare-title">{{ t('qqWelfare.bindingTitle') }}</h2></div><span v-if="status" class="welfare-tag">{{ t(status.verified ? 'qqWelfare.verified' : 'qqWelfare.notVerified') }}</span></div>
    <p>{{ t('qqWelfare.bindingDescription') }}</p>
    <p v-if="error" role="alert" class="welfare-notice">{{ error }}</p>
    <p v-if="loading" aria-live="polite">{{ t('qqWelfare.loading') }}</p>
    <template v-else-if="status">
      <p v-if="status.verified">{{ t('qqWelfare.bindingDone') }}</p>
      <p v-else-if="!status.ready" class="welfare-notice">{{ t('qqWelfare.unavailable') }}</p>
      <template v-else>
        <p>{{ t('qqWelfare.generateHint') }}</p>
        <div v-if="code" class="welfare-code-box"><p>{{ t('qqWelfare.verifyHint', {group: status.group_number}) }}</p><code>{{ command }}</code><p>{{ t('qqWelfare.codeExpires') }}</p><button class="ef-btn" type="button" @click="copyToClipboard(command)">{{ t('qqWelfare.copy') }}</button></div>
        <p v-if="expired" role="status">{{ t('qqWelfare.expired') }}</p>
        <button class="ef-btn go" type="button" :disabled="busy || !!code" @click="generate">{{ t('qqWelfare.generate') }}</button>
      </template>
    </template>
    <button class="ef-btn" type="button" :disabled="loading || busy" @click="load()">{{ t('qqWelfare.refresh') }}</button>
  </section>
</template>
<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { qqWelfare, type WelfareBinding } from '@/api/qqWelfare'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/endfield/qq-welfare.css'
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const status = ref<WelfareBinding | null>(null)
const loading = ref(true), busy = ref(false), error = ref(''), code = ref(''), expired = ref(false)
const command = computed(() => `${t('qqWelfare.verifyCommand')} ${code.value}`)
let deadline = 0, interval: ReturnType<typeof setInterval> | undefined
async function load(silent = false) {
  if (!silent) loading.value = true
  error.value = ''
  try { status.value = await qqWelfare.binding(); if (status.value.verified) code.value = '' }
  catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') }
  finally { loading.value = false }
}
async function generate() {
  busy.value = true; error.value = ''
  try { const data = await qqWelfare.bindingCode(); code.value = data.code; deadline = Date.now() + data.expires_in * 1000; expired.value = false }
  catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') }
  finally { busy.value = false }
}
onMounted(() => { void load(); interval = setInterval(() => {
  if (!code.value) return
  if (Date.now() >= deadline) { code.value = ''; expired.value = true; return }
  if (!document.hidden && !loading.value) void load(true)
}, 5000) })
onUnmounted(() => { clearInterval(interval) })
</script>
