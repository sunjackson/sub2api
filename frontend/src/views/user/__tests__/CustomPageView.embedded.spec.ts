import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import CustomPageView from '../CustomPageView.vue'

vi.mock('vue-router', () => ({
  useRoute: () => ({
    params: { id: 'billing' }
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'zh-CN' }
    })
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    publicSettingsLoaded: true,
    cachedPublicSettings: {
      custom_menu_items: [
        {
          id: 'billing',
          url: 'https://billing.example.com/checkout?plan=pro'
        }
      ]
    },
    fetchPublicSettings: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    isAdmin: false,
    user: { id: 42 },
    token: 'secret-jwt-token'
  })
}))

vi.mock('@/stores/adminSettings', () => ({
  useAdminSettingsStore: () => ({
    customMenuItems: []
  })
}))

describe('CustomPageView embedded iframe', () => {
  it('renders embedded URL without auth token and with iframe safety attributes', () => {
    const wrapper = mount(CustomPageView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })

    const iframe = wrapper.get('iframe')
    const src = iframe.attributes('src') ?? ''
    const url = new URL(src)

    expect(url.origin).toBe('https://billing.example.com')
    expect(url.searchParams.get('plan')).toBe('pro')
    expect(url.searchParams.get('user_id')).toBe('42')
    expect(url.searchParams.get('lang')).toBe('zh-CN')
    expect(url.searchParams.has('token')).toBe(false)
    expect(src).not.toContain('secret-jwt-token')
    expect(iframe.attributes('sandbox')).toContain('allow-scripts')
    expect(iframe.attributes('sandbox')).toContain('allow-forms')
    expect(iframe.attributes('referrerpolicy')).toBe('strict-origin-when-cross-origin')
  })
})
