<template>
  <AppLayout>
    <div class="space-y-5">
      <div class="rounded-2xl border border-gray-200 bg-white p-5 shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
              {{ t('admin.accountQuotaMonitor.title', '额度监控') }}
            </h1>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
              {{ t('admin.accountQuotaMonitor.pageDescription', '按中转平台 Base URL 去重监控账号余额，自动跳过接口不可用的配置。') }}
            </p>
          </div>
          <router-link to="/admin/channels/pricing" class="btn btn-secondary">
            <Icon name="arrowLeft" size="sm" />
            {{ t('admin.accountQuotaMonitor.backToChannels', '返回渠道定价') }}
          </router-link>
        </div>
      </div>

      <AccountQuotaMonitorPanel />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import AccountQuotaMonitorPanel from '@/components/admin/AccountQuotaMonitorPanel.vue'

const { t } = useI18n()
</script>
