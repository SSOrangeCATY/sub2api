<template>
  <div
    :class="[
      'pt-skeleton',
      customClass
    ]"
    :style="style"
    aria-hidden="true"
  ></div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

interface Props {
  variant?: 'rect' | 'circle' | 'text'
  width?: string | number
  height?: string | number
  class?: string
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'rect',
  width: '100%'
})

const customClass = computed(() => props.class || '')

const style = computed(() => {
  const s: Record<string, string> = {}
  
  if (props.width) {
    s.width = typeof props.width === 'number' ? `${props.width}px` : props.width
  }
  
  if (props.height) {
    s.height = typeof props.height === 'number' ? `${props.height}px` : props.height
  } else if (props.variant === 'text') {
    s.height = '1em'
    s.marginTop = '0.25em'
    s.marginBottom = '0.25em'
  }
  
  return s
})
</script>

<style scoped>
.pt-skeleton { background-color: var(--ef-chip, #e4e4e0); background-image: repeating-linear-gradient(135deg, transparent 0 5px, var(--ef-rule, #d9d9d6) 5px 6px); border-radius: 0; }
</style>
