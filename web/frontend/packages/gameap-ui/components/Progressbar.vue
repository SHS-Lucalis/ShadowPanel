<template>
  <div class="progress">
    <div
      class="progress-bar progress-bar-info"
      role="progressbar"
      :aria-valuenow="progress"
      aria-valuemin="0"
      aria-valuemax="100"
      :style="{ width: progress + '%'}"
    >
      {{ progress }}%
    </div>
  </div>
</template>

<script setup>
defineProps({
  progress: {
    type: Number,
    default: 0
  }
})
</script>
