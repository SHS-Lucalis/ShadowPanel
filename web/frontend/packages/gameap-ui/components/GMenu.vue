<script setup>
import { ref, provide, onMounted, onUnmounted } from 'vue'

const props = defineProps({
  as: { type: String, default: 'div' }
})

const isOpen = ref(false)
const menuRef = ref(null)
const activeIndex = ref(-1)
const items = ref([])

const toggle = () => {
  isOpen.value = !isOpen.value
  if (!isOpen.value) activeIndex.value = -1
}

const close = () => {
  isOpen.value = false
  activeIndex.value = -1
}

const registerItem = (id) => {
  items.value.push(id)
  return items.value.length - 1
}

const unregisterItem = (id) => {
  const idx = items.value.indexOf(id)
  if (idx > -1) items.value.splice(idx, 1)
}

const setActiveIndex = (index) => {
  activeIndex.value = index
}

const handleClickOutside = (event) => {
  if (menuRef.value && !menuRef.value.contains(event.target)) {
    close()
  }
}

onMounted(() => {
  document.addEventListener('click', handleClickOutside, true)
})

onUnmounted(() => {
  document.removeEventListener('click', handleClickOutside, true)
})

provide('menu', {
  isOpen,
  toggle,
  close,
  activeIndex,
  items,
  registerItem,
  unregisterItem,
  setActiveIndex
})
</script>

<template>
  <component :is="as" ref="menuRef">
    <slot />
  </component>
</template>
