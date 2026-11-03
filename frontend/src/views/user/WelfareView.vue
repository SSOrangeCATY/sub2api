<template>
  <AppLayout>
    <main class="welfare-page welfare-center">
      <header class="welfare-section-head">
        <div><span class="welfare-eyebrow">// WELFARE CENTER</span><h1>{{ t('qqWelfare.centerTitle') }}</h1></div>
        <button class="ef-btn" :disabled="loading || busy" @click="load">{{ t('qqWelfare.refresh') }}</button>
      </header>
      <p v-if="error" role="alert" class="welfare-notice">{{ error }}</p>
      <p v-if="message" role="status" class="welfare-notice">{{ message }}</p>
      <p v-if="loading" aria-live="polite">{{ t('qqWelfare.loading') }}</p>
      <ProfileQQWelfareCard v-else-if="!verified" @verified="unlock" />
      <template v-else-if="snapshot">
        <section class="welfare-account" :aria-label="t('qqWelfare.myPoints')">
          <div><span class="welfare-eyebrow">// MY POINTS</span><strong>{{ snapshot.points }}<small>{{ t('qqWelfare.pointsUnit') }}</small></strong></div>
          <div><button class="ef-btn go" :disabled="busy || snapshot.checked_in" @click="checkIn">{{ t(snapshot.checked_in ? 'qqWelfare.checkedIn' : 'qqWelfare.checkIn') }}</button><p>{{ t('qqWelfare.checkinHint') }}</p></div>
        </section>
        <section class="welfare-panel" aria-labelledby="welfare-products">
          <div class="welfare-section-head"><div><span class="welfare-eyebrow">// POINT REWARDS</span><h2 id="welfare-products">{{ t('qqWelfare.products') }}</h2></div></div>
          <p v-if="!snapshot.products.length" class="welfare-empty">{{ t('qqWelfare.emptyProducts') }}</p>
          <article v-for="product in snapshot.products" :key="product.id" class="welfare-reward-row">
            <span class="welfare-row-id">{{ String(product.id).padStart(2, '0') }}</span>
            <div><h3>{{ product.title }}</h3><p>${{ product.balance_value }} / {{ t('qqWelfare.stock') }} {{ product.stock }}</p></div>
            <strong>{{ product.points_cost }} {{ t('qqWelfare.pointsUnit') }}</strong>
            <button class="ef-btn" :disabled="busy || snapshot.points < product.points_cost" @click="selected = product">{{ t('qqWelfare.exchange') }}</button>
          </article>
          <p>{{ t('qqWelfare.autoCredit') }}</p>
        </section>
        <section class="welfare-panel" aria-labelledby="welfare-coupon">
          <span class="welfare-eyebrow">// SHARED CODES</span><h2 id="welfare-coupon">{{ t('qqWelfare.coupons') }}</h2>
          <form class="welfare-claim-form" @submit.prevent="claim">
            <label class="sr-only" for="welfare-code">{{ t('qqWelfare.code') }}</label>
            <input id="welfare-code" v-model="coupon" type="text" required minlength="3" maxlength="32" autocomplete="off" :placeholder="t('qqWelfare.enterCode')" :disabled="busy" />
            <button class="ef-btn go" :disabled="busy || !coupon.trim()">{{ t('qqWelfare.claim') }}</button>
          </form><p>{{ t('qqWelfare.oncePerAccount') }}</p>
        </section>
        <section class="welfare-panel" aria-labelledby="welfare-orders">
          <span class="welfare-eyebrow">// CLAIM HISTORY</span><h2 id="welfare-orders">{{ t('qqWelfare.orders') }}</h2>
          <p v-if="!snapshot.orders.length" class="welfare-empty">{{ t('qqWelfare.emptyOrders') }}</p>
          <article v-for="order in snapshot.orders" :key="order.id" class="welfare-order-row">
            <div><h3>{{ order.title }}</h3><p>{{ formatTime(order.created_at) }}</p><details><summary>{{ t('qqWelfare.receipt') }}</summary><code>{{ order.id }}</code></details></div>
            <div><strong>+${{ order.balance_value }}</strong><p>{{ order.points_cost }} {{ t('qqWelfare.pointsUnit') }}</p></div>
          </article>
        </section>
      </template>
      <ConfirmDialog :show="!!selected" :title="t('qqWelfare.confirmExchange')" :message="selected ? `${selected.title} / ${selected.points_cost} ${t('qqWelfare.pointsUnit')} / $${selected.balance_value}` : ''" :confirm-text="t('qqWelfare.exchange')" :loading="busy" @confirm="exchange" @cancel="selected = null" />
    </main>
  </AppLayout>
</template>
<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import ProfileQQWelfareCard from '@/components/user/profile/ProfileQQWelfareCard.vue'
import { qqWelfare, type WelfareProduct, type WelfareSnapshot } from '@/api/qqWelfare'
import { useAuthStore } from '@/stores/auth'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/endfield/qq-welfare.css'
const { t, locale } = useI18n()
const auth = useAuthStore()
const loading = ref(true), busy = ref(false), verified = ref(false), error = ref(''), message = ref(''), coupon = ref('')
const snapshot = ref<WelfareSnapshot | null>(null), selected = ref<WelfareProduct | null>(null)
// Persist an unresolved request across refreshes so retrying cannot purchase twice.
const pendingKey = (payload: string) => `welfare-request:${auth.user?.id}:${payload}`
function requestID(action: string, target: string) {
  const payload = `${action}:${target}`
  const pending = sessionStorage.getItem(pendingKey(payload))
  if (pending) { try { const data = JSON.parse(pending); if (data.payload === payload) return data.id as string } catch { /* replace corrupt local state */ } }
  const id = crypto.randomUUID()
  sessionStorage.setItem(pendingKey(payload), JSON.stringify({ payload, id }))
  return id
}
async function load() {
  loading.value = true; error.value = ''
  try { const binding = await qqWelfare.binding(); verified.value = binding.verified && binding.ready; if (verified.value) snapshot.value = (await qqWelfare.center()).snapshot ?? null }
  catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') }
  finally { loading.value = false }
}
async function unlock() { if (!verified.value) await load() }
async function perform(action: 'checkin' | 'exchange' | 'claim', target = '', version = 0) {
  if (busy.value) return
  busy.value = true; error.value = ''; message.value = ''
  try {
    const id = action === 'checkin' ? '' : requestID(action, target)
    const result = await qqWelfare.action(action, id, action === 'claim' ? target : '', action === 'exchange' ? Number(target) : 0, version)
    if (action !== 'checkin') sessionStorage.removeItem(pendingKey(`${action}:${target}`))
    message.value = result.message; selected.value = null
    if (action === 'claim') coupon.value = ''
    snapshot.value = null
    try {
      snapshot.value = (await qqWelfare.center()).snapshot ?? null
      if (action !== 'checkin') await auth.refreshUser()
    } catch { error.value = t('qqWelfare.refreshFailed') }
  } catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') }
  finally { busy.value = false }
}
const checkIn = () => perform('checkin')
const exchange = () => selected.value && perform('exchange', String(selected.value.id), selected.value.version)
const claim = () => perform('claim', coupon.value.trim().toUpperCase())
const formatTime = (value: string) => new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short', timeZone: 'Asia/Shanghai' }).format(new Date(value))
const beijingDay = (date: Date) => new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Shanghai', year: 'numeric', month: '2-digit', day: '2-digit' }).format(date)
let loadedDay = beijingDay(new Date()), dayTimer: ReturnType<typeof setInterval> | undefined
function refreshOnReturn() { if (!busy.value && !loading.value && !selected.value && !document.hidden) { loadedDay = beijingDay(new Date()); void load() } }
onMounted(() => {
  void load()
  window.addEventListener('focus', refreshOnReturn)
  dayTimer = setInterval(() => { if (beijingDay(new Date()) !== loadedDay) refreshOnReturn() }, 30000)
})
onUnmounted(() => { window.removeEventListener('focus', refreshOnReturn); clearInterval(dayTimer) })
</script>
