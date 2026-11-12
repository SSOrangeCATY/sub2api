import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import GroupDistributionChart from '../GroupDistributionChart.vue'

const messages: Record<string, string> = {
  'admin.dashboard.groupDistribution': 'Group Distribution',
  'admin.dashboard.group': 'Group',
  'admin.dashboard.noGroup': 'No Group',
  'admin.dashboard.requests': 'Requests',
  'admin.dashboard.tokens': 'Tokens',
  'admin.dashboard.actual': 'Actual',
  'admin.dashboard.accountCost': 'Account Cost',
  'admin.dashboard.standard': 'Standard',
  'admin.dashboard.metricTokens': 'By Tokens',
  'admin.dashboard.metricActualCost': 'By Actual Cost',
  'admin.dashboard.noDataAvailable': 'No data available',
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

describe('GroupDistributionChart', () => {
  const groupStats = [
    {
      group_id: 1,
      group_name: 'group-a',
      requests: 9,
      total_tokens: 1200,
      cost: 1.8,
      actual_cost: 0.1,
    },
    {
      group_id: 2,
      group_name: 'group-b',
      requests: 4,
      total_tokens: 600,
      cost: 0.7,
      actual_cost: 0.9,
    },
  ]

  it('uses total_tokens and token ordering by default', () => {
    const wrapper = mount(GroupDistributionChart, {
      props: {
        groupStats,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const bars = wrapper.findAll('.ef-dist li')
    expect(bars.map((bar) => bar.find('.ef-dist-name').text())).toEqual(['group-a', 'group-b'])
    expect(bars.map((bar) => bar.find('.ef-dist-val').text())).toEqual(['1.20K', '600'])
    expect(bars.map((bar) => bar.find('.ef-dist-pct').text())).toEqual(['66.7%', '33.3%'])
    expect(bars[0].find('.ef-dist-track').attributes('style')).toContain('--p: 1')

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('group-a')
    expect(rows[1].text()).toContain('group-b')

  })

  it('uses actual_cost and reorders rows in actual cost mode', () => {
    const wrapper = mount(GroupDistributionChart, {
      props: {
        groupStats,
        metric: 'actual_cost',
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    const bars = wrapper.findAll('.ef-dist li')
    expect(bars.map((bar) => bar.find('.ef-dist-name').text())).toEqual(['group-b', 'group-a'])
    expect(bars.map((bar) => bar.find('.ef-dist-val').text())).toEqual(['0.900', '0.100'])
    expect(bars.map((bar) => bar.find('.ef-dist-pct').text())).toEqual(['90.0%', '10.0%'])
    expect(bars[0].find('.ef-dist-track').attributes('style')).toContain('--p: 1')

    const rows = wrapper.findAll('tbody tr')
    expect(rows[0].text()).toContain('group-b')
    expect(rows[1].text()).toContain('group-a')

  })

  it('can hide account cost for user usage stats without account_cost', () => {
    const wrapper = mount(GroupDistributionChart, {
      props: {
        groupStats,
        showAccountCost: false,
      },
      global: {
        stubs: {
          LoadingSpinner: true,
        },
      },
    })

    expect(wrapper.text()).not.toContain('Account Cost')
    expect(wrapper.findAll('thead th')).toHaveLength(5)
    expect(wrapper.findAll('tbody tr')[0].findAll('td')).toHaveLength(5)
  })
})
