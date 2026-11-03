import { apiClient } from './client'

export interface WelfareProduct {
  id: number; version: number; title: string; points_cost: number; balance_value: number; stock: number; enabled: boolean; expires_at: string | null
}
export interface WelfareCoupon {
  id: number; code: string; title: string; balance_value: number; max_claims: number; claimed: number; enabled: boolean; expires_at: string | null
}
export interface WelfareOrder {
  id: string; user_id: number; title: string; points_cost: number; balance_value: number; created_at: string
}
export interface WelfareConfig {
  enabled: boolean; group_number: string; group_open_id: string; last_bot_seen_at: string | null
}
export interface WelfareAdmin {
  config: WelfareConfig; products: WelfareProduct[]; coupons: WelfareCoupon[]; orders: WelfareOrder[]; verified_accounts: number
}
export interface WelfareBinding { verified: boolean; ready: boolean; group_number: string }
export const qqWelfare = {
  async binding() { return (await apiClient.get<WelfareBinding>('/user/qq-welfare')).data },
  async bindingCode() { return (await apiClient.post<{code: string; expires_in: number}>('/user/qq-welfare/code')).data },
  async admin() { return (await apiClient.get<WelfareAdmin>('/admin/qq-welfare')).data },
  async config(enabled: boolean, group_number: string) { return (await apiClient.put('/admin/qq-welfare/config', {enabled, group_number})).data },
  async groupCode() { return (await apiClient.post<{code: string; expires_in: number}>('/admin/qq-welfare/group-code')).data },
  async product(product: WelfareProduct) { return (await apiClient.post<WelfareProduct>('/admin/qq-welfare/products', product)).data },
  async coupon(coupon: WelfareCoupon) { return (await apiClient.post<WelfareCoupon>('/admin/qq-welfare/coupons', coupon)).data },
}
