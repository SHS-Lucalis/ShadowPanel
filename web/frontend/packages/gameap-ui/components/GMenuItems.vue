<script setup>
import { inject, computed } from 'vue'

const props = defineProps({
  unmount: { type: Boolean, default: true }
})

const { isOpen, close } = inject('menu')

const handleKeydown = (event) => {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
  }
}

const shouldRender = computed(() => {
  if (props.unmount) return isOpen.value
  return true
})

const shouldShow = computed(() => isOpen.value)
</script>

<template>
  <div
    v-if="shouldRender"
    v-show="shouldShow"
    role="menu"
    tabindex="-1"
    @keydown="handleKeydown"
  >
    <slot />
  </div>
</template>
