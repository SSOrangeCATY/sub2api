import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import type { MonitorTimelinePoint } from '@/api/channelMonitor'
import MonitorTimeline from '../monitor/MonitorTimeline.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

const point = (status: MonitorTimelinePoint['status'], latency: number | null = 150): MonitorTimelinePoint => ({
  status, latency_ms: latency, ping_latency_ms: null, checked_at: '2026-11-01T00:00:00Z',
})

describe('channel history presentation', () => {
  it('keeps sixty time slots while only displaying recorded states', () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 30, buckets: [point('failed'), point('operational')] } })
    const bars = wrapper.findAll('.monitor-timeline__bar')
    const slots = wrapper.findAll('.monitor-timeline__slot')
    expect(slots).toHaveLength(60)
    expect(bars).toHaveLength(2)
    expect(slots.slice(0, 58).every(slot => slot.attributes('aria-hidden') === 'true' && slot.find('.monitor-timeline__bar').exists() === false)).toBe(true)
    expect(slots[58].get('.monitor-timeline__bar').attributes('data-status')).toBe('operational')
    expect(slots[59].get('.monitor-timeline__bar').attributes('data-status')).toBe('failed')
    expect(wrapper.findAll('.monitor-timeline__legend b').map(item => item.text())).toEqual(['1', '0', '1', '0'])
    expect(wrapper.text()).not.toContain('monitorCommon.timelineNoData')
  })

  it('preserves all status kinds and distinct heights without relying on color utilities', () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 0, length: 5, buckets: [point('error', null), point('failed'), point('degraded'), point('operational')] } })
    const bars = wrapper.findAll('.monitor-timeline__bar')
    expect(bars.map(bar => bar.attributes('data-status'))).toEqual(['operational', 'degraded', 'failed', 'error'])
    expect(bars.map(bar => bar.element.style.height)).toEqual(['100%', '65%', '35%', '35%'])
    expect(bars.every(bar => bar.attributes('aria-label') === bar.attributes('title'))).toBe(true)
    expect(bars[3].attributes('title')).not.toMatch(/ms$/)
    expect(wrapper.html()).not.toMatch(/bg-(emerald|red|amber)-/)
  })

  it('caps visible data and legend totals at the configured length', () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 0, length: 3, buckets: [point('operational'), point('degraded'), point('failed'), point('error')] } })
    expect(wrapper.findAll('.monitor-timeline__bar')).toHaveLength(3)
    expect(wrapper.findAll('.monitor-timeline__legend b').map(item => item.text())).toEqual(['1', '1', '1', '0'])
  })

  it('hides unknown payload states and updates with new props', async () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 0, length: 1, buckets: [point('new_status' as MonitorTimelinePoint['status'])] } })
    expect(wrapper.find('.monitor-timeline__bar').exists()).toBe(false)
    expect(wrapper.get('.monitor-timeline__slot').attributes('aria-hidden')).toBe('true')
    expect(wrapper.text()).not.toContain('monitorCommon.timelineNoData')
    await wrapper.setProps({ buckets: [point('degraded')] })
    expect(wrapper.get('.monitor-timeline__bar').attributes('data-status')).toBe('degraded')
  })

  it('does not render placeholder bars or a no-data legend for an empty history', () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 0 } })
    expect(wrapper.findAll('.monitor-timeline__slot')).toHaveLength(60)
    expect(wrapper.findAll('.monitor-timeline__bar')).toHaveLength(0)
    expect(wrapper.findAll('[data-status="empty"]')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('monitorCommon.timelineNoData')
  })

  it('does not present maintenance as a failed history', () => {
    const wrapper = mount(MonitorTimeline, { props: { countdownSeconds: 0, maintenance: true } })
    expect(wrapper.find('.monitor-timeline__maintenance').exists()).toBe(true)
    expect(wrapper.find('.monitor-timeline__bars').exists()).toBe(false)
    expect(wrapper.find('.monitor-timeline__legend').exists()).toBe(false)
  })
})
