<template>
  <div class="mb-5 flex flex-col gap-3 md:flex-row md:flex-wrap md:items-center md:gap-4">
    <div class="flex-1 md:max-w-md md:min-w-[220px]">
      <n-input
          v-model:value="searchInput"
          :placeholder="trans('dedicated_servers.filter_search_placeholder')"
          clearable
      >
        <template #prefix>
          <GIcon name="search" />
        </template>
      </n-input>
    </div>

    <n-radio-group
        :value="statusFilter"
        size="small"
        @update:value="setStatus"
    >
      <n-radio-button
          v-for="opt in statusOptions"
          :key="opt.value"
          :value="opt.value"
      >
        <span class="inline-flex items-center gap-1.5">
          <GIcon v-if="opt.icon" :name="opt.icon" />
          <span>{{ opt.label }}</span>
        </span>
      </n-radio-button>
    </n-radio-group>

    <n-radio-group
        :value="osFilter"
        size="small"
        @update:value="setOs"
    >
      <n-radio-button
          v-for="opt in osOptions"
          :key="opt.value"
          :value="opt.value"
      >
        <span class="inline-flex items-center gap-1.5">
          <GIcon v-if="opt.icon" :name="opt.icon" />
          <span>{{ opt.label }}</span>
        </span>
      </n-radio-button>
    </n-radio-group>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { NInput, NRadioGroup, NRadioButton } from 'naive-ui'
import { GIcon } from '@gameap/ui'
import { trans } from '@/i18n/i18n'

const props = defineProps({
    modelValue: {
        type: Object,
        default: () => ({ search: '', status: 'all', os: 'all' }),
    },
})

const emit = defineEmits(['update:modelValue'])

const searchInput = ref(props.modelValue.search || '')
const statusFilter = ref(props.modelValue.status || 'all')
const osFilter = ref(props.modelValue.os || 'all')

const statusOptions = computed(() => [
    { value: 'all', label: trans('main.all'), icon: null },
    { value: 'online', label: trans('dedicated_servers.online'), icon: 'online' },
    { value: 'offline', label: trans('dedicated_servers.offline'), icon: 'offline' },
])

const osOptions = computed(() => [
    { value: 'all', label: trans('main.all'), icon: null },
    { value: 'linux', label: 'Linux', icon: 'linux' },
    { value: 'windows', label: 'Windows', icon: 'windows' },
    { value: 'macos', label: 'macOS', icon: 'apple' },
])

let searchTimer = null
watch(searchInput, (v) => {
    clearTimeout(searchTimer)
    searchTimer = setTimeout(() => emitUpdate({ search: v }), 200)
})

function setStatus(v) {
    statusFilter.value = v
    emitUpdate({ status: v })
}

function setOs(v) {
    osFilter.value = v
    emitUpdate({ os: v })
}

function emitUpdate(patch) {
    emit('update:modelValue', {
        search: searchInput.value,
        status: statusFilter.value,
        os: osFilter.value,
        ...patch,
    })
}

watch(
    () => props.modelValue,
    (v) => {
        searchInput.value = v.search || ''
        statusFilter.value = v.status || 'all'
        osFilter.value = v.os || 'all'
    },
    { deep: true },
)
</script>
