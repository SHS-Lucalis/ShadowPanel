<script setup>
import { inject, ref, computed, onMounted, onUnmounted } from 'vue'

const { close, activeIndex, registerItem, unregisterItem, setActiveIndex } = inject('menu')

const itemId = Symbol()
const index = ref(-1)

onMounted(() => {
  index.value = registerItem(itemId)
})

onUnmounted(() => {
  unregisterItem(itemId)
})

const active = computed(() => activeIndex.value === index.value)

const handleMouseEnter = () => {
  setActiveIndex(index.value)
}

const handleMouseLeave = () => {
  setActiveIndex(-1)
}
</script>

<template>
  <div
    role="menuitem"
    @mouseenter="handleMouseEnter"
    @mouseleave="handleMouseLeave"
  >
    <slot :active="active" :close="close" />
  </div>
</template>
