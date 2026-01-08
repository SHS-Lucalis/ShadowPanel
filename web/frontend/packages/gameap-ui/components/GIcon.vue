<template>
  <component
    v-if="isComponent"
    :is="resolvedIcon"
    :class="props.class"
    :style="sizeStyle"
  />
  <i
    v-else
    :class="[resolvedIcon, sizeClass, props.class]"
  />
</template>

<script setup>
import { computed } from 'vue'
import { getIcon, hasIcon } from '../icons/registry.js'

const props = defineProps({
  name: {
    type: String,
    required: true
  },
  size: {
    type: String,
    default: 'md',
    validator: (value) => ['sm', 'md', 'lg', 'xl'].includes(value)
  },
  class: {
    type: String,
    default: ''
  }
})

const resolvedIcon = computed(() => {
  if (!hasIcon(props.name)) {
    console.warn(`GIcon: Unknown icon "${props.name}"`)
    return 'fa-solid fa-circle-question'
  }
  return getIcon(props.name)
})

const isComponent = computed(() => {
  return typeof resolvedIcon.value === 'object' || typeof resolvedIcon.value === 'function'
})

const fontAwesomeSizeMap = {
  sm: 'fa-sm',
  md: '',
  lg: 'fa-lg',
  xl: 'fa-2x'
}

const componentSizeMap = {
  sm: '0.875em',
  md: '1em',
  lg: '1.25em',
  xl: '2em'
}

const sizeClass = computed(() => fontAwesomeSizeMap[props.size] || '')

const sizeStyle = computed(() => ({
  width: componentSizeMap[props.size],
  height: componentSizeMap[props.size]
}))
</script>
