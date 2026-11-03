<template>
  <slot v-if="hasWorkspaceLayout" />
  <WorkspaceLayout v-else ref="workspace"><slot /></WorkspaceLayout>
</template>

<script setup lang="ts">
import { inject, provide, ref } from 'vue'
import WorkspaceLayout from './WorkspaceLayout.vue'
import { workspaceLayoutKey } from './workspaceLayoutContext'

// Views can still render independently, while routed views reuse their parent's chrome.
const hasWorkspaceLayout = inject(workspaceLayoutKey, false)
provide(workspaceLayoutKey, true)
const workspace = ref<InstanceType<typeof WorkspaceLayout>>()
defineExpose({ replayTour: () => workspace.value?.replayTour() })
</script>
