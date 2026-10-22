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
    class="ef ef-contours flex min-h-screen flex-col"
  >
    <!-- ============ 命令栏 ============ -->
    <header class="ef-cmdbar">
      <router-link to="/" class="brand">
        <span class="seal" aria-hidden="true">
          <img v-if="siteLogo" :src="siteLogo" alt="" class="h-5 w-5 object-contain" />
          <template v-else>▌</template>
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
          {{ isDark ? t('home.themeLight') : t('home.themeDark') }}
        </button>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="ef-btn go">
          {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
        </router-link>
      </div>
    </header>

    <div class="ef-tri ef-in-sweep" aria-hidden="true"><i></i><i></i><i></i></div>

    <main class="flex-1">
      <!-- ============ 首屏 ============ -->
      <section class="mx-auto w-full max-w-6xl px-[var(--ef-pad)] pb-14 pt-10 md:pt-14">
        <p class="ef-eyebrow slash ef-in-stroke">
          {{ t('home.eyebrowIndex') }}<template v-if="todayLabel"> · {{ todayLabel }}</template>
        </p>

        <div class="mt-7 grid gap-10 lg:grid-cols-[minmax(0,1fr)_22rem] lg:items-start lg:gap-14">
          <div class="min-w-0">
            <h1 class="m-0">
              <span class="ef-wordmark">
                <span class="solid ef-in-power" style="--i: 0">{{ siteName }}</span>
                <span class="hollow ef-in-power" style="--i: 1">{{ siteSubtitle }}</span>
              </span>
            </h1>

            <p class="ef-lede ef-in-come" style="--i: 2">{{ t('home.heroDescription') }}</p>

            <div class="mt-7 flex flex-wrap items-center gap-2.5">
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

            <ul class="ef-facts mt-9 ef-in-come" style="--i: 3">
              <li class="ef-field">{{ t('home.tags.subscriptionToApi') }}</li>
              <li class="ef-field">{{ t('home.tags.stickySession') }}</li>
              <li class="ef-field">{{ t('home.tags.realtimeBilling') }}</li>
            </ul>
          </div>

          <!-- 右：站点真实字段（替换原装饰性假终端） -->
          <aside class="ef-panel cut-br ef-in-come" style="--i: 2">
            <h2>
              {{ t('home.eyebrowStatus') }}
              <small v-if="appStore.cachedPublicSettings?.version">
                v{{ appStore.cachedPublicSettings.version }}
              </small>
            </h2>
            <ul class="ef-ledger">
              <li v-for="f in facts" :key="f.label">
                <span>{{ f.label }}</span>
                <b>{{ f.value }}</b>
              </li>
            </ul>
          </aside>
        </div>
      </section>

      <!-- ============ 能力 ============ -->
      <section class="mx-auto w-full max-w-6xl px-[var(--ef-pad)] pb-14">
        <div class="ef-group-head">
          <span class="ef-eyebrow slash">{{ t('home.eyebrowCapabilities') }}</span>
          <span class="ef-field">No.01</span>
        </div>
        <ul class="ef-records">
          <li v-for="(c, i) in capabilities" :key="c.title">
            <div class="ef-record">
              <span class="ef-field">{{ String(i + 1).padStart(2, '0') }}</span>
              <span class="ef-tag">{{ c.title }}</span>
              <span class="name ef-read">{{ c.desc }}</span>
              <span class="ef-field" aria-hidden="true">{{ c.mark }}</span>
            </div>
          </li>
        </ul>
      </section>

      <!-- ============ 运行状态 ============ -->
      <section class="mx-auto w-full max-w-6xl px-[var(--ef-pad)] pb-16">
        <div class="ef-group-head">
          <span class="ef-eyebrow slash">{{ t('home.eyebrowStatus') }}</span>
          <span class="ef-field">No.02</span>
        </div>

        <div class="mt-5 flex flex-wrap items-center gap-x-8 gap-y-3">
          <span class="flex items-center gap-2">
            <span class="ef-field">{{ t('home.status.monitors') }}</span>
            <span class="ef-state" :class="monitorsOn ? '' : 'is-draft'">
              {{ monitorsOn ? t('home.value.enabled') : t('home.value.disabled') }}
            </span>
          </span>
          <span class="flex items-center gap-2">
            <span class="ef-field">{{ t('home.status.plaza') }}</span>
            <span class="ef-state" :class="plazaOn ? '' : 'is-draft'">
              {{ plazaOn ? t('home.value.enabled') : t('home.value.disabled') }}
            </span>
          </span>
        </div>

        <ul v-if="monitorRows.length" class="ef-records mt-6">
          <li v-for="m in monitorRows" :key="m.id">
            <div class="ef-record">
              <span class="ef-field">{{ m.provider }}</span>
              <span class="ef-tag">{{ m.name }}</span>
              <span class="name ef-read">{{ m.model }}</span>
              <span class="ef-field">{{ m.availability }}</span>
            </div>
          </li>
        </ul>
        <p v-else class="ef-empty">
          {{ isAuthenticated ? t('home.status.noMonitors') : t('home.status.signedOutHint') }}
        </p>
      </section>
    </main>

    <!-- ============ 页脚 ============ -->
    <footer class="border-t border-[var(--ef-rule)] px-[var(--ef-pad)] py-7">
      <div class="mx-auto flex w-full max-w-6xl flex-wrap items-start justify-between gap-5">
        <div>
          <p class="ef-eyebrow">{{ siteName }}</p>
          <p class="ef-field mt-1.5">{{ t('home.footerTagline') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-x-6 gap-y-1.5">
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="ef-field">
            {{ t('home.docs') }}
          </a>
          <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="ef-field">
            GitHub
          </a>
          <span class="ef-field">
            © {{ currentYear }} {{ siteName }} · {{ t('home.footer.allRightsReserved') }}
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
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { isEfDark, setEfTheme } from '@/utils/theme'
import { channelMonitorUserAPI, type UserMonitorView } from '@/api/channelMonitor'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
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

/** 侧栏字段表：全部来自 cachedPublicSettings，缺失一律显示 "—" */
const facts = computed(() => {
  const s = appStore.cachedPublicSettings
  const dash = '—'
  const tri = (v: boolean | undefined, on: string, off: string) =>
    v === true ? on : v === false ? off : dash
  return [
    { label: t('home.field.version'), value: s?.version || dash },
    { label: t('home.field.timezone'), value: s?.server_timezone || dash },
    {
      label: t('home.field.registration'),
      value: tri(s?.registration_enabled, t('home.value.open'), t('home.value.closed')),
    },
    {
      label: t('home.field.invitation'),
      value: tri(s?.invitation_code_enabled, t('home.value.required'), t('home.value.none')),
    },
    {
      label: t('home.field.subscription'),
      value: tri(s?.subscription_enabled, t('home.value.enabled'), t('home.value.disabled')),
    },
  ]
})

/** 三条能力：文案走 i18n，序号右侧的短码只用拉丁字母保证等宽 */
const capabilities = computed(() => [
  { title: t('home.capability.access'), desc: t('home.capability.accessDesc'), mark: 'KEY' },
  { title: t('home.capability.reliable'), desc: t('home.capability.reliableDesc'), mark: 'SLA' },
  { title: t('home.capability.billing'), desc: t('home.capability.billingDesc'), mark: 'USD' },
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
/* LocaleSwitcher 原本是浅色页头组件，放进固定深色的 .ef-cmdbar 后
   需要把它的灰字与白底下拉面板改成命令栏的配色，否则几乎不可见。 */
.ef-cmdbar-i18n :deep(button) {
  color: #b8b8b8;
  border-radius: 0;
  transition: color 0.15s;
}
.ef-cmdbar-i18n :deep(button:hover) {
  color: var(--ef-signal);
  background: transparent;
}
/* 下拉面板：深色底 + 浅色字，去掉圆角与投影 */
.ef-cmdbar-i18n :deep(.absolute) {
  border-radius: 0;
  box-shadow: none;
  border-color: #353535;
  background: #232323;
}
.ef-cmdbar-i18n :deep(.absolute button),
.ef-cmdbar-i18n :deep(.absolute button:hover) {
  color: #ededed;
  background: transparent;
  border-radius: 0;
}
.ef-cmdbar-i18n :deep(.absolute button:hover) {
  background: #353535;
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
  text-decoration-color: var(--ef-signal);
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