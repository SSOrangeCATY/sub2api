<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="flex min-h-screen flex-col bg-gray-50 text-gray-900 dark:bg-dark-950 dark:text-white"
  >
    <header class="border-b border-gray-200 px-4 py-4 sm:px-6 dark:border-dark-800">
      <nav class="mx-auto flex max-w-5xl flex-wrap items-center justify-between gap-3 sm:gap-4">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="flex h-10 shrink-0 items-center gap-1.5 rounded-lg px-2.5 text-sm font-medium text-gray-500 hover:bg-gray-100 hover:text-gray-700 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg text-gray-500 hover:bg-gray-100 dark:text-dark-400 dark:hover:bg-dark-800"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="inline-flex min-h-10 shrink-0 items-center justify-center rounded-lg bg-gray-900 px-4 py-2 text-sm font-medium text-white hover:bg-gray-800 dark:bg-white dark:text-gray-900 dark:hover:bg-gray-200"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="flex min-w-0 flex-1 items-center justify-center px-4 py-16 sm:px-6">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="[overflow-wrap:anywhere] text-3xl font-bold md:text-4xl">{{ siteName }}</h1>
        <p class="mt-4 whitespace-pre-wrap [overflow-wrap:anywhere] text-base text-gray-600 dark:text-dark-300">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="mt-8 inline-flex min-h-10 items-center justify-center rounded-lg bg-primary-600 px-5 py-2.5 text-sm font-medium text-white hover:bg-primary-700"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="min-w-0 border-t border-gray-200 px-4 py-5 text-center text-sm text-gray-500 [overflow-wrap:anywhere] sm:px-6 dark:border-dark-800 dark:text-dark-400">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div
    v-else
    class="ef ef-default-home pt-home-refined ef-contours flex min-h-screen flex-col"
  >
    <!-- ============ 命令栏 ============ -->
    <header class="ef-cmdbar">
      <router-link to="/" class="brand">
        <span class="seal" aria-hidden="true">
          <img :src="siteLogo || DEFAULT_BRAND_MARK" alt="" class="h-full w-full object-contain" />
        </span>
        <span>{{ siteName }}</span>
      </router-link>

      <nav aria-label="primary">
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">
          <small>DOCS</small>
          {{ t('home.docs') }}
        </a>
        <router-link v-if="showModelPlazaEntry" to="/model-plaza">
          <small>PLAZA</small>
          {{ t('nav.modelPlaza') }}
        </router-link>
      </nav>

      <div class="util">
        <LocaleSwitcher class="ef-cmdbar-i18n" />
        <button
          type="button"
          class="ef-state"
          :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          :aria-label="isDark ? t('home.switchToLight') : t('home.switchToDark')"
          @click="toggleTheme"
        >
          <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          <span class="hidden sm:inline">{{ isDark ? t('home.themeLight') : t('home.themeDark') }}</span>
        </button>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="ef-btn go">
          {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
        </router-link>
      </div>
    </header>

    <main class="flex-1">
      <!-- ============ 首屏 ============ -->
      <section class="pt-home-intro mx-auto w-full max-w-6xl px-[var(--ef-pad)]">
        <div class="pt-home-overline">
          <p>// {{ t('home.eyebrowIndex') }}</p>
          <time v-if="todayLabel" :datetime="todayLabel.replace(/\./g, '-')">{{ todayLabel }}</time>
        </div>

        <div class="pt-home-hero">
          <div class="min-w-0">
            <h1 class="m-0">
              <span class="ef-wordmark">
                <span class="solid ef-in-power" style="--i: 0">{{ siteName }}</span>
                <span v-if="wordmarkSub" class="hollow ef-in-power" style="--i: 1">{{ wordmarkSub }}</span>
              </span>
            </h1>

            <h2 class="pt-home-promise">{{ t('home.heroSubtitle') }}</h2>
            <p class="ef-lede ef-in-come" style="--i: 2">
              <span v-if="subtitleIsDescription" class="home-subtitle">{{ siteSubtitle }}</span>
              {{ t('home.heroDescription') }}
            </p>

            <div class="pt-home-hero-actions flex flex-wrap items-center gap-2.5">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="ef-btn go"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="sm" :stroke-width="2" />
              </router-link>
              <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="ef-btn">
                {{ t('home.viewDocs') }}
              </a>
            </div>
          </div>
        </div>
        <div class="pt-home-hero-tail">
          <ul class="pt-home-tags">
            <li class="ef-field">{{ t('home.tags.subscriptionToApi') }}</li>
            <li class="ef-field">{{ t('home.tags.stickySession') }}</li>
            <li class="ef-field">{{ t('home.tags.realtimeBilling') }}</li>
          </ul>
          <HomeCommunityCard />
        </div>
      </section>

      <!-- ============ 能力 ============ -->
      <section class="pt-home-section mx-auto w-full max-w-6xl px-[var(--ef-pad)]">
        <div class="pt-home-section-head">
          <div>
            <p>// CAPABILITIES</p>
            <h2>{{ t('home.eyebrowCapabilities') }}</h2>
          </div>
          <span class="ef-field">01 / 02</span>
        </div>
        <ul class="pt-capabilities">
          <li v-for="(c, i) in capabilities" :key="c.title" class="pt-capability">
            <div class="pt-capability-mark">
              <Icon :name="i === 0 ? 'key' : i === 1 ? 'server' : 'dollar'" size="lg" />
              <span>{{ String(i + 1).padStart(2, '0') }}</span>
            </div>
            <h3>{{ c.title }}</h3>
            <p>{{ c.desc }}</p>
          </li>
        </ul>
      </section>

      <!-- ============ 运行状态 ============ -->
      <section class="pt-home-section pt-home-channels mx-auto w-full max-w-6xl px-[var(--ef-pad)]">
        <div class="pt-home-section-head">
          <div>
            <p>// CHANNEL STATUS</p>
            <h2>{{ t('home.eyebrowStatus') }}</h2>
          </div>
          <span class="ef-field">02 / 02</span>
        </div>

        <div class="pt-home-feature-flags flex flex-wrap items-center gap-x-8 gap-y-3">
          <span class="flex items-center gap-2">
            <span class="ef-field">{{ t('home.status.monitors') }}</span>
            <span class="pt-status" :class="monitorsOn ? '' : 'pt-status--muted'">
              {{ monitorsOn ? t('home.value.enabled') : t('home.value.disabled') }}
            </span>
          </span>
          <span class="flex items-center gap-2">
            <span class="ef-field">{{ t('home.status.plaza') }}</span>
            <span class="pt-status" :class="plazaOn ? '' : 'pt-status--muted'">
              {{ plazaOn ? t('home.value.enabled') : t('home.value.disabled') }}
            </span>
          </span>
        </div>

        <ul v-if="monitorRows.length" class="pt-home-monitor-list">
          <li v-for="m in monitorRows" :key="m.id">
            <div class="pt-home-monitor-row">
              <span class="pt-home-monitor-name">{{ m.name }}</span>
              <span class="pt-home-monitor-model">
                <small>{{ m.provider }}</small>
                <span>{{ m.model }}</span>
              </span>
              <span class="pt-home-monitor-availability">
                <strong>{{ m.availability }}</strong>
                <small>{{ t('channelStatus.columns.availability7d') }}</small>
              </span>
            </div>
          </li>
        </ul>
        <p v-else class="pt-home-monitor-empty">
          {{ isAuthenticated ? t('home.status.noMonitors') : t('home.status.signedOutHint') }}
        </p>
      </section>
    </main>

    <!-- ============ 页脚 ============ -->
    <footer class="pt-home-footer border-t border-[var(--ef-rule)] px-[var(--ef-pad)] py-7">
      <div class="mx-auto flex w-full max-w-6xl flex-wrap items-start justify-between gap-5">
        <div>
          <p class="ef-eyebrow ef-site-name">{{ siteName }}</p>
          <p class="ef-field mt-1.5">{{ t('home.footerTagline') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-x-6 gap-y-1.5">
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="ef-field">
            {{ t('nav.modelPlaza') }}
          </router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="ef-field">
            {{ t('home.docs') }}
          </a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="ef-field">
            GitHub
          </a>
          <span class="ef-field">
            © {{ currentYear }} <span class="ef-site-name">{{ siteName }}</span> · {{ t('home.footer.allRightsReserved') }}
          </span>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import HomeCommunityCard from '@/components/home/HomeCommunityCard.vue'
import { sanitizeUrl } from '@/utils/url'
import { DEFAULT_BRAND_MARK } from '@/utils/branding'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { isEfDark, setEfTheme } from '@/utils/theme'
import { channelMonitorUserAPI, type UserMonitorView } from '@/api/channelMonitor'
import '@/styles/endfield/home.css'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'ptcrys ai')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle ?? 'gateway')
// 字标只承载短副名；后台填写的长描述仍完整放在正文中。
const wordmarkSub = computed(() => {
  const raw = siteSubtitle.value.trim()
  if (!raw) return ''
  const head = raw.split(/[·|/—–-]/)[0].trim()
  return Array.from(head).length <= 24 ? head : 'gateway'
})
const subtitleIsDescription = computed(() => siteSubtitle.value.trim() !== wordmarkSub.value)
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

// ==================== 展示型首页（P1） ====================
/** 当天日期，用于 eyebrow 右上角的地名式日期戳 */
const todayLabel = computed(() => {
  const d = new Date()
  const p = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}.${p(d.getMonth() + 1)}.${p(d.getDate())}`
})

/** 功能开关：直接读公共设置，不发新请求 */
const monitorsOn = computed(() => appStore.cachedPublicSettings?.channel_monitor_enabled === true)
const plazaOn = computed(() => appStore.cachedPublicSettings?.model_plaza_enabled === true)

/** 三条能力：文案走 i18n，模板按实际顺序编号。 */
const capabilities = computed(() => [
  { title: t('home.capability.access'), desc: t('home.capability.accessDesc') },
  { title: t('home.capability.reliable'), desc: t('home.capability.reliableDesc') },
  { title: t('home.capability.billing'), desc: t('home.capability.billingDesc') },
])

/** 渠道监控行：仅登录且开关开启时才拉取，匿名请求会 401 */
const monitorViews = ref<UserMonitorView[]>([])
const monitorRows = computed(() =>
  monitorViews.value.slice(0, 6).map((m) => ({
    id: m.id,
    provider: String(m.provider || '').toUpperCase() || '—',
    name: m.name || '—',
    model: m.primary_model || '—',
    availability:
      typeof m.availability_7d === 'number' ? `${(m.availability_7d * 100).toFixed(1)}%` : '—',
  })),
)

async function loadMonitors() {
  if (!isAuthenticated.value || !monitorsOn.value) return
  try {
    const res = await channelMonitorUserAPI.list()
    monitorViews.value = Array.isArray(res?.items) ? res.items : []
  } catch {
    // 首页不能因为监控接口失败而报错，静默降级为空态
    monitorViews.value = []
  }
}

// Toggle theme
function toggleTheme() {
  isDark.value = !isDark.value
  setEfTheme(isDark.value)
}

// Initialize theme（main.ts 已写好 DOM，这里只同步本地状态）
function initTheme() {
  isDark.value = isEfDark()
}

onMounted(() => {
  initTheme()

  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }

  // 渠道监控：等公共设置到位后再决定是否拉取
  void loadMonitors()
})
</script>

<style scoped>
.ef-default-home .ef-btn.go {
  --fill: var(--ef-signal);
  color: var(--ef-on-signal);
}
.ef-default-home .ef-btn.go:focus-visible {
  outline-color: var(--ef-on-signal);
}
.ef-default-home a:hover {
  text-decoration-color: var(--ef-trace);
}
.ef-default-home :deep(.ef-state.is-alert) {
  border-color: var(--ef-on-signal);
}
.ef-default-home :deep(.ef-prose a) {
  text-decoration-color: var(--ef-trace);
  text-decoration-thickness: 1px;
  text-underline-offset: 0.25em;
  text-decoration-skip-ink: auto;
}

.ef-wordmark,
.ef-cmdbar .brand,
.ef-site-name {
  text-transform: none;
}

.ef-wordmark {
  --ef-wordmark: clamp(2.75rem, 7.5vw, 6rem);
  max-width: 100%;
}
.ef-wordmark .solid {
  overflow-wrap: anywhere;
  line-height: 0.95;
}
.ef-wordmark .hollow {
  margin-top: 0.35em;
  font-size: clamp(1.25rem, 3vw, 2rem);
  line-height: 1.1;
  overflow-wrap: anywhere;
  -webkit-text-stroke: 1px var(--ef-ink);
}
.home-subtitle {
  display: block;
  margin-bottom: 0.5rem;
  overflow-wrap: anywhere;
  font-size: 0.9375rem;
  line-height: 1.5;
}
.ef-cmdbar .ef-state {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
}
@media (max-width: 719px) {
  .ef-cmdbar {
    height: 55px;
    flex-wrap: nowrap;
    align-items: center;
    gap: 0.5rem;
    padding-inline: 1rem;
  }
  .ef-cmdbar .brand {
    min-width: 0;
    margin-right: auto;
    gap: 0.5rem;
    padding-block: 0;
    font-size: 0.875rem;
  }
  .ef-cmdbar .brand > span:last-child {
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }
  .ef-cmdbar .seal {
    width: 1.625rem;
    height: 1.625rem;
    flex-shrink: 0;
  }
  .ef-cmdbar nav {
    display: none;
  }
  .ef-cmdbar .util {
    flex-shrink: 0;
    gap: 0.5rem;
  }
  .ef-cmdbar .ef-state {
    min-width: 2rem;
    min-height: 2rem;
    justify-content: center;
  }
  .ef-cmdbar .ef-btn {
    padding-inline: 0.75rem;
  }
}

/* LocaleSwitcher 原本是浅色页头组件，放进固定深色的 .ef-cmdbar 后
   需要把它的灰字与白底下拉面板改成命令栏的配色，否则几乎不可见。

   .ef-cmdbar 在明暗两种主题下都是同一个深色（内部写死 #161616 / #ededed），
   且没有把这几级色暴露成变量，所以这里按它的取值就地声明一组局部变量：
   不能用 --ef-ink / --ef-panel —— 那两个在深色主题下会翻转，命令栏不会。 */
.ef-cmdbar-i18n {
  --cmd-bg: #161616;
  --cmd-fg: #ededed;
  --cmd-fg-2: #b8b8b8;
  --cmd-line: #353535;
  --cmd-panel: #232323;
}
.ef-cmdbar-i18n :deep(button) {
  color: var(--cmd-fg-2);
  border-radius: 0;
  transition: color 0.15s;
}
.ef-cmdbar-i18n :deep(button:hover) {
  color: var(--ef-on-signal);
  background: var(--ef-signal);
}
/* 下拉面板：深色底 + 浅色字，去掉圆角与投影 */
.ef-cmdbar-i18n :deep(.absolute) {
  border-radius: 0;
  box-shadow: none;
  border-color: var(--cmd-line);
  background: var(--cmd-panel);
}
.ef-cmdbar-i18n :deep(.absolute button),
.ef-cmdbar-i18n :deep(.absolute button:hover) {
  color: var(--cmd-fg);
  background: transparent;
  border-radius: 0;
}
.ef-cmdbar-i18n :deep(.absolute button:hover) {
  background: var(--cmd-line);
}

.code-prompt {
  color: var(--ef-ink);
  font-weight: 700;
}
.code-cmd {
  color: var(--ef-ink);
  font-weight: 600;
}
.code-flag {
  color: var(--ef-ink-2);
}
.code-url {
  color: var(--ef-ink);
  text-decoration: underline;
  text-decoration-thickness: 1px;
  text-underline-offset: 0.25em;
  text-decoration-color: var(--ef-trace);
}
.code-comment {
  color: var(--ef-ink-2);
  font-style: italic;
}
.code-success {
  color: var(--ef-on-signal);
  background: var(--ef-signal);
  padding: 0 6px;
  border-radius: 0;
  font-weight: 700;
}
.code-response {
  color: var(--ef-ink-2);
}

/* Blinking Cursor */
.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: var(--ef-ink);
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%,
  50% {
    opacity: 1;
  }
  51%,
  100% {
    opacity: 0;
  }
}

/* Dark mode adjustments */
:deep(.dark) .terminal-window {
  box-shadow:
    0 25px 50px -12px rgba(0, 0, 0, 0.6),
    0 0 0 1px rgba(20, 184, 166, 0.2),
    0 0 40px rgba(20, 184, 166, 0.1),
    inset 0 1px 0 rgba(255, 255, 255, 0.1);
}
</style>
