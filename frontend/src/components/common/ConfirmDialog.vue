<template>
  <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-400">{{ message }}</p>
      <slot></slot>
    </div>

    <template #footer>
      <div class="flex justify-end space-x-3">
        <button
          :disabled="loading"
          @click="handleCancel"
          type="button"
          class="btn btn-secondary"
        >
          {{ cancelText }}
        </button>
        <button
          :disabled="loading"
          @click="handleConfirm"
          type="button"
          :class="['btn', danger ? 'btn-danger' : 'btn-primary']"
        >
          {{ confirmText }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'

const { t } = useI18n()

interface Props {
  show: boolean
  title: string
  message: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  loading?: boolean
}

interface Emits {
  (e: 'confirm'): void
  (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
  danger: false,
  loading: false
})

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()

const handleConfirm = () => {
  if (!props.loading) emit('confirm')
}

const handleCancel = () => {
  if (!props.loading) emit('cancel')
}
</script>
