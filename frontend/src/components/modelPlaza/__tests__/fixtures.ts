import type { ModelPlazaGroup, PlazaModel } from '@/api/modelPlaza'
import type { UserSupportedModelPricing } from '@/api/channels'
export function pricing(overrides: Partial<UserSupportedModelPricing> = {}): UserSupportedModelPricing {
  return { billing_mode: 'token', input_price: 2e-6, output_price: 8e-6, cache_write_price: 2.5e-6, cache_read_price: .2e-6, cache_write_1h_price: 5e-6, cache_write_12h_price: 6e-6, image_input_price: null, image_output_price: null, per_request_price: null, intervals: [], ...overrides }
}
export function model(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return { name: 'example-model', platform: 'openai', pricing: pricing(), official_pricing: { input_price: 10e-6, output_price: 40e-6, cache_write_price: 12.5e-6, cache_read_price: 1e-6 }, ...overrides }
}
export function group(overrides: Partial<ModelPlazaGroup> = {}): ModelPlazaGroup {
  return { id: 1, name: 'Group A', description: '', platform: 'openai', subscription_type: 'standard', rate_multiplier: .5, peak_rate_enabled: false, peak_start: '', peak_end: '', peak_rate_multiplier: 1, is_exclusive: false, image_rate_independent: false, image_rate_multiplier: 1, video_rate_independent: false, video_rate_multiplier: 1, long_context_pricing_enabled: true, models: [model()], ...overrides }
}
