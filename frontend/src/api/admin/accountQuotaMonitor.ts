/**
 * Admin Account Quota Monitor API endpoints
 * Monitors upstream relay account balances/quotas without exposing account secrets.
 */

import { apiClient } from '../client'

export type AccountQuotaProvider = 'sub2api' | 'newapi' | 'custom'
export type AccountQuotaStatus = 'unknown' | 'ok' | 'low_balance' | 'error'

export interface AccountQuotaMonitor {
  id: number
  name: string
  account_id: number
  account_name: string
  account_platform: string
  account_type: string
  provider: AccountQuotaProvider
  endpoint: string
  api_key_override_set: boolean
  api_key_override_masked: string
  api_key_override_decrypt_failed: boolean
  enabled: boolean
  interval_seconds: number
  low_balance_threshold: number | null
  currency: string
  last_balance: number | null
  last_quota_total: number | null
  last_quota_used: number | null
  last_checked_at: string | null
  last_status: AccountQuotaStatus
  last_message: string
  created_by: number
  created_at: string
  updated_at: string
  consumption_24h: number | null
  avg_daily_consumption: number | null
  estimated_days_remaining: number | null
  estimated_depleted_at: string | null
}

export interface ListParams {
  page?: number
  page_size?: number
  provider?: AccountQuotaProvider
  enabled?: boolean
  account_id?: number
  search?: string
}

export interface ListResponse {
  items: AccountQuotaMonitor[]
  total: number
  page: number
  page_size: number
  pages: number
}

export interface CreateParams {
  name: string
  account_id: number
  provider: AccountQuotaProvider
  endpoint?: string
  api_key_override?: string
  enabled?: boolean
  interval_seconds?: number
  low_balance_threshold?: number | null
  currency?: string
}

export type UpdateParams = Partial<CreateParams> & {
  clear_api_key_override?: boolean
  clear_low_balance_threshold?: boolean
}

export interface BatchFilters {
  platform?: string
  type?: string
  status?: string
  group?: string
  search?: string
  privacy_mode?: string
}

export interface BatchCreateParams {
  account_ids?: number[]
  filters?: BatchFilters
  provider: AccountQuotaProvider
  endpoint?: string
  api_key_override?: string
  enabled?: boolean
  interval_seconds?: number
  low_balance_threshold?: number | null
  currency?: string
  update_existing?: boolean
  max_accounts?: number
}

export interface BatchFailure {
  account_id: number
  account_name: string
  reason: string
}

export interface BatchCreateResponse {
  selected: number
  created: number
  updated: number
  skipped_existing: number
  skipped_duplicate: number
  duplicate_endpoints: string[]
  failed: number
  items: AccountQuotaMonitor[]
  failures: BatchFailure[]
}

export interface CheckResult {
  monitor_id: number
  account_id: number
  balance: number | null
  quota_total: number | null
  quota_used: number | null
  currency: string
  status: AccountQuotaStatus
  message: string
  checked_at: string
}

export interface HistoryItem extends CheckResult {
  id: number
}

export interface HistoryResponse {
  items: HistoryItem[]
}

export interface CurrencyTotal {
  Currency?: string
  Balance?: number
  Count?: number
  currency?: string
  balance?: number
  count?: number
}

export interface SummaryResponse {
  total: number
  enabled: number
  ok: number
  low_balance: number
  error: number
  unknown: number
  total_balance_by_currency: CurrencyTotal[]
}

export interface TrendPoint {
  bucket: string
  currency: string
  balance: number
  count: number
}

export interface TrendResponse {
  items: TrendPoint[]
}

export interface CandidateAccount {
  id: number
  name: string
  platform: string
  type: string
  status: string
}

export interface CandidateGroup {
  endpoint: string
  endpoint_key: string
  provider: AccountQuotaProvider
  provider_detected: boolean
  account_count: number
  monitor_count: number
  covered: boolean
  missing_account_count: number
  duplicate_monitor_count: number
  representative_account_id: number
  sample_accounts: CandidateAccount[]
  existing_monitor_ids: number[]
  status_counts: Record<string, number>
}

export interface CandidateOverviewResponse {
  total_accounts: number
  accounts_with_endpoint: number
  accounts_without_endpoint: number
  endpoint_groups: number
  covered_groups: number
  missing_groups: number
  detected_provider_groups: number
  custom_provider_groups: number
  duplicate_monitor_groups: number
  groups: CandidateGroup[]
}

export async function list(
  params: ListParams = {},
  options?: { signal?: AbortSignal }
): Promise<ListResponse> {
  const { data } = await apiClient.get<ListResponse>('/admin/quota-monitors', {
    params,
    signal: options?.signal,
  })
  return data
}

export async function get(id: number): Promise<AccountQuotaMonitor> {
  const { data } = await apiClient.get<AccountQuotaMonitor>(`/admin/quota-monitors/${id}`)
  return data
}

export async function create(params: CreateParams): Promise<AccountQuotaMonitor> {
  const { data } = await apiClient.post<AccountQuotaMonitor>('/admin/quota-monitors', params)
  return data
}

export async function batchCreate(params: BatchCreateParams): Promise<BatchCreateResponse> {
  const { data } = await apiClient.post<BatchCreateResponse>('/admin/quota-monitors/batch', params)
  return data
}

export async function update(id: number, params: UpdateParams): Promise<AccountQuotaMonitor> {
  const { data } = await apiClient.put<AccountQuotaMonitor>(`/admin/quota-monitors/${id}`, params)
  return data
}

export async function del(id: number): Promise<void> {
  await apiClient.delete(`/admin/quota-monitors/${id}`)
}

export async function runNow(id: number): Promise<CheckResult> {
  const { data } = await apiClient.post<CheckResult>(`/admin/quota-monitors/${id}/run`)
  return data
}

export async function listHistory(id: number, limit = 100): Promise<HistoryResponse> {
  const { data } = await apiClient.get<HistoryResponse>(`/admin/quota-monitors/${id}/history`, {
    params: { limit },
  })
  return data
}

export async function summary(): Promise<SummaryResponse> {
  const { data } = await apiClient.get<SummaryResponse>('/admin/quota-monitors/summary')
  return data
}

export async function trend(days = 7, bucket: 'hour' | 'day' = 'hour'): Promise<TrendResponse> {
  const { data } = await apiClient.get<TrendResponse>('/admin/quota-monitors/trend', {
    params: { days, bucket },
  })
  return data
}

export async function candidates(): Promise<CandidateOverviewResponse> {
  const { data } = await apiClient.get<CandidateOverviewResponse>('/admin/quota-monitors/candidates')
  return data
}

export const accountQuotaMonitorAPI = {
  list,
  get,
  create,
  batchCreate,
  update,
  del,
  runNow,
  listHistory,
  summary,
  trend,
  candidates,
}

export default accountQuotaMonitorAPI
