<template>
  <div class="fixed bottom-0 left-0 right-0 z-30 bg-white dark:bg-stone-800 border-t border-stone-200 dark:border-stone-700 py-3 px-4">
    <div class="flex justify-end" :class="contentClass">
      <slot />
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { useUISettingsStore } from '@/store/uiSettings'

const uiSettingsStore = useUISettingsStore()

const contentClass = computed(() => {
  return uiSettingsStore.isMenuMinimized ? 'sm:ml-16' : 'sm:ml-56'
})
</script>
