import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import ProfileQQWelfareCard from '../ProfileQQWelfareCard.vue'
const api=vi.hoisted(()=>({binding:vi.fn(),bindingCode:vi.fn()}))
vi.mock('@/api/qqWelfare',()=>({qqWelfare:api}))
vi.mock('vue-i18n',()=>({useI18n:()=>({t:(key:string)=>key})}))
vi.mock('@/composables/useClipboard',()=>({useClipboard:()=>({copyToClipboard:vi.fn()})}))
describe('QQ account verification',()=>{
 beforeEach(()=>{vi.clearAllMocks()})
 it('gates code creation until an administrator connects the group',async()=>{
  api.binding.mockResolvedValue({ready:false,verified:false,group_number:'487375322'})
  const wrapper=mount(ProfileQQWelfareCard);await flushPromises()
  expect(wrapper.text()).toContain('qqWelfare.unavailable')
  expect(wrapper.findAll('button').some(b=>b.text()==='qqWelfare.generate')).toBe(false)
  expect(api.bindingCode).not.toHaveBeenCalled();wrapper.unmount()
 })
 it('shows a single-use command and stops offering generation after verification',async()=>{
  api.binding.mockResolvedValue({ready:true,verified:false,group_number:'487375322'})
  api.bindingCode.mockResolvedValue({code:'PRIVATE_VERIFICATION',expires_in:600})
  const wrapper=mount(ProfileQQWelfareCard);await flushPromises()
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.generate')!.trigger('click');await flushPromises()
  expect(wrapper.find('code').text()).toContain('PRIVATE_VERIFICATION')
  api.binding.mockResolvedValue({ready:true,verified:true,group_number:'487375322'})
  await wrapper.findAll('button').find(b=>b.text()==='qqWelfare.refresh')!.trigger('click');await flushPromises()
  expect(wrapper.text()).not.toContain('PRIVATE_VERIFICATION')
  expect(wrapper.text()).toContain('qqWelfare.bindingDone');wrapper.unmount()
 })
})
