<template>
  <div class="ef-auth-layout pt-auth-surface pt-public-contours">
    <header class="pt-auth-header">
      <RouterLink to="/home" class="pt-auth-brand">
        <img :src="DEFAULT_BRAND_MARK" alt="" width="30" height="30" />
        <span>{{ siteName }}</span>
      </RouterLink>
      <RouterLink v-if="appStore.cachedPublicSettings?.model_plaza_enabled" to="/model-plaza" class="pt-auth-model-link">{{ t('modelPlaza.title') }} <span aria-hidden="true">↗</span></RouterLink>
    </header>
    <div class="pt-auth-body">
      <aside class="pt-auth-intro">
        <p class="pt-workspace-eyebrow">// {{ siteSubtitle }}</p>
        <h1 class="pt-auth-name">{{ siteName }}</h1>
        <p class="pt-auth-subtitle">gateway</p>
        <div class="pt-auth-intro-rule" aria-hidden="true"></div>
      </aside>
      <div class="pt-auth-form-column">
      <div class="pt-auth-form card-glass p-6 sm:p-8">
        <slot />
      </div>

      <!-- Footer Links -->
      <div class="mt-6 text-center text-sm">
        <slot name="footer" />
      </div>

      <!-- Copyright -->
      <div class="mt-8 text-center text-xs text-gray-400 dark:text-dark-500">
        &copy; {{ currentYear }} {{ siteName }}. All rights reserved.
      </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { RouterLink } from 'vue-router'
import { useI18n } from 'vue-i18n'
import '@/styles/endfield/public-surfaces.css'
import { useAppStore } from '@/stores'
import { DEFAULT_BRAND_MARK } from '@/utils/branding'

const appStore = useAppStore()
const { t } = useI18n()

const siteName = computed(() => appStore.siteName || 'ptcrys ai')
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'gateway')

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

