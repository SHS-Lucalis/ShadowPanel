<template>
  <div class="flex justify-center">
    <Transition
        enter-active-class="transition-opacity duration-[2000ms]"
        enter-from-class="opacity-0"
        enter-to-class="opacity-100"
        leave-active-class="transition-opacity duration-150"
        leave-from-class="opacity-100"
        leave-to-class="opacity-0"
    >
      <div v-if="showTransition" class="fa-3x">
        <i class="fa-solid fa-gear fa-spin"></i>
      </div>
    </Transition>
  </div>
</template>

<script setup>
import { onMounted, ref } from "vue"

const showTransition = ref(false)

onMounted(() => {
  setTimeout(() => {
    showTransition.value = true
  }, 10)
})
</script>
