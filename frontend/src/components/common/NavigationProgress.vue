<script setup lang="ts">
/**
 * 导航进度条组件
 * 在页面顶部显示加载进度，提供导航反馈
 */
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'

const { isLoading } = useNavigationLoadingState()
const { t } = useI18n()

// 进度条可见性
const isVisible = computed(() => isLoading.value)
</script>

<template>
  <Transition name="progress-fade">
    <div
      v-show="isVisible"
      class="navigation-progress"
      role="progressbar"
      :aria-label="t('common.loading')"
      aria-valuemin="0"
      aria-valuemax="100"
    >
      <div class="navigation-progress-bar" />
    </div>
  </Transition>
</template>

<style scoped>
.navigation-progress {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 4px;
  z-index: 9999;
  overflow: hidden;
  background: #161616;
}

.navigation-progress-bar {
  height: 100%;
  width: 24%;
  background: var(--ef-signal);
}

@media (prefers-reduced-motion: no-preference) {
  .navigation-progress-bar { animation: progress-slide 1200ms steps(8, end) infinite; }
}

/* 进度条滑动动画 */
@keyframes progress-slide {
  0% {
    transform: translateX(-100%);
  }
  100% {
    transform: translateX(420%);
  }
}

/* 淡入淡出过渡 */
.progress-fade-enter-active {
  transition: opacity 0.15s ease-out;
}

.progress-fade-leave-active {
  transition: opacity 0.3s ease-out;
}

.progress-fade-enter-from,
.progress-fade-leave-to {
  opacity: 0;
}

/* 减少动画模式 */
@media (prefers-reduced-motion: reduce) {
  .progress-fade-enter-active,
  .progress-fade-leave-active { transition: none; }
  .navigation-progress-bar {
    animation: none;
    transform: none;
    width: 100%;
    background: repeating-linear-gradient(90deg, var(--ef-signal) 0 12px, #161616 12px 16px);
  }
}
</style>
