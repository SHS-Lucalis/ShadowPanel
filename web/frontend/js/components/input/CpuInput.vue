<template>
  <n-input-group>
    <n-input-number
      v-model:value="displayValue"
      :min="0"
      :precision="precision"
      style="flex: 1"
    />
    <n-select
      v-model:value="unit"
      :options="unitOptions"
      style="width: 130px"
    />
  </n-input-group>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import { NInputGroup, NInputNumber, NSelect } from 'naive-ui'
import { trans } from '@/i18n/i18n'

const props = defineProps({
  modelValue: {
    type: Number,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

const unitOptions = [
  { label: 'millicores', value: 'millicores' },
  { label: '%', value: 'percent' },
  { label: 'cores', value: 'cores' }
]

const unit = ref('millicores')
const displayValue = ref(null)
const isInternalUpdate = ref(false)

const precision = computed(() => {
  switch (unit.value) {
    case 'cores':
      return 2
    case 'percent':
      return 0
    case 'millicores':
    default:
      return 0
  }
})

const convertToDisplay = (millicores, targetUnit) => {
  if (millicores === null || millicores === undefined) {
    return null
  }
  switch (targetUnit) {
    case 'cores':
      return millicores / 1000
    case 'percent':
      return millicores / 10
    case 'millicores':
    default:
      return millicores
  }
}

const convertToMillicores = (value, sourceUnit) => {
  if (value === null || value === undefined) {
    return null
  }
  switch (sourceUnit) {
    case 'cores':
      return Math.round(value * 1000)
    case 'percent':
      return Math.round(value * 10)
    case 'millicores':
    default:
      return Math.round(value)
  }
}

watch(() => props.modelValue, (newValue) => {
  if (isInternalUpdate.value) {
    isInternalUpdate.value = false
    return
  }
  displayValue.value = convertToDisplay(newValue, unit.value)
}, { immediate: true })

watch([displayValue, unit], ([newDisplayValue, newUnit]) => {
  const millicores = convertToMillicores(newDisplayValue, newUnit)
  if (millicores !== props.modelValue) {
    isInternalUpdate.value = true
    emit('update:modelValue', millicores)
  }
})

watch(unit, (newUnit, oldUnit) => {
  if (displayValue.value !== null) {
    const millicores = convertToMillicores(displayValue.value, oldUnit)
    displayValue.value = convertToDisplay(millicores, newUnit)
  }
})
</script>
