<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accountQuotaMonitor.title', '额度监控')"
    width="full"
    @close="handleClose"
  >
    <div class="space-y-5">
      <div class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.total', '监控总数') }}</p>
          <p class="mt-1 text-2xl font-semibold text-gray-900 dark:text-white">{{ summaryData?.total ?? 0 }}</p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.enabledCount', '启用') }} {{ summaryData?.enabled ?? 0 }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.health', '状态') }}</p>
          <p class="mt-1 text-sm text-gray-700 dark:text-gray-200">
            <span class="text-green-600 dark:text-green-400">OK {{ summaryData?.ok ?? 0 }}</span>
            <span class="mx-2 text-gray-300">/</span>
            <span class="text-amber-600 dark:text-amber-400">LOW {{ summaryData?.low_balance ?? 0 }}</span>
            <span class="mx-2 text-gray-300">/</span>
            <span class="text-red-600 dark:text-red-400">ERR {{ summaryData?.error ?? 0 }}</span>
          </p>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.unknown') }} {{ summaryData?.unknown ?? 0 }}</p>
        </div>
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900 lg:col-span-2">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.totalBalance', '当前总余额') }}</p>
          <div class="mt-1 flex flex-wrap gap-2">
            <span
              v-for="item in normalizedCurrencyTotals"
              :key="item.currency"
              class="rounded-lg bg-primary-50 px-3 py-1 text-sm font-semibold text-primary-700 dark:bg-primary-900/20 dark:text-primary-300"
            >
              {{ item.currency }} {{ formatNumber(item.balance) }}
            </span>
            <span v-if="normalizedCurrencyTotals.length === 0" class="text-sm text-gray-400">-</span>
          </div>
          <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.totalBalanceHint', '按最近一次成功/失败记录中的余额汇总') }}</p>
        </div>
      </div>

      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div class="flex flex-1 flex-wrap items-center gap-2">
          <div class="relative w-full sm:w-64">
            <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input
              v-model="filters.search"
              type="text"
              class="input pl-9"
              :placeholder="t('admin.accountQuotaMonitor.searchPlaceholder', '搜索监控/账号/端点')"
              @input="handleSearch"
            />
          </div>
          <Select v-model="filters.provider" :options="providerFilterOptions" class="w-40" @change="reloadFirstPage" />
          <Select v-model="filters.enabled" :options="enabledFilterOptions" class="w-36" @change="reloadFirstPage" />
        </div>
        <div class="flex flex-wrap items-center justify-end gap-2">
          <button class="btn btn-secondary" :disabled="loading" @click="reload">
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            {{ t('common.refresh', '刷新') }}
          </button>
          <button class="btn btn-primary" @click="openCreateForm">
            <Icon name="plus" size="sm" />
            {{ t('admin.accountQuotaMonitor.create', '新增额度监控') }}
          </button>
        </div>
      </div>

      <div v-if="showForm" class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/10">
        <div class="mb-3 flex items-center justify-between">
          <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
            {{ editing ? t('admin.accountQuotaMonitor.edit', '编辑额度监控') : t('admin.accountQuotaMonitor.create', '新增额度监控') }}
          </h4>
          <button type="button" class="text-gray-400 hover:text-gray-600" @click="closeForm">
            <Icon name="x" size="sm" />
          </button>
        </div>
        <form class="grid gap-4 lg:grid-cols-2" @submit.prevent="submitForm">
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.account', '账号') }} <span class="text-red-500">*</span></label>
            <div class="relative account-quota-account-search">
              <input
                v-model="accountSearch"
                type="text"
                class="input"
                :placeholder="selectedAccountLabel || t('admin.accountQuotaMonitor.form.accountPlaceholder', '搜索并选择账号')"
                @input="searchAccounts"
                @focus="searchAccounts"
              />
              <div
                v-if="showAccountDropdown && accountResults.length > 0"
                class="absolute z-50 mt-1 max-h-56 w-full overflow-auto rounded-lg border bg-white shadow-lg dark:border-dark-600 dark:bg-dark-800"
              >
                <button
                  v-for="account in accountResults"
                  :key="account.id"
                  type="button"
                  class="w-full px-3 py-2 text-left text-sm hover:bg-gray-100 dark:hover:bg-dark-700"
                  @click="selectAccount(account)"
                >
                  <span class="font-medium text-gray-900 dark:text-gray-100">{{ account.name }}</span>
                  <span class="ml-2 text-xs text-gray-400">#{{ account.id }} / {{ account.platform }} / {{ account.type }}</span>
                </button>
              </div>
            </div>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ selectedAccountLabel || t('admin.accountQuotaMonitor.form.accountHint', '使用账号内保存的 base_url 与 api_key/access_token；不会返回密钥到前端') }}
            </p>
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.name', '监控名称') }} <span class="text-red-500">*</span></label>
            <input v-model="form.name" required class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.provider', '中转平台') }}</label>
            <Select v-model="form.provider" :options="providerOptions" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.endpoint', '余额查询端点 / Base URL') }}</label>
            <input v-model="form.endpoint" class="input" placeholder="https://example.com" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accountQuotaMonitor.form.endpointHint', '可留空，服务端会优先读取账号 credentials.base_url；填写完整路径时会先尝试该路径') }}
            </p>
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.interval', '查询间隔（秒）') }}</label>
            <input v-model.number="form.interval_seconds" type="number" min="60" max="86400" class="input" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.threshold', '低余额阈值') }}</label>
            <input v-model.number="form.low_balance_threshold" type="number" min="0" step="0.000001" class="input" placeholder="10" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.currency', '币种/单位') }}</label>
            <input v-model="form.currency" class="input" placeholder="USD" />
          </div>

          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.apiKeyOverride', '覆盖密钥（可选）') }}</label>
            <input v-model="form.api_key_override" type="password" class="input" autocomplete="new-password" />
            <label v-if="editing?.api_key_override_set" class="mt-2 flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <input v-model="form.clear_api_key_override" type="checkbox" class="rounded border-gray-300 text-primary-600" />
              {{ t('admin.accountQuotaMonitor.form.clearApiKeyOverride', '清除已保存的覆盖密钥') }}
            </label>
          </div>

          <div class="flex items-center gap-3">
            <Toggle :modelValue="form.enabled" @update:modelValue="form.enabled = $event" />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accountQuotaMonitor.form.enabled', '启用定时查询') }}</span>
          </div>

          <div class="flex justify-end gap-2 lg:col-span-2">
            <button type="button" class="btn btn-secondary" @click="closeForm">{{ t('common.cancel', '取消') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="submitting">
              {{ submitting ? t('common.submitting', '提交中...') : t('common.save', '保存') }}
            </button>
          </div>
        </form>
      </div>

      <DataTable :columns="columns" :data="monitors" :loading="loading">
        <template #cell-name="{ row }">
          <div class="min-w-48">
            <div class="flex items-center gap-2">
              <span class="font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
              <span v-if="row.api_key_override_decrypt_failed" class="rounded bg-red-100 px-1.5 py-0.5 text-[10px] text-red-700 dark:bg-red-900/30 dark:text-red-300">KEY ERR</span>
            </div>
            <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">{{ row.endpoint || t('admin.accountQuotaMonitor.autoEndpoint', '使用账号配置') }}</div>
          </div>
        </template>

        <template #cell-account="{ row }">
          <div class="text-sm">
            <div class="font-medium text-gray-900 dark:text-gray-100">{{ row.account_name || ('#' + row.account_id) }}</div>
            <div class="text-xs text-gray-400">#{{ row.account_id }} / {{ row.account_platform }}</div>
          </div>
        </template>

        <template #cell-provider="{ row }">
          <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(row.provider)">{{ providerLabel(row.provider) }}</span>
        </template>

        <template #cell-balance="{ row }">
          <div class="text-sm text-gray-900 dark:text-gray-100">
            {{ formatBalance(row.last_balance, row.currency) }}
            <div v-if="row.last_quota_used != null || row.last_quota_total != null" class="text-xs text-gray-400">
              {{ t('admin.accountQuotaMonitor.used', '已用') }} {{ formatNumber(row.last_quota_used) }} / {{ formatNumber(row.last_quota_total) }}
            </div>
          </div>
        </template>

        <template #cell-status="{ row }">
          <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="statusBadgeClass(row.last_status)">{{ statusLabel(row.last_status) }}</span>
          <div v-if="row.last_message" class="mt-1 max-w-64 truncate text-xs text-gray-400" :title="row.last_message">{{ row.last_message }}</div>
        </template>

        <template #cell-forecast="{ row }">
          <div class="text-sm text-gray-900 dark:text-gray-100">
            <div>{{ formatRemainingDays(row.estimated_days_remaining) }}</div>
            <div class="text-xs text-gray-400">24h: {{ formatNumber(row.consumption_24h) }}</div>
          </div>
        </template>

        <template #cell-enabled="{ row }">
          <Toggle :modelValue="row.enabled" @update:modelValue="toggleEnabled(row)" />
        </template>

        <template #cell-last_checked_at="{ row }">
          <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatDateTime(row.last_checked_at) }}</span>
        </template>

        <template #cell-actions="{ row }">
          <div class="flex items-center gap-1">
            <button class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" :disabled="runningId === row.id" @click="runNow(row)">
              <Icon name="play" size="sm" :class="runningId === row.id ? 'animate-pulse' : ''" />
            </button>
            <button class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" @click="loadHistory(row)">
              <Icon name="chart" size="sm" />
            </button>
            <button class="rounded-lg p-1.5 text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700" @click="openEditForm(row)">
              <Icon name="edit" size="sm" />
            </button>
            <button class="rounded-lg p-1.5 text-gray-500 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-900/20" @click="deleteMonitor(row)">
              <Icon name="trash" size="sm" />
            </button>
          </div>
        </template>

        <template #empty>
          <EmptyState
            :title="t('admin.accountQuotaMonitor.emptyTitle', '暂无额度监控')"
            :description="t('admin.accountQuotaMonitor.emptyDesc', '选择账号并配置 sub2api / NewAPI / 自定义余额接口后即可定时监控')"
            :action-text="t('admin.accountQuotaMonitor.create', '新增额度监控')"
            @action="openCreateForm"
          />
        </template>
      </DataTable>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="onPageChange"
        @update:pageSize="onPageSizeChange"
      />

      <div class="grid gap-4 lg:grid-cols-2">
        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-3 flex items-center justify-between">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accountQuotaMonitor.trend', '总量趋势') }}</h4>
            <span class="text-xs text-gray-400">7d / hour</span>
          </div>
          <div v-if="trendPoints.length === 0" class="py-8 text-center text-sm text-gray-400">{{ t('common.noData', '暂无数据') }}</div>
          <div v-else class="space-y-2">
            <div v-for="point in trendPoints.slice(-12)" :key="point.bucket + point.currency" class="flex items-center justify-between text-sm">
              <span class="text-gray-500 dark:text-gray-400">{{ formatDateTime(point.bucket) }}</span>
              <span class="font-medium text-gray-900 dark:text-gray-100">{{ point.currency }} {{ formatNumber(point.balance) }}</span>
            </div>
          </div>
        </div>

        <div class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <div class="mb-3 flex items-center justify-between">
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">
              {{ selectedHistoryMonitor ? selectedHistoryMonitor.name : t('admin.accountQuotaMonitor.history', '查询历史') }}
            </h4>
            <button v-if="selectedHistoryMonitor" class="text-xs text-gray-400 hover:text-gray-600" @click="selectedHistoryMonitor = null; historyItems = []">
              {{ t('common.close', '关闭') }}
            </button>
          </div>
          <div v-if="historyItems.length === 0" class="py-8 text-center text-sm text-gray-400">
            {{ t('admin.accountQuotaMonitor.historyHint', '点击表格中的趋势按钮查看单个监控历史') }}
          </div>
          <div v-else class="max-h-72 space-y-2 overflow-auto">
            <div v-for="item in historyItems" :key="item.id" class="rounded-lg bg-gray-50 px-3 py-2 text-sm dark:bg-dark-800">
              <div class="flex items-center justify-between">
                <span :class="statusTextClass(item.status)">{{ statusLabel(item.status) }}</span>
                <span class="text-xs text-gray-400">{{ formatDateTime(item.checked_at) }}</span>
              </div>
              <div class="mt-1 text-gray-900 dark:text-gray-100">{{ formatBalance(item.balance, item.currency) }}</div>
              <div v-if="item.message" class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ item.message }}</div>
            </div>
          </div>
        </div>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account } from '@/types'
import type { Column } from '@/components/common/types'
import type {
  AccountQuotaMonitor,
  AccountQuotaProvider,
  AccountQuotaStatus,
  HistoryItem,
  SummaryResponse,
  TrendPoint,
} from '@/api/admin/accountQuotaMonitor'
import { extractApiErrorMessage } from '@/utils/apiError'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'

const props = defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()

const { t } = useI18n()
const appStore = useAppStore()

const monitors = ref<AccountQuotaMonitor[]>([])
const loading = ref(false)
const submitting = ref(false)
const runningId = ref<number | null>(null)
const summaryData = ref<SummaryResponse | null>(null)
const trendPoints = ref<TrendPoint[]>([])
const historyItems = ref<HistoryItem[]>([])
const selectedHistoryMonitor = ref<AccountQuotaMonitor | null>(null)

const filters = reactive({ search: '', provider: '' as '' | AccountQuotaProvider, enabled: '' as '' | 'true' | 'false' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null

const showForm = ref(false)
const editing = ref<AccountQuotaMonitor | null>(null)
const accountSearch = ref('')
const accountResults = ref<Account[]>([])
const showAccountDropdown = ref(false)
const selectedAccountLabel = ref('')

const form = reactive({
  name: '',
  account_id: 0,
  provider: 'sub2api' as AccountQuotaProvider,
  endpoint: '',
  api_key_override: '',
  enabled: true,
  interval_seconds: 3600,
  low_balance_threshold: null as number | null | '',
  currency: 'USD',
  clear_api_key_override: false,
})

const columns = computed<Column[]>(() => [
  { key: 'name', label: t('admin.accountQuotaMonitor.columns.name', '监控'), sortable: false },
  { key: 'account', label: t('admin.accountQuotaMonitor.columns.account', '账号'), sortable: false },
  { key: 'provider', label: t('admin.accountQuotaMonitor.columns.provider', '平台'), sortable: false },
  { key: 'balance', label: t('admin.accountQuotaMonitor.columns.balance', '余额'), sortable: false },
  { key: 'status', label: t('admin.accountQuotaMonitor.columns.status', '状态'), sortable: false },
  { key: 'forecast', label: t('admin.accountQuotaMonitor.columns.forecast', '预测'), sortable: false },
  { key: 'enabled', label: t('admin.accountQuotaMonitor.columns.enabled', '启用'), sortable: false },
  { key: 'last_checked_at', label: t('admin.accountQuotaMonitor.columns.lastChecked', '最近查询'), sortable: false },
  { key: 'actions', label: t('admin.accountQuotaMonitor.columns.actions', '操作'), sortable: false },
])

const providerOptions = computed(() => [
  { value: 'sub2api', label: 'sub2api' },
  { value: 'newapi', label: 'NewAPI' },
  { value: 'custom', label: t('admin.accountQuotaMonitor.provider.custom', '自定义') },
])
const providerFilterOptions = computed(() => [{ value: '', label: t('admin.accountQuotaMonitor.allProviders', '全部平台') }, ...providerOptions.value])
const enabledFilterOptions = computed(() => [
  { value: '', label: t('admin.accountQuotaMonitor.allEnabled', '全部状态') },
  { value: 'true', label: t('common.enabled', '启用') },
  { value: 'false', label: t('common.disabled', '禁用') },
])

const normalizedCurrencyTotals = computed(() => {
  const items = summaryData.value?.total_balance_by_currency || []
  return items.map((item) => ({
    currency: item.currency || item.Currency || 'USD',
    balance: Number(item.balance ?? item.Balance ?? 0),
    count: Number(item.count ?? item.Count ?? 0),
  }))
})

watch(() => props.show, (show) => {
  if (show) reload()
})

onUnmounted(() => {
  if (abortController) abortController.abort()
  if (searchTimeout) clearTimeout(searchTimeout)
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
})

function handleClose() {
  emit('close')
}

async function reload() {
  if (!props.show) return
  if (abortController) abortController.abort()
  const ctrl = new AbortController()
  abortController = ctrl
  loading.value = true
  try {
    const params: Record<string, unknown> = { page: pagination.page, page_size: pagination.page_size }
    if (filters.search.trim()) params.search = filters.search.trim()
    if (filters.provider) params.provider = filters.provider
    if (filters.enabled === 'true') params.enabled = true
    if (filters.enabled === 'false') params.enabled = false
    const [listRes, summaryRes, trendRes] = await Promise.all([
      adminAPI.accountQuotaMonitor.list(params, { signal: ctrl.signal }),
      adminAPI.accountQuotaMonitor.summary(),
      adminAPI.accountQuotaMonitor.trend(7, 'hour'),
    ])
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = listRes.items || []
    pagination.total = listRes.total
    summaryData.value = summaryRes
    trendPoints.value = trendRes.items || []
  } catch (err: unknown) {
    const e = err as { name?: string; code?: string }
    if (e?.name === 'AbortError' || e?.code === 'ERR_CANCELED') return
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.loadError', '加载额度监控失败')))
  } finally {
    if (abortController === ctrl) {
      loading.value = false
      abortController = null
    }
  }
}

function reloadFirstPage() {
  pagination.page = 1
  reload()
}

function handleSearch() {
  if (searchTimeout) clearTimeout(searchTimeout)
  searchTimeout = setTimeout(reloadFirstPage, 300)
}

function onPageChange(page: number) {
  pagination.page = page
  reload()
}

function onPageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  reload()
}

function resetForm() {
  editing.value = null
  form.name = ''
  form.account_id = 0
  form.provider = 'sub2api'
  form.endpoint = ''
  form.api_key_override = ''
  form.enabled = true
  form.interval_seconds = 3600
  form.low_balance_threshold = null
  form.currency = 'USD'
  form.clear_api_key_override = false
  accountSearch.value = ''
  accountResults.value = []
  selectedAccountLabel.value = ''
  showAccountDropdown.value = false
}

function openCreateForm() {
  resetForm()
  showForm.value = true
}

function openEditForm(row: AccountQuotaMonitor) {
  resetForm()
  editing.value = row
  form.name = row.name
  form.account_id = row.account_id
  form.provider = row.provider
  form.endpoint = row.endpoint || ''
  form.enabled = row.enabled
  form.interval_seconds = row.interval_seconds
  form.low_balance_threshold = row.low_balance_threshold
  form.currency = row.currency || 'USD'
  selectedAccountLabel.value = `${row.account_name || ('#' + row.account_id)} #${row.account_id}`
  showForm.value = true
}

function closeForm() {
  showForm.value = false
}

async function searchAccounts() {
  showAccountDropdown.value = true
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
  accountSearchTimeout = setTimeout(async () => {
    try {
      const res = await adminAPI.accounts.list(1, 20, { search: accountSearch.value.trim(), lite: 'true' })
      accountResults.value = res.items || []
    } catch (err) {
      accountResults.value = []
    }
  }, 250)
}

function selectAccount(account: Account) {
  form.account_id = account.id
  selectedAccountLabel.value = `${account.name} #${account.id}`
  accountSearch.value = ''
  showAccountDropdown.value = false
  const credentials = account.credentials || {}
  const baseURL = typeof credentials.base_url === 'string' ? credentials.base_url : ''
  if (!form.endpoint && baseURL) form.endpoint = baseURL
  if (!form.name) form.name = `${account.name} 额度监控`
}

function normalizedThreshold(): number | null {
  if (form.low_balance_threshold === null || form.low_balance_threshold === '' || form.low_balance_threshold === undefined) return null
  const value = Number(form.low_balance_threshold)
  return Number.isFinite(value) ? value : null
}

async function submitForm() {
  if (!form.account_id) {
    appStore.showError(t('admin.accountQuotaMonitor.form.accountRequired', '请先选择账号'))
    return
  }
  submitting.value = true
  try {
    const payload = {
      name: form.name.trim(),
      account_id: form.account_id,
      provider: form.provider,
      endpoint: form.endpoint.trim(),
      enabled: form.enabled,
      interval_seconds: Number(form.interval_seconds) || 3600,
      low_balance_threshold: normalizedThreshold(),
      currency: form.currency.trim() || 'USD',
      api_key_override: form.api_key_override.trim() || undefined,
      clear_api_key_override: form.clear_api_key_override,
      clear_low_balance_threshold: Boolean(editing.value && normalizedThreshold() === null && editing.value.low_balance_threshold !== null),
    }
    if (editing.value) {
      await adminAPI.accountQuotaMonitor.update(editing.value.id, payload)
    } else {
      await adminAPI.accountQuotaMonitor.create(payload)
    }
    appStore.showSuccess(t('common.saved', '已保存'))
    closeForm()
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.saveError', '保存额度监控失败')))
  } finally {
    submitting.value = false
  }
}

async function toggleEnabled(row: AccountQuotaMonitor) {
  const next = !row.enabled
  try {
    await adminAPI.accountQuotaMonitor.update(row.id, { enabled: next })
    row.enabled = next
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error', '操作失败')))
  }
}

async function runNow(row: AccountQuotaMonitor) {
  runningId.value = row.id
  try {
    const result = await adminAPI.accountQuotaMonitor.runNow(row.id)
    appStore.showSuccess(result.status === 'error'
      ? t('admin.accountQuotaMonitor.runCompletedWithError', '查询完成但余额接口返回错误')
      : t('admin.accountQuotaMonitor.runSuccess', '查询完成'))
    await reload()
    if (selectedHistoryMonitor.value?.id === row.id) await loadHistory(row)
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.runError', '手动查询失败')))
  } finally {
    runningId.value = null
  }
}

async function loadHistory(row: AccountQuotaMonitor) {
  selectedHistoryMonitor.value = row
  try {
    const res = await adminAPI.accountQuotaMonitor.listHistory(row.id, 100)
    historyItems.value = res.items || []
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.historyError', '加载历史失败')))
  }
}

async function deleteMonitor(row: AccountQuotaMonitor) {
  if (!window.confirm(t('admin.accountQuotaMonitor.deleteConfirm', { name: row.name }, `确认删除额度监控「${row.name}」？`))) return
  try {
    await adminAPI.accountQuotaMonitor.del(row.id)
    appStore.showSuccess(t('common.deleted', '已删除'))
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error', '操作失败')))
  }
}

function providerLabel(provider: AccountQuotaProvider) {
  if (provider === 'newapi') return 'NewAPI'
  if (provider === 'sub2api') return 'sub2api'
  return t('admin.accountQuotaMonitor.provider.custom', '自定义')
}

function providerBadgeClass(provider: AccountQuotaProvider) {
  if (provider === 'sub2api') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  if (provider === 'newapi') return 'bg-purple-100 text-purple-700 dark:bg-purple-900/30 dark:text-purple-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function statusLabel(status: AccountQuotaStatus) {
  const labels: Record<AccountQuotaStatus, string> = {
    unknown: t('admin.accountQuotaMonitor.status.unknown', '未知'),
    ok: 'OK',
    low_balance: t('admin.accountQuotaMonitor.status.low', '余额低'),
    error: t('admin.accountQuotaMonitor.status.error', '错误'),
  }
  return labels[status] || status
}

function statusBadgeClass(status: AccountQuotaStatus) {
  if (status === 'ok') return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  if (status === 'low_balance') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (status === 'error') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}

function statusTextClass(status: AccountQuotaStatus) {
  if (status === 'ok') return 'text-green-600 dark:text-green-400'
  if (status === 'low_balance') return 'text-amber-600 dark:text-amber-400'
  if (status === 'error') return 'text-red-600 dark:text-red-400'
  return 'text-gray-500 dark:text-gray-400'
}

function formatNumber(value: number | null | undefined) {
  if (value === null || value === undefined || Number.isNaN(Number(value))) return '-'
  return new Intl.NumberFormat(undefined, { maximumFractionDigits: 6 }).format(Number(value))
}

function formatBalance(value: number | null | undefined, currency: string) {
  if (value === null || value === undefined) return '-'
  return `${currency || ''} ${formatNumber(value)}`.trim()
}

function formatRemainingDays(value: number | null | undefined) {
  if (value === null || value === undefined || !Number.isFinite(value)) return t('admin.accountQuotaMonitor.notEnoughData', '数据不足')
  if (value < 1) return t('admin.accountQuotaMonitor.lessThanOneDay', '< 1 天')
  return t('admin.accountQuotaMonitor.daysRemaining', { days: value.toFixed(1) }, `${value.toFixed(1)} 天`)
}

function formatDateTime(value: string | null | undefined) {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}
</script>
