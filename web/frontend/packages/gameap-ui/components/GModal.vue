<template>
  <n-modal
    v-bind="mergedProps"
  >
    <template v-for="(_, slotName) in $slots" :key="slotName" #[slotName]="slotProps">
      <slot :name="slotName" v-bind="slotProps || {}" />
    </template>
  </n-modal>
</template>

<script setup>
import { computed, useAttrs } from 'vue'
import { NModal } from 'naive-ui'

const props = defineProps({
  show: {
    type: Boolean,
    default: false
  },
  preset: {
    type: String,
    default: 'card'
  },
  bordered: {
    type: Boolean,
    default: false
  },
  title: {
    type: String,
    default: ''
  },
  segmented: {
    type: Object,
    default: () => ({ content: 'soft', footer: 'soft' })
  }
})

defineOptions({
  inheritAttrs: false
})

const emit = defineEmits(['update:show'])

const attrs = useAttrs()

const mergedProps = computed(() => ({
  show: props.show,
  'onUpdate:show': (value) => emit('update:show', value),
  preset: props.preset,
  bordered: props.bordered,
  title: props.title,
  segmented: props.segmented,
  class: 'custom-card',
  ...attrs
}))
</script>
