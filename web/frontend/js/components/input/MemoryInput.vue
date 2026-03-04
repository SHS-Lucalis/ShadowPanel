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
      style="width: 100px"
    />
  </n-input-group>
</template>

<script setup>
import { ref, watch, computed } from 'vue'
import { NInputGroup, NInputNumber, NSelect } from 'naive-ui'

const props = defineProps({
  modelValue: {
    type: Number,
    default: null
  }
})

const emit = defineEmits(['update:modelValue'])

const BYTES_IN_MB = 1024 * 1024
const BYTES_IN_GB = 1024 * 1024 * 1024

const unitOptions = [
  { label: 'Bytes', value: 'B' },
  { label: 'MB', value: 'M' },
  { label: 'GB', value: 'G' }
]

const unit = ref('M')
const displayValue = ref(null)
const isInternalUpdate = ref(false)
const isUnitChangeFromLoad = ref(false)

const precision = computed(() => unit.value === 'B' ? 0 : 2)

const smartRound = (value) => {
  const nearestInt = Math.round(value)
  if (Math.abs(value - nearestInt) < 0.05) {
    return nearestInt
  }

  const nearestHalf = Math.round(value * 2) / 2
  if (Math.abs(value - nearestHalf) < 0.02) {
    return nearestHalf
  }

  return Math.round(value * 100) / 100
}

const determineUnit = (bytes) => {
  if (bytes === null || bytes === undefined || bytes === 0) {
    return 'M'
  }
  if (bytes >= BYTES_IN_GB && bytes % BYTES_IN_GB === 0) {
    return 'G'
  }
  if (bytes >= BYTES_IN_MB && bytes % BYTES_IN_MB === 0) {
    return 'M'
  }
  if (bytes >= BYTES_IN_GB) {
    return 'G'
  }
  if (bytes >= BYTES_IN_MB) {
    return 'M'
  }
  return 'B'
}

const convertToDisplay = (bytes, targetUnit) => {
  if (bytes === null || bytes === undefined) {
    return null
  }
  switch (targetUnit) {
    case 'G':
      return bytes / BYTES_IN_GB
    case 'M':
      return bytes / BYTES_IN_MB
    default:
      return bytes
  }
}

const convertToBytes = (value, sourceUnit) => {
  if (value === null || value === undefined) {
    return null
  }
  switch (sourceUnit) {
    case 'G':
      return Math.round(value * BYTES_IN_GB)
    case 'M':
      return Math.round(value * BYTES_IN_MB)
    default:
      return Math.round(value)
  }
}

watch(() => props.modelValue, (newValue) => {
  if (isInternalUpdate.value) {
    isInternalUpdate.value = false
    return
  }
  const newUnit = determineUnit(newValue)
  isUnitChangeFromLoad.value = true
  unit.value = newUnit
  const rawDisplayValue = convertToDisplay(newValue, newUnit)
  displayValue.value = newUnit === 'B' ? Math.round(rawDisplayValue) : smartRound(rawDisplayValue)
}, { immediate: true })

watch([displayValue, unit], ([newDisplayValue, newUnit]) => {
  const bytes = convertToBytes(newDisplayValue, newUnit)
  if (bytes !== props.modelValue) {
    isInternalUpdate.value = true
    emit('update:modelValue', bytes)
  }
})

watch(unit, (newUnit, oldUnit) => {
  if (isUnitChangeFromLoad.value) {
    isUnitChangeFromLoad.value = false
    return
  }
  if (displayValue.value !== null) {
    const bytes = convertToBytes(displayValue.value, oldUnit)
    const newDisplayValue = convertToDisplay(bytes, newUnit)
    displayValue.value = newUnit === 'B' ? Math.round(newDisplayValue) : smartRound(newDisplayValue)
  }
})
</script>
