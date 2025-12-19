<script setup>
import { inject } from 'vue'

const { isOpen, toggle } = inject('menu')

const handleClick = () => {
  toggle()
}

const handleKeydown = (event) => {
  if (['Enter', ' ', 'ArrowDown'].includes(event.key)) {
    event.preventDefault()
    toggle()
  }
}
</script>

<template>
  <button
    type="button"
    :aria-expanded="isOpen"
    aria-haspopup="true"
    @click="handleClick"
    @keydown="handleKeydown"
  >
    <slot />
  </button>
</template>
