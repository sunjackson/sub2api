import { describe, expect, it } from 'vitest'
import { detectQuotaMonitorProvider, normalizeQuotaMonitorEndpointKey } from '../quotaMonitorDetection'

describe('detectQuotaMonitorProvider', () => {
  it('detects supported relay platforms from host or path', () => {
    expect(detectQuotaMonitorProvider('https://relay.sub2api.example.com/v1')).toBe('sub2api')
    expect(detectQuotaMonitorProvider('newapi.example.com')).toBe('newapi')
    expect(detectQuotaMonitorProvider('https://example.com/sub2api')).toBe('sub2api')
  })

  it('marks unknown urls as other and empty input as blank', () => {
    expect(detectQuotaMonitorProvider('https://api.openai.com')).toBe('other')
    expect(detectQuotaMonitorProvider('')).toBe('')
  })

  it('normalizes endpoint keys for duplicate detection', () => {
    expect(normalizeQuotaMonitorEndpointKey('HTTPS://Sub2API.Example.COM/api/v1/')).toBe('https://sub2api.example.com')
    expect(normalizeQuotaMonitorEndpointKey('sub2api.example.com/v1/?x=1#frag')).toBe('https://sub2api.example.com')
    expect(normalizeQuotaMonitorEndpointKey('https://sub2api.example.com/custom/')).toBe('https://sub2api.example.com/custom')
  })
})
