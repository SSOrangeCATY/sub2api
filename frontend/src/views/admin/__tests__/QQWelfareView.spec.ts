import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import QQWelfareView from '../QQWelfareView.vue'
const api = vi.hoisted(() => ({ admin: vi.fn(), product: vi.fn(), coupon: vi.fn(), config: vi.fn(), couponClaims: vi.fn() }))
vi.mock('@/api/qqWelfare', () => ({ qqWelfare: api }))
vi.mock('vue-i18n', async (importOriginal) => ({ ...await importOriginal<typeof import('vue-i18n')>(), useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard: vi.fn() }) }))
const initial = {config:{enabled:true,require_qq_for_api:false,group_number:'487375322',group_open_id:'group',last_bot_seen_at:null},products:[{id:1,version:3,title:'Reward',points_cost:1,balance_value:1,stock:9,enabled:true,expires_at:null}],coupons:[],orders:[],verified_accounts:2}
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
 it('persists API verification independently of welfare enablement',async()=>{
  const wrapper=render();await flushPromises()
  const switches=wrapper.findAll('form input[type=checkbox]')
  await switches[0]!.setValue(false);await switches[1]!.setValue(true)
  await wrapper.find('form').trigger('submit');await flushPromises()
  expect(api.config).toHaveBeenCalledWith(false,'487375322',true)
  wrapper.unmount()
 })
 it('sends paid qualification and shows identities in paginated coupon details',async()=>{
  const coupon={id:2,code:'VIP',title:'Paid campaign',balance_value:1,min_paid_balance:10,max_claims:0,claimed:21,enabled:true,expires_at:null}
  api.admin.mockResolvedValue({...structuredClone(initial),coupons:[coupon]})
  api.couponClaims.mockResolvedValue({total:21,orders:[{id:'receipt-1',user_id:7,username:'Alice',email:'alice@example.test',title:'Paid campaign',balance_value:1,points_cost:0,created_at:'2026-11-01T00:00:00Z'}]})
  const wrapper=render();await flushPromises();await wrapper.find('#welfare-tab-coupons').trigger('click')
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.edit')!.trigger('click')
  const minimum=wrapper.get('input[max="100000000"]');await minimum.setValue('25')
  await wrapper.get('form.welfare-panel').trigger('submit');await flushPromises()
  expect(api.coupon).toHaveBeenCalledWith(expect.objectContaining({id:2,min_paid_balance:25}))
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.claimDetails')!.trigger('click');await flushPromises()
  expect(api.couponClaims).toHaveBeenLastCalledWith(2,1)
  expect(wrapper.get('[aria-label="coupon claims"]').text()).toContain('Alice')
  expect(wrapper.get('[aria-label="coupon claims"]').text()).toContain('alice@example.test')
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.next')!.trigger('click');await flushPromises()
  expect(api.couponClaims).toHaveBeenLastCalledWith(2,2)
  wrapper.unmount()
 })

})
