import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useAppStore } from '@/stores/app'
import WorkspacePageHeader from '../WorkspacePageHeader.vue'
import { workspacePresentation } from '@/utils/workspacePresentation'

const route = {
  name: 'PurchaseSubscription',
  meta: { title: 'Purchase Subscription', titleKey: 'nav.buySubscription', descriptionKey: 'purchase.description' },
}
vi.mock('vue-router', async () => ({
  ...await vi.importActual<typeof import('vue-router')>('vue-router'),
  useRoute: () => route,
}))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key, te: () => true }),
}))
enableAutoUnmount(afterEach)
beforeEach(() => setActivePinia(createPinia()))

describe('workspace heading uses the same billing mode as the page title', () => {
  it.each([
    [false, false, 'nav.recharge', 'purchase.rechargeDescription', 'RECHARGE'],
    [true, true, 'nav.subscribe', 'purchase.subscriptionDescription', 'SUBSCRIPTIONS'],
    [true, false, 'nav.buySubscription', 'purchase.description', 'RECHARGE / SUBSCRIPTIONS'],
  ] as const)('subscription=%s balanceDisabled=%s', (subscription, disabled, title, description, eyebrow) => {
    const store = useAppStore()
    store.cachedPublicSettings = { subscription_enabled: subscription, payment_balance_disabled: disabled } as NonNullable<typeof store.cachedPublicSettings>
    const wrapper = mount(WorkspacePageHeader)
    expect(wrapper.get('h1').text()).toBe(title)
    expect(wrapper.get('.pt-workspace-description').text()).toBe(description)
    expect(wrapper.get('.pt-workspace-eyebrow').text()).toBe(`// ${eyebrow}`)
  })
})

describe('workspace composition preserves page-owned headings', () => {
  it.each(['/channel-status', '/monitor', '/admin/channels/monitor'])('monitor %s retains its own heading and background', (path) => {
    expect(workspacePresentation(path)).toEqual({ surface: 'monitor', heading: false })
  })
  it.each(['/custom/redeem-code-shop', '/custom/another-page'])('custom content %s retains its own composition', (path) => {
    expect(workspacePresentation(path)).toEqual({ surface: 'embedded', heading: false })
  })
  it.each(['/dashboard', '/admin/dashboard', '/payment/result', '/batch-image'])('%s does not gain a duplicate heading', (path) => {
    expect(workspacePresentation(path).heading).toBe(false)
  })
  it('embedded model plaza receives the workspace heading', () => {
    expect(workspacePresentation('/model-plaza')).toEqual({ surface: 'records', heading: true })
  })
})
