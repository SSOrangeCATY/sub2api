<template>
  <div class="space-y-2">
    <div class="flex items-center gap-2">
      <input :id="id" :value="modelValue ?? ''" type="number" step="0.0001" min="0.0001" max="999999.9999" class="input min-w-0 flex-1" :placeholder="t('admin.groups.monitorFallback.ratePlaceholder')" :aria-label="t('admin.groups.monitorFallback.entryRate')" :aria-describedby="id + '-hint'" @input="update" />
      <button v-if="modelValue != null" type="button" class="btn btn-secondary shrink-0" :aria-label="t('admin.groups.monitorFallback.clearRate')" @click="emit('update:modelValue', null)">{{ t('admin.groups.monitorFallback.clearRate') }}</button>
    </div>
    <p :id="id + '-hint'" class="input-hint">{{ t('admin.groups.monitorFallback.rateHint') }}</p>
  </div>
</template>
<script setup lang="ts">
import { useI18n } from 'vue-i18n'
defineProps<{ id: string; modelValue: number | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: number | null] }>()
const { t } = useI18n()
function update(event: Event) {
  const input = event.target as HTMLInputElement
  emit('update:modelValue', input.value === '' ? null : input.valueAsNumber)
}
</script>
