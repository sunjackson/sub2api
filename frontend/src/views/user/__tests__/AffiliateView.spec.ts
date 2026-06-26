import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '@/views/user/AffiliateView.vue'

const {
  getAffiliateDetailMock,
  transferAffiliateQuotaMock,
  showErrorMock,
  showSuccessMock,
  refreshUserMock,
} = vi.hoisted(() => ({
  getAffiliateDetailMock: vi.fn(),
  transferAffiliateQuotaMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  refreshUserMock: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail: (...args: any[]) => getAffiliateDetailMock(...args),
    transferAffiliateQuota: (...args: any[]) => transferAffiliateQuotaMock(...args),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: (...args: any[]) => showErrorMock(...args),
    showSuccess: (...args: any[]) => showSuccessMock(...args),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: (...args: any[]) => refreshUserMock(...args),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/utils/format', () => ({
  formatCurrency: (value: number | null | undefined) => `$${(value ?? 0).toFixed(2)}`,
  formatDateTime: () => '2026-06-09 10:00:00',
}))

describe('AffiliateView', () => {
  beforeEach(() => {
    getAffiliateDetailMock.mockReset()
    transferAffiliateQuotaMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    refreshUserMock.mockReset()

    getAffiliateDetailMock.mockResolvedValue({
      user_id: 1,
      aff_code: 'AFF123',
      aff_count: 1,
      aff_quota: 2,
      aff_frozen_quota: 0,
      aff_history_quota: 3,
      registration_reward_total: 1,
      total_reward: 4,
      effective_rebate_rate_percent: 20,
      invitees: [
        {
          user_id: 2,
          email: 'i***@example.com',
          username: 'invitee',
          created_at: '2026-06-09T02:00:00Z',
          registration_reward_total: 1,
          quota_rebate_total: 3,
          total_rebate: 4,
        },
      ],
    })
  })

  it('separates signup rewards from transferable rebate quota', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    const text = wrapper.text()
    expect(text).toContain('affiliate.stats.registrationReward')
    expect(text).toContain('affiliate.stats.registrationRewardHint')
    expect(text).toContain('affiliate.stats.totalReward')
    expect(text).toContain('affiliate.invitees.columns.registrationReward')
    expect(text).toContain('affiliate.invitees.columns.quotaRebate')
    expect(text).toContain('affiliate.invitees.columns.totalReward')
    expect(text).toContain('affiliate.tips.line3')
    expect(text).toContain('$1.00')
    expect(text).toContain('$3.00')
    expect(text).toContain('$4.00')
  })
})
