<template>
  <button
    type="button"
    @click="toggle"
    class="ef-toggle"
    role="switch"
    :aria-checked="modelValue"
  >
    <span
      class="ef-toggle__thumb"
    />
  </button>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void
}>()

function toggle() {
  emit('update:modelValue', !props.modelValue)
}
</script>
