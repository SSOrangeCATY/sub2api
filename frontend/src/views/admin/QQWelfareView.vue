<template>
  <AppLayout>
    <div class="welfare-page">
      <header class="welfare-section-head"><div><span class="welfare-eyebrow">// QQ BOT</span><h1>{{ t('qqWelfare.adminTitle') }}</h1><p>{{ t('qqWelfare.adminDescription') }}</p></div><button class="ef-btn" type="button" :disabled="busy || loading" @click="load">{{ t('qqWelfare.refresh') }}</button></header>
      <p v-if="error" class="welfare-notice" role="alert">{{ error }}</p>
      <p v-if="success" role="status">{{ success }}</p>
      <p v-if="loading">{{ t('qqWelfare.loading') }}</p>
      <template v-if="data">
        <div class="welfare-metrics"><div><label>{{ t('qqWelfare.accounts') }}</label><strong>{{ data.verified_accounts }}</strong></div><div><label>{{ t('qqWelfare.productCount') }}</label><strong>{{ data.products.length }}</strong></div><div><label>{{ t('qqWelfare.lastEvent') }}</label><strong class="is-time">{{ formatTime(data.config.last_bot_seen_at) }}</strong></div></div>
        <section class="welfare-panel">
          <div class="welfare-section-head"><h2>{{ t('qqWelfare.configTitle') }}</h2><span class="welfare-tag" :class="{'is-current': data.config.group_open_id}">{{ t(data.config.group_open_id ? 'qqWelfare.groupLinked' : 'qqWelfare.groupUnlinked') }}</span></div>
          <form @submit.prevent="saveConfig"><div class="welfare-fields"><label>{{ t('qqWelfare.group') }}<input v-model.trim="groupNumber" required inputmode="numeric" pattern="[0-9]{5,16}" maxlength="16" /></label><label class="welfare-check"><input v-model="enabled" type="checkbox" />{{ t('qqWelfare.enabled') }}</label></div><button class="ef-btn go" :disabled="busy" type="submit">{{ t('qqWelfare.saveConfig') }}</button><button class="ef-btn" :disabled="busy || !data.config.group_number" type="button" @click="generateGroupCode">{{ t('qqWelfare.bindGroup') }}</button></form>
          <p>{{ t('qqWelfare.groupHint') }}</p>
          <div v-if="groupCode" class="welfare-code-box"><code>{{ groupCommand }}</code><p>{{ t('qqWelfare.codeExpires') }}</p><button class="ef-btn" type="button" @click="copyToClipboard(groupCommand)">{{ t('qqWelfare.copy') }}</button></div>
        </section>
        <nav class="welfare-tabs" role="tablist" :aria-label="t('qqWelfare.adminTitle')"><button v-for="key in tabs" :id="`welfare-tab-${key}`" :key="key" class="ef-btn" type="button" role="tab" :aria-selected="tab === key" :aria-controls="`welfare-panel-${key}`" @click="tab = key; product = null; coupon = null">{{ t(`qqWelfare.${key}`) }}</button></nav>
        <section v-if="tab === 'products'" id="welfare-panel-products" role="tabpanel" aria-labelledby="welfare-tab-products">
          <div class="welfare-section-head"><p>{{ t('qqWelfare.rewardHint') }}</p><button class="ef-btn go" type="button" :disabled="busy" @click="product = emptyProduct()">{{ t('qqWelfare.newProduct') }}</button></div>
          <form v-if="product" class="welfare-panel" @submit.prevent="saveProduct">
            <h3>{{ t(product.id ? 'qqWelfare.edit' : 'qqWelfare.newProduct') }}</h3><div class="welfare-fields">
              <label>{{ t('qqWelfare.title') }}<input v-model.trim="product.title" required maxlength="80" /></label>
              <label>{{ t('qqWelfare.points') }}<input v-model.number="product.points_cost" required type="number" min="1" max="1000000" step="1" /></label>
              <label>{{ t('qqWelfare.value') }}<input v-model.number="product.balance_value" required type="number" min="0.00000001" max="10000" step="0.00000001" /></label>
              <label>{{ t('qqWelfare.stock') }}<input v-model.number="product.stock" required type="number" min="0" max="1000000" step="1" /></label>
              <label>{{ t('qqWelfare.expiry') }}<input v-model="expiry" type="datetime-local" /></label>
              <label class="welfare-check"><input v-model="product.enabled" type="checkbox" />{{ t('qqWelfare.publish') }}</label>
            </div><button class="ef-btn go" :disabled="busy" type="submit">{{ t('qqWelfare.save') }}</button><button class="ef-btn" :disabled="busy" type="button" @click="product = null">{{ t('qqWelfare.cancel') }}</button>
          </form>
          <div class="welfare-table-scroll"><table class="welfare-table"><thead><tr><th>ID</th><th>{{ t('qqWelfare.title') }}</th><th>{{ t('qqWelfare.points') }}</th><th>{{ t('qqWelfare.value') }}</th><th>{{ t('qqWelfare.stock') }}</th><th>{{ t('qqWelfare.status') }}</th><th>{{ t('qqWelfare.edit') }}</th></tr></thead><tbody><tr v-for="p in data.products" :key="p.id"><td>{{ p.id }}</td><td>{{ p.title }}</td><td>{{ p.points_cost }}</td><td>${{ p.balance_value }}</td><td>{{ p.stock }}</td><td>{{ t(p.enabled ? 'qqWelfare.publish' : 'qqWelfare.offline') }}</td><td><button class="ef-btn" type="button" :disabled="busy" @click="editProduct(p)">{{ t('qqWelfare.edit') }}</button></td></tr></tbody></table></div><p v-if="!data.products.length" class="welfare-empty">{{ t('qqWelfare.emptyProducts') }}</p>
        </section>
        <section v-if="tab === 'coupons'" id="welfare-panel-coupons" role="tabpanel" aria-labelledby="welfare-tab-coupons">
          <div class="welfare-section-head"><p>{{ t('qqWelfare.couponHint') }}</p><button class="ef-btn go" type="button" :disabled="busy" @click="coupon = emptyCoupon()">{{ t('qqWelfare.newCoupon') }}</button></div>
          <form v-if="coupon" class="welfare-panel" @submit.prevent="saveCoupon"><h3>{{ t(coupon.id ? 'qqWelfare.edit' : 'qqWelfare.newCoupon') }}</h3><div class="welfare-fields">
            <label>{{ t('qqWelfare.code') }}<input v-model.trim="coupon.code" required pattern="[A-Za-z0-9_-]{3,32}" maxlength="32" :readonly="!!coupon.id" /></label>
            <label>{{ t('qqWelfare.title') }}<input v-model.trim="coupon.title" required maxlength="80" /></label>
            <label>{{ t('qqWelfare.value') }}<input v-model.number="coupon.balance_value" required type="number" min="0.00000001" max="10000" step="0.00000001" /></label>
            <label>{{ t('qqWelfare.maxClaims') }}<input v-model.number="coupon.max_claims" required type="number" min="0" max="1000000" step="1" /></label>
            <label>{{ t('qqWelfare.expiry') }}<input v-model="expiry" type="datetime-local" /></label>
            <label class="welfare-check"><input v-model="coupon.enabled" type="checkbox" />{{ t('qqWelfare.publish') }}</label>
          </div><p v-if="coupon.id">{{ t('qqWelfare.couponImmutable') }}</p><button class="ef-btn go" :disabled="busy" type="submit">{{ t('qqWelfare.save') }}</button><button class="ef-btn" :disabled="busy" type="button" @click="coupon = null">{{ t('qqWelfare.cancel') }}</button></form>
          <div class="welfare-table-scroll"><table class="welfare-table"><thead><tr><th>{{ t('qqWelfare.code') }}</th><th>{{ t('qqWelfare.title') }}</th><th>{{ t('qqWelfare.value') }}</th><th>{{ t('qqWelfare.claimed') }}</th><th>{{ t('qqWelfare.status') }}</th><th>{{ t('qqWelfare.edit') }}</th></tr></thead><tbody><tr v-for="p in data.coupons" :key="p.id"><td><code>{{ p.code }}</code></td><td>{{ p.title }}</td><td>${{ p.balance_value }}</td><td>{{ p.claimed }} / {{ p.max_claims || '∞' }}</td><td>{{ t(p.enabled ? 'qqWelfare.publish' : 'qqWelfare.offline') }}</td><td><button class="ef-btn" :disabled="busy" type="button" @click="editCoupon(p)">{{ t('qqWelfare.edit') }}</button></td></tr></tbody></table></div><p v-if="!data.coupons.length" class="welfare-empty">{{ t('qqWelfare.emptyCoupons') }}</p>
        </section>
        <section v-if="tab === 'orders'" id="welfare-panel-orders" role="tabpanel" aria-labelledby="welfare-tab-orders"><div class="welfare-table-scroll"><table class="welfare-table"><thead><tr><th>{{ t('qqWelfare.receipt') }}</th><th>{{ t('qqWelfare.user') }}</th><th>{{ t('qqWelfare.title') }}</th><th>{{ t('qqWelfare.spent') }}</th><th>{{ t('qqWelfare.value') }}</th><th>{{ t('qqWelfare.time') }}</th></tr></thead><tbody><tr v-for="o in data.orders" :key="o.id"><td><code>{{ o.id }}</code></td><td>{{ o.user_id }}</td><td>{{ o.title }}</td><td>{{ o.points_cost }}</td><td>${{ o.balance_value }}</td><td>{{ formatTime(o.created_at) }}</td></tr></tbody></table></div><p v-if="!data.orders.length" class="welfare-empty">{{ t('qqWelfare.emptyOrders') }}</p></section>
        <p>{{ t('qqWelfare.commandHint') }}</p>
      </template>
    </div>
  </AppLayout>
</template>
<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { qqWelfare, type WelfareAdmin, type WelfareProduct, type WelfareCoupon } from '@/api/qqWelfare'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import '@/styles/endfield/qq-welfare.css'
const { t } = useI18n(), { copyToClipboard } = useClipboard()
const data = ref<WelfareAdmin | null>(null), busy = ref(false), loading = ref(true), error = ref(''), success = ref('')
const enabled = ref(true), groupNumber = ref(''), groupCode = ref(''), expiry = ref('')
const tabs = ['products', 'coupons', 'orders'] as const
const tab = ref<typeof tabs[number]>('products')
const product = ref<WelfareProduct | null>(null), coupon = ref<WelfareCoupon | null>(null)
const groupCommand = computed(() => `${t('qqWelfare.groupCommand')} ${groupCode.value}`)
const formatTime = (value: string | null) => value ? new Date(value).toLocaleString(undefined, {timeZone: 'Asia/Shanghai', hour12: false}) : t('qqWelfare.never')
const localDate = (value: string | null) => value ? new Date(new Date(value).getTime() - new Date(value).getTimezoneOffset() * 60000).toISOString().slice(0,16) : ''
function emptyProduct(): WelfareProduct { expiry.value = ''; return {id:0,version:0,title:'',points_cost:1,balance_value:0,stock:0,enabled:false,expires_at:null} }
function emptyCoupon(): WelfareCoupon { expiry.value = ''; return {id:0,code:'',title:'',balance_value:0,max_claims:0,claimed:0,enabled:false,expires_at:null} }
function editProduct(p: WelfareProduct) { product.value = {...p}; expiry.value = localDate(p.expires_at) }
function editCoupon(p: WelfareCoupon) { coupon.value = {...p}; expiry.value = localDate(p.expires_at) }
async function load() { loading.value = true; error.value = ''; try { data.value = await qqWelfare.admin(); enabled.value = data.value.config.enabled; groupNumber.value = data.value.config.group_number } catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') } finally { loading.value = false } }
async function run(action: () => Promise<unknown>) { busy.value = true; error.value = ''; success.value = ''; try { await action(); success.value = t('qqWelfare.saved'); await load() } catch (err) { error.value = extractApiErrorMessage(err) || t('qqWelfare.failure') } finally { busy.value = false } }
async function saveConfig() { if (data.value?.config.group_open_id && groupNumber.value !== data.value.config.group_number && !window.confirm(t('qqWelfare.groupChanged'))) return; await run(async () => { await qqWelfare.config(enabled.value, groupNumber.value); groupCode.value = '' }) }
async function generateGroupCode() { await run(async () => { const result = await qqWelfare.groupCode(); groupCode.value = result.code }) }
async function saveProduct() { if (!product.value) return; const item = {...product.value, expires_at: expiry.value ? new Date(expiry.value).toISOString() : null}; await run(async () => { await qqWelfare.product(item); product.value = null }) }
async function saveCoupon() { if (!coupon.value) return; const item = {...coupon.value, expires_at: expiry.value ? new Date(expiry.value).toISOString() : null}; await run(async () => { await qqWelfare.coupon(item); coupon.value = null }) }
onMounted(load)
</script>
