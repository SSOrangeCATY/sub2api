<template>
  <div class="ef-app-layout min-h-screen bg-gray-50 dark:bg-dark-950" :data-page="route.path">
    <!-- Background Decoration -->
    <div class="pointer-events-none fixed inset-0 bg-mesh-gradient"></div>

    <!-- Sidebar -->
    <AppSidebar />

    <!-- Main Content Area -->
    <div
      class="relative min-h-screen transition-all duration-300"
      :class="[sidebarCollapsed ? 'lg:ml-[72px]' : 'lg:ml-64']"
    >
      <!-- Header -->
      <AppHeader class="pt-workspace-topbar" />

      <!-- Main Content -->
      <main class="pt-workspace p-4 md:p-6 lg:p-8" :data-surface="presentation.surface">
        <WorkspacePageHeader v-if="presentation.heading && route.meta.title" />
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import '@/styles/onboarding.css'
import '@/styles/endfield/workspace.css'
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/stores'
import { useAuthStore } from '@/stores/auth'
import { useOnboardingTour } from '@/composables/useOnboardingTour'
import { useOnboardingStore } from '@/stores/onboarding'
import AppSidebar from './AppSidebar.vue'
import AppHeader from './AppHeader.vue'
import WorkspacePageHeader from './WorkspacePageHeader.vue'
import { workspacePresentation } from '@/utils/workspacePresentation'

const appStore = useAppStore()
const route = useRoute()
const presentation = computed(() => workspacePresentation(route.path))
const authStore = useAuthStore()
const sidebarCollapsed = computed(() => appStore.sidebarCollapsed)
const isAdmin = computed(() => authStore.user?.role === 'admin')

const { replayTour } = useOnboardingTour({
  storageKey: isAdmin.value ? 'admin_guide' : 'user_guide',
  autoStart: true
})

const onboardingStore = useOnboardingStore()

onMounted(() => {
  onboardingStore.setReplayCallback(replayTour)
})

defineExpose({ replayTour })
</script>

<style scoped>
/* Keep shared legacy styles out of custom Home HTML while making the regular
   application use readable ink traces in the light theme. */
.ef-app-layout :deep([class*='hover:text-']:hover) {
  text-decoration-color: var(--ef-trace);
}
.ef-app-layout :deep(.ef-state.is-alert) {
  border-color: var(--ef-on-signal);
}
.ef-app-layout :deep(.driver-active-element) {
  outline-color: var(--ef-trace) !important;
}
.ef-app-layout :deep(.ef-prose a) {
  text-decoration-color: var(--ef-trace);
  text-decoration-thickness: 1px;
  text-underline-offset: 0.25em;
  text-decoration-skip-ink: auto;
}
:global(body:has(.ef-app-layout) .theme-tour-popover .driver-popover-prev-btn:hover) {
  border-color: var(--ef-on-signal) !important;
}
</style>
