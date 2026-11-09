import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import UserBalanceModal from '../UserBalanceModal.vue'
import type { AdminUser } from '@/types'
const mocks = vi.hoisted(() => ({ updateBalance: vi.fn(), showError: vi.fn(), showSuccess: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { users: mocks } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => mocks }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
enableAutoUnmount(afterEach)
beforeEach(() => { vi.clearAllMocks(); vi.spyOn(console, 'error').mockImplementation(() => {}) })
afterEach(() => vi.restoreAllMocks())
describe('balance operation error feedback', () => {
  it.each([
    [{ message: 'User balance changed; retry' }, 'User balance changed; retry'],
    [{ response: { data: { detail: 'Legacy detail' } } }, 'Legacy detail'],
    [{}, 'common.error'],
  ])('shows the available API error and keeps the dialog open', async (error, message) => {
    mocks.updateBalance.mockRejectedValueOnce(error)
    const w = mount(UserBalanceModal, { props: { show: true, operation: 'add', user: { id: 1, email: 'a@example.com', balance: 5 } as AdminUser },
      global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } } })
    await w.get('input[type="number"]').setValue('1')
    await w.get('form').trigger('submit'); await flushPromises()
    expect(mocks.showError).toHaveBeenCalledWith(message)
    expect(w.emitted('success')).toBeUndefined(); expect(w.emitted('close')).toBeUndefined()
    expect(w.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
  })
})

it('classifies administrator deposits and resets the source for the next user',async()=>{
 mocks.updateBalance.mockResolvedValue({})
 const w=mount(UserBalanceModal,{props:{show:true,operation:'add',user:{id:1,email:'a@example.com',balance:5} as AdminUser},global:{stubs:{BaseDialog:{template:'<div><slot /><slot name="footer" /></div>'}}}})
 expect((w.get('select').element as HTMLSelectElement).value).toBe('gift')
 await w.get('input[type=number]').setValue('10');await w.get('select').setValue('paid')
 await w.get('form').trigger('submit');await flushPromises()
 expect(mocks.updateBalance).toHaveBeenLastCalledWith(1,10,'add','','paid')
 await w.setProps({show:false});await w.setProps({show:true})
 expect((w.get('select').element as HTMLSelectElement).value).toBe('gift')
 await w.setProps({operation:'subtract'});expect(w.find('select').exists()).toBe(false)
})
