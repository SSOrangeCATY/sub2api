import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import QQWelfareView from '../QQWelfareView.vue'
const api = vi.hoisted(() => ({ admin: vi.fn(), product: vi.fn(), coupon: vi.fn() }))
vi.mock('@/api/qqWelfare', () => ({ qqWelfare: api }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
const initial = {config:{enabled:true,group_number:'487375322',group_open_id:'group',last_bot_seen_at:null},products:[{id:1,version:3,title:'Reward',points_cost:1,balance_value:1,stock:9,enabled:true,expires_at:null}],coupons:[],orders:[],verified_accounts:2}
describe('QQ welfare administration', () => {
 beforeEach(() => { vi.clearAllMocks(); api.admin.mockResolvedValue(structuredClone(initial)); api.product.mockResolvedValue({}) })
 const render = () => mount(QQWelfareView,{global:{stubs:{AppLayout:{template:'<div><slot /></div>'}}}})
 it('starts new products unpublished and retains the form on rejected stale inventory',async()=>{
  const wrapper=render(); await flushPromises()
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.newProduct')!.trigger('click')
  expect((wrapper.find('form.welfare-panel input[type=checkbox]').element as HTMLInputElement).checked).toBe(false)
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.edit')!.trigger('click')
  api.product.mockRejectedValueOnce(new Error('stock changed'))
  await wrapper.find('form.welfare-panel').trigger('submit');await flushPromises()
  expect(api.product).toHaveBeenCalledWith(expect.objectContaining({id:1,version:3,stock:9}))
  expect(wrapper.find('form.welfare-panel').exists()).toBe(true)
  expect(wrapper.find('[role=alert]').exists()).toBe(true)
  wrapper.unmount()
 })
 it('creates shared codes unpublished and exposes once-per-account explanation',async()=>{
  const wrapper=render(); await flushPromises()
  await wrapper.find('#welfare-tab-coupons').trigger('click')
  expect(wrapper.text()).toContain('qqWelfare.couponHint')
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.newCoupon')!.trigger('click')
  expect((wrapper.find('form.welfare-panel input[type=checkbox]').element as HTMLInputElement).checked).toBe(false)
  wrapper.unmount()
 })
})
