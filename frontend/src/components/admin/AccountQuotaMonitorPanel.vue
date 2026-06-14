<template>
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

      <div class="rounded-2xl border border-primary-100 bg-gradient-to-br from-primary-50/80 to-white p-4 dark:border-primary-900/40 dark:from-primary-950/20 dark:to-dark-900">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accountQuotaMonitor.overview.title', '账号 Base URL 一览') }}
            </h3>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accountQuotaMonitor.overview.description', '按 Base URL 自动去重，同一家中转只保留一个余额监控；保存前会真实请求余额接口，失败不会纳入监控。') }}
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="reload">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
              {{ t('common.refresh', '刷新') }}
            </button>
            <button
              class="btn btn-primary"
              :disabled="creatingAllCandidates || missingCandidateGroups.length === 0"
              @click="createAllCandidateMonitors"
            >
              <Icon name="sparkles" size="sm" :class="creatingAllCandidates ? 'animate-pulse' : ''" />
              {{ creatingAllCandidates ? t('common.submitting', '提交中...') : t('admin.accountQuotaMonitor.overview.createMissing', '探测并补齐待监控') }}
            </button>
          </div>
        </div>

        <div class="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
          <div class="rounded-xl bg-white/80 p-3 ring-1 ring-gray-200 dark:bg-dark-900/70 dark:ring-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.overview.endpointGroups', 'Base URL 分组') }}</p>
            <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ candidateOverview?.endpoint_groups ?? 0 }}</p>
          </div>
          <div class="rounded-xl bg-white/80 p-3 ring-1 ring-gray-200 dark:bg-dark-900/70 dark:ring-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.overview.coveredGroups', '已覆盖 / 待补齐') }}</p>
            <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ candidateOverview?.covered_groups ?? 0 }} / {{ candidateOverview?.missing_groups ?? 0 }}</p>
          </div>
          <div class="rounded-xl bg-white/80 p-3 ring-1 ring-gray-200 dark:bg-dark-900/70 dark:ring-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.overview.accountsWithEndpoint', '可识别账号') }}</p>
            <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ candidateOverview?.accounts_with_endpoint ?? 0 }}</p>
          </div>
          <div class="rounded-xl bg-white/80 p-3 ring-1 ring-gray-200 dark:bg-dark-900/70 dark:ring-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.overview.customGroups', '其他/自动探测') }}</p>
            <p class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">{{ candidateOverview?.custom_provider_groups ?? 0 }}</p>
          </div>
          <div class="rounded-xl bg-white/80 p-3 ring-1 ring-gray-200 dark:bg-dark-900/70 dark:ring-dark-700">
            <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.overview.duplicates', '重复监控组') }}</p>
            <p class="mt-1 text-xl font-semibold" :class="(candidateOverview?.duplicate_monitor_groups ?? 0) > 0 ? 'text-amber-600 dark:text-amber-400' : 'text-gray-900 dark:text-white'">
              {{ candidateOverview?.duplicate_monitor_groups ?? 0 }}
            </p>
          </div>
        </div>

        <div class="mt-4 overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900">
          <div v-if="visibleCandidateGroups.length === 0" class="px-4 py-6 text-center text-sm text-gray-400">
            {{ t('admin.accountQuotaMonitor.overview.empty', '暂无可一览的账号 Base URL') }}
          </div>
          <div v-else class="divide-y divide-gray-100 dark:divide-dark-800">
            <div
              v-for="group in visibleCandidateGroups"
              :key="group.endpoint_key"
              class="grid gap-3 px-4 py-3 lg:grid-cols-[minmax(0,1fr)_120px_120px_150px_150px]"
            >
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <span class="truncate font-medium text-gray-900 dark:text-white">{{ group.endpoint_key }}</span>
                  <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="providerBadgeClass(group.provider)">
                    {{ providerLabel(group.provider) }}
                  </span>
                  <span v-if="!group.provider_detected" class="text-xs text-gray-400">
                    {{ t('admin.accountQuotaMonitor.overview.customProbeHint', '其他，保存时自动探测') }}
                  </span>
                </div>
                <p class="mt-1 truncate text-xs text-gray-500 dark:text-gray-400">
                  {{ sampleAccountNames(group) }}
                </p>
              </div>
              <div class="text-sm text-gray-600 dark:text-gray-300">
                <p class="text-xs text-gray-400">{{ t('admin.accountQuotaMonitor.overview.accounts', '账号数') }}</p>
                <p class="font-semibold">{{ group.account_count }}</p>
              </div>
              <div class="text-sm text-gray-600 dark:text-gray-300">
                <p class="text-xs text-gray-400">{{ t('admin.accountQuotaMonitor.overview.monitors', '监控数') }}</p>
                <p class="font-semibold">{{ group.monitor_count }}</p>
              </div>
              <div>
                <span class="rounded-md px-2 py-0.5 text-xs font-medium" :class="candidateStatusClass(group)">
                  {{ candidateStatusLabel(group) }}
                </span>
                <p v-if="group.duplicate_monitor_count > 0" class="mt-1 text-xs text-amber-600 dark:text-amber-400">
                  {{ t('admin.accountQuotaMonitor.overview.duplicateHint', { count: group.duplicate_monitor_count }, `多余 ${group.duplicate_monitor_count} 个`) }}
                </p>
              </div>
              <div class="flex justify-start lg:justify-end">
                <button
                  v-if="!group.covered"
                  class="btn btn-secondary btn-sm"
                  :disabled="creatingCandidateKey === group.endpoint_key || group.representative_account_id <= 0"
                  @click="createCandidateMonitor(group)"
                >
                  <Icon name="play" size="sm" :class="creatingCandidateKey === group.endpoint_key ? 'animate-pulse' : ''" />
                  {{ t('admin.accountQuotaMonitor.overview.createOne', '探测创建') }}
                </button>
                <span v-else class="text-xs text-gray-400">{{ t('admin.accountQuotaMonitor.overview.covered', '已覆盖') }}</span>
              </div>
            </div>
          </div>
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
          <button class="btn btn-secondary" @click="openBatchForm">
            <Icon name="users" size="sm" />
            {{ t('admin.accountQuotaMonitor.batchCreate', '批量补齐监控') }}
          </button>
          <button class="btn btn-primary" @click="openCreateForm">
            <Icon name="plus" size="sm" />
            {{ t('admin.accountQuotaMonitor.create', '新增额度监控') }}
          </button>
        </div>
      </div>

      <div v-if="showBatchForm" class="rounded-xl border border-blue-200 bg-blue-50/50 p-4 dark:border-blue-800 dark:bg-blue-900/10">
        <div class="mb-3 flex items-center justify-between">
          <div>
            <h4 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.accountQuotaMonitor.batchCreate', '批量补齐监控') }}</h4>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.accountQuotaMonitor.batchHint', '按账号筛选条件为全部渠道账号补齐额度监控；已存在的监控默认跳过。') }}</p>
          </div>
          <button type="button" class="text-gray-400 hover:text-gray-600" @click="closeBatchForm">
            <Icon name="x" size="sm" />
          </button>
        </div>
        <form class="grid gap-4 lg:grid-cols-3" @submit.prevent="submitBatchForm">
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.batch.accountPlatform', '账号平台') }}</label>
            <Select v-model="batchForm.account_platform" :options="accountPlatformFilterOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.batch.accountType', '账号类型') }}</label>
            <Select v-model="batchForm.account_type" :options="accountTypeFilterOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.batch.accountStatus', '账号状态') }}</label>
            <Select v-model="batchForm.status" :options="accountStatusFilterOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.batch.search', '账号搜索') }}</label>
            <input v-model="batchForm.search" class="input" :placeholder="t('admin.accountQuotaMonitor.batch.searchPlaceholder', '名称关键字，可留空')" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.provider', '中转平台') }}</label>
            <Select v-model="batchForm.provider" :options="providerOptions" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.endpoint', '余额查询端点 / Base URL') }}</label>
            <input v-model="batchForm.endpoint" class="input" placeholder="https://example.com" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.interval', '查询间隔（秒）') }}</label>
            <input v-model.number="batchForm.interval_seconds" type="number" min="60" max="86400" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.threshold', '低余额阈值') }}</label>
            <input v-model.number="batchForm.low_balance_threshold" type="number" min="0" step="0.000001" class="input" placeholder="10" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.currency', '币种/单位') }}</label>
            <input v-model="batchForm.currency" class="input" placeholder="USD" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.accountQuotaMonitor.form.apiKeyOverride', '覆盖密钥（可选）') }}</label>
            <input v-model="batchForm.api_key_override" type="password" class="input" autocomplete="new-password" />
          </div>
          <div class="flex items-center gap-3">
            <Toggle :modelValue="batchForm.enabled" @update:modelValue="batchForm.enabled = $event" />
            <span class="text-sm text-gray-700 dark:text-gray-300">{{ t('admin.accountQuotaMonitor.form.enabled', '启用定时查询') }}</span>
          </div>
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="batchForm.update_existing" type="checkbox" class="rounded border-gray-300 text-primary-600" />
            {{ t('admin.accountQuotaMonitor.batch.updateExisting', '覆盖更新已存在监控') }}
          </label>
          <div class="flex justify-end gap-2 lg:col-span-3">
            <button type="button" class="btn btn-secondary" @click="closeBatchForm">{{ t('common.cancel', '取消') }}</button>
            <button type="submit" class="btn btn-primary" :disabled="submitting">
              {{ submitting ? t('common.submitting', '提交中...') : t('admin.accountQuotaMonitor.batch.submit', '批量创建/补齐') }}
            </button>
          </div>
        </form>
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
          <div v-if="pendingDeleteId === row.id" class="flex items-center gap-1">
            <button class="rounded-lg px-2 py-1 text-xs font-medium text-red-600 hover:bg-red-50 dark:hover:bg-red-900/20" @click="confirmDeleteMonitor(row)">
              {{ t('common.confirm', '确认') }}
            </button>
            <button class="rounded-lg px-2 py-1 text-xs text-gray-500 hover:bg-gray-100 dark:hover:bg-dark-700" @click="pendingDeleteId = null">
              {{ t('common.cancel', '取消') }}
            </button>
          </div>
          <div v-else class="flex items-center gap-1">
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
  CandidateGroup,
  CandidateOverviewResponse,
  HistoryItem,
  SummaryResponse,
  TrendPoint,
  BatchCreateParams,
} from '@/api/admin/accountQuotaMonitor'
import { extractApiErrorMessage } from '@/utils/apiError'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'

const props = defineProps<{ active?: boolean }>()

const { t } = useI18n()
const appStore = useAppStore()

const monitors = ref<AccountQuotaMonitor[]>([])
const loading = ref(false)
const submitting = ref(false)
const runningId = ref<number | null>(null)
const summaryData = ref<SummaryResponse | null>(null)
const trendPoints = ref<TrendPoint[]>([])
const candidateOverview = ref<CandidateOverviewResponse | null>(null)
const historyItems = ref<HistoryItem[]>([])
const selectedHistoryMonitor = ref<AccountQuotaMonitor | null>(null)
const creatingCandidateKey = ref<string | null>(null)
const creatingAllCandidates = ref(false)
const pendingDeleteId = ref<number | null>(null)

const filters = reactive({ search: '', provider: '' as '' | AccountQuotaProvider, enabled: '' as '' | 'true' | 'false' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let accountSearchTimeout: ReturnType<typeof setTimeout> | null = null

const showForm = ref(false)
const showBatchForm = ref(false)
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

const batchForm = reactive({
  account_platform: '',
  account_type: '',
  status: 'active',
  search: '',
  provider: 'sub2api' as AccountQuotaProvider,
  endpoint: '',
  api_key_override: '',
  enabled: true,
  interval_seconds: 3600,
  low_balance_threshold: null as number | null | '',
  currency: 'USD',
  update_existing: false,
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
  { value: 'custom', label: t('admin.accountQuotaMonitor.provider.custom', '其他/自动探测') },
])
const providerFilterOptions = computed(() => [{ value: '', label: t('admin.accountQuotaMonitor.allProviders', '全部平台') }, ...providerOptions.value])
const enabledFilterOptions = computed(() => [
  { value: '', label: t('admin.accountQuotaMonitor.allEnabled', '全部状态') },
  { value: 'true', label: t('common.enabled', '启用') },
  { value: 'false', label: t('common.disabled', '禁用') },
])
const accountPlatformFilterOptions = computed(() => [
  { value: '', label: t('admin.accountQuotaMonitor.batch.allAccountPlatforms', '全部账号平台') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
])
const accountTypeFilterOptions = computed(() => [
  { value: '', label: t('admin.accountQuotaMonitor.batch.allAccountTypes', '全部账号类型') },
  { value: 'oauth', label: 'OAuth' },
  { value: 'setup-token', label: 'Setup Token' },
  { value: 'apikey', label: 'API Key' },
  { value: 'upstream', label: 'Upstream' },
  { value: 'bedrock', label: 'Bedrock' },
  { value: 'service_account', label: 'Service Account' },
])
const accountStatusFilterOptions = computed(() => [
  { value: '', label: t('admin.accountQuotaMonitor.batch.allAccountStatus', '全部账号状态') },
  { value: 'active', label: t('admin.channels.statusActive', 'Active') },
  { value: 'inactive', label: t('admin.accounts.statusInactive', 'Inactive') },
  { value: 'error', label: t('admin.accounts.statusError', 'Error') },
])

const normalizedCurrencyTotals = computed(() => {
  const items = summaryData.value?.total_balance_by_currency || []
  return items.map((item) => ({
    currency: item.currency || item.Currency || 'USD',
    balance: Number(item.balance ?? item.Balance ?? 0),
    count: Number(item.count ?? item.Count ?? 0),
  }))
})

const candidateGroups = computed(() => candidateOverview.value?.groups || [])
const missingCandidateGroups = computed(() =>
  candidateGroups.value.filter((group) => !group.covered && group.representative_account_id > 0)
)
const visibleCandidateGroups = computed(() => candidateGroups.value.slice(0, 10))

watch(() => props.active, (active) => {
  if (active !== false) reload()
}, { immediate: true })

onUnmounted(() => {
  if (abortController) abortController.abort()
  if (searchTimeout) clearTimeout(searchTimeout)
  if (accountSearchTimeout) clearTimeout(accountSearchTimeout)
})

async function reload() {
  if (props.active === false) return
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
    const [listRes, summaryRes, trendRes, candidateRes] = await Promise.all([
      adminAPI.accountQuotaMonitor.list(params, { signal: ctrl.signal }),
      adminAPI.accountQuotaMonitor.summary(),
      adminAPI.accountQuotaMonitor.trend(7, 'hour'),
      adminAPI.accountQuotaMonitor.candidates(),
    ])
    if (ctrl.signal.aborted || abortController !== ctrl) return
    monitors.value = listRes.items || []
    pagination.total = listRes.total
    summaryData.value = summaryRes
    trendPoints.value = trendRes.items || []
    candidateOverview.value = candidateRes
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
  showBatchForm.value = false
  showForm.value = true
}

function resetBatchForm() {
  batchForm.account_platform = ''
  batchForm.account_type = ''
  batchForm.status = 'active'
  batchForm.search = ''
  batchForm.provider = 'sub2api'
  batchForm.endpoint = ''
  batchForm.api_key_override = ''
  batchForm.enabled = true
  batchForm.interval_seconds = 3600
  batchForm.low_balance_threshold = null
  batchForm.currency = 'USD'
  batchForm.update_existing = false
}

function openBatchForm() {
  resetBatchForm()
  showForm.value = false
  showBatchForm.value = true
}

function closeBatchForm() {
  showBatchForm.value = false
}

function openEditForm(row: AccountQuotaMonitor) {
  resetForm()
  showBatchForm.value = false
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

function normalizedBatchThreshold(): number | null {
  if (batchForm.low_balance_threshold === null || batchForm.low_balance_threshold === '' || batchForm.low_balance_threshold === undefined) return null
  const value = Number(batchForm.low_balance_threshold)
  return Number.isFinite(value) ? value : null
}

async function submitBatchForm() {
  submitting.value = true
  try {
    const payload: BatchCreateParams = {
      filters: {
        platform: batchForm.account_platform || undefined,
        type: batchForm.account_type || undefined,
        status: batchForm.status || undefined,
        search: batchForm.search.trim() || undefined,
      },
      provider: batchForm.provider,
      endpoint: batchForm.endpoint.trim(),
      enabled: batchForm.enabled,
      interval_seconds: Number(batchForm.interval_seconds) || 3600,
      low_balance_threshold: normalizedBatchThreshold(),
      currency: batchForm.currency.trim() || 'USD',
      api_key_override: batchForm.api_key_override.trim() || undefined,
      update_existing: batchForm.update_existing,
      max_accounts: 1000,
    }
    const result = await adminAPI.accountQuotaMonitor.batchCreate(payload)
    appStore.showSuccess(t(
      'admin.accountQuotaMonitor.batch.success',
      {
        created: result.created,
        updated: result.updated,
        skipped: result.skipped_existing,
        duplicates: result.skipped_duplicate || 0,
        failed: result.failed
      },
      `已创建 ${result.created} 个，更新 ${result.updated} 个，跳过 ${result.skipped_existing} 个，重复端点 ${result.skipped_duplicate || 0} 个，失败 ${result.failed} 个`
    ))
    closeBatchForm()
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.batch.error', '批量创建额度监控失败')))
  } finally {
    submitting.value = false
  }
}

async function createCandidateMonitor(group: CandidateGroup) {
  if (!group || group.representative_account_id <= 0) return
  creatingCandidateKey.value = group.endpoint_key
  try {
    const result = await adminAPI.accountQuotaMonitor.batchCreate({
      account_ids: [group.representative_account_id],
      provider: group.provider || 'custom',
      endpoint: group.endpoint,
      enabled: true,
      interval_seconds: 3600,
      currency: 'USD',
      update_existing: false,
      max_accounts: 1,
    })
    if (result.created > 0 || result.updated > 0) {
      appStore.showSuccess(t('admin.accountQuotaMonitor.overview.createSuccess', '已通过真实余额接口验证并创建监控'))
    } else {
      const reason = result.failures?.[0]?.reason || t('admin.accountQuotaMonitor.overview.createSkipped', '未创建，可能已存在或接口不可用')
      appStore.showWarning(reason)
    }
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('admin.accountQuotaMonitor.overview.createError', '探测创建失败')))
  } finally {
    creatingCandidateKey.value = null
  }
}

async function createAllCandidateMonitors() {
  const targets = missingCandidateGroups.value
  if (targets.length === 0) return
  creatingAllCandidates.value = true
  let created = 0
  let failed = 0
  try {
    for (const group of targets) {
      creatingCandidateKey.value = group.endpoint_key
      try {
        const result = await adminAPI.accountQuotaMonitor.batchCreate({
          account_ids: [group.representative_account_id],
          provider: group.provider || 'custom',
          endpoint: group.endpoint,
          enabled: true,
          interval_seconds: 3600,
          currency: 'USD',
          update_existing: false,
          max_accounts: 1,
        })
        created += result.created + result.updated
        failed += result.failed
        if (result.created === 0 && result.updated === 0 && result.failed === 0) failed += 1
      } catch {
        failed += 1
      }
    }
    const message = t(
      'admin.accountQuotaMonitor.overview.createAllResult',
      { created, failed },
      `已创建 ${created} 个监控，失败/跳过 ${failed} 个`
    )
    if (failed > 0) appStore.showWarning(message)
    else appStore.showSuccess(message)
    await reload()
  } finally {
    creatingCandidateKey.value = null
    creatingAllCandidates.value = false
  }
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
  pendingDeleteId.value = row.id
}

async function confirmDeleteMonitor(row: AccountQuotaMonitor) {
  try {
    await adminAPI.accountQuotaMonitor.del(row.id)
    appStore.showSuccess(t('common.deleted', '已删除'))
    pendingDeleteId.value = null
    await reload()
  } catch (err) {
    appStore.showError(extractApiErrorMessage(err, t('common.error', '操作失败')))
  }
}

function providerLabel(provider: AccountQuotaProvider) {
  if (provider === 'newapi') return 'NewAPI'
  if (provider === 'sub2api') return 'sub2api'
  return t('admin.accountQuotaMonitor.provider.custom', '其他/自动探测')
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

function candidateStatusLabel(group: CandidateGroup) {
  if (group.covered && group.duplicate_monitor_count > 0) {
    return t('admin.accountQuotaMonitor.overview.statusDuplicate', '已覆盖，有重复')
  }
  if (group.covered) {
    return t('admin.accountQuotaMonitor.overview.statusCovered', '已覆盖')
  }
  return t('admin.accountQuotaMonitor.overview.statusMissing', '待补齐')
}

function candidateStatusClass(group: CandidateGroup) {
  if (group.covered && group.duplicate_monitor_count > 0) return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (group.covered) return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
  return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
}

function sampleAccountNames(group: CandidateGroup) {
  const names = (group.sample_accounts || []).map((account) => `${account.name || ('#' + account.id)} #${account.id}`)
  if (names.length === 0) return t('admin.accountQuotaMonitor.overview.noSampleAccounts', '暂无样例账号')
  const suffix = group.account_count > names.length ? ` +${group.account_count - names.length}` : ''
  return names.join('、') + suffix
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
