/**
 * Detects whether an upstream account Base URL looks like a supported quota-monitor provider.
 * Unknown relay domains intentionally stay unsupported for automatic monitor creation.
 */

export type AutoQuotaMonitorProvider = 'sub2api' | 'newapi' | 'custom'
export type QuotaMonitorDetection = AutoQuotaMonitorProvider | ''

const providerPatterns: Array<[AutoQuotaMonitorProvider, RegExp]> = [
  ['newapi', /(^|[.\-_/])new[-_]?api([.\-_/]|$)/i],
  ['sub2api', /(^|[.\-_/])sub2[-_]?api([.\-_/]|$)/i],
  ['sub2api', /^sub2[.\-_/]/i],
  ['sub2api', /^sub[.\-_/]/i],
  ['sub2api', /(^|[.\-_/])givemetoken([.\-_/]|$)/i],
  ['sub2api', /(^|[.\-_/])jgy\.ai([.\-_/]|$)/i],
]

export function detectQuotaMonitorProvider(rawBaseURL: string): QuotaMonitorDetection {
  const raw = rawBaseURL.trim()
  if (!raw) return ''

  let haystack = raw.toLowerCase()
  try {
    const normalized = /^[a-z][a-z0-9+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`
    const url = new URL(normalized)
    haystack = `${url.hostname}${url.pathname}`.toLowerCase()
  } catch {
    // Keep the raw haystack so partially typed hosts can still be detected.
  }

  for (const [provider, pattern] of providerPatterns) {
    if (pattern.test(haystack)) return provider
  }

  const path = haystack.replace(/^[^/]+/, '')
  if (/\/api\/(user\/self|user\/dashboard|token\/self|user\/token|usage\/token)\/?$/i.test(path)) return 'newapi'
  if (/\/api\/v1\/(user|user\/profile|user\/self)\/?$/i.test(path)) return 'sub2api'
  return 'custom'
}

export function normalizeQuotaMonitorEndpointKey(rawEndpoint: string): string {
  const raw = rawEndpoint.trim()
  if (!raw) return ''

  try {
    const normalized = /^[a-z][a-z0-9+.-]*:\/\//i.test(raw) ? raw : `https://${raw}`
    const url = new URL(normalized)
    url.hash = ''
    url.search = ''
    url.protocol = url.protocol.toLowerCase()
    url.hostname = url.hostname.toLowerCase()
    let path = url.pathname.replace(/\/+$/, '')
    if (path === '/' || path === '/v1' || path === '/api' || path === '/api/v1') {
      path = ''
    }
    url.pathname = path
    return url.toString().replace(/\/+$/, '')
  } catch {
    return raw.toLowerCase().replace(/[?#].*$/, '').replace(/\/+$/, '')
  }
}
