<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accountQuotaMonitor.title', '额度监控')"
    width="full"
    @close="emit('close')"
  >
    <AccountQuotaMonitorPanel :active="show" />
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import AccountQuotaMonitorPanel from './AccountQuotaMonitorPanel.vue'

defineProps<{ show: boolean }>()
const emit = defineEmits<{ (e: 'close'): void }>()
const { t } = useI18n()
</script>
