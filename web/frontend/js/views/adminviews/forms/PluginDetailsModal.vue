<template>
  <div class="plugin-details">
    <div v-if="loading && !plugin" class="flex justify-center py-8">
      <Loading />
    </div>

    <div v-else-if="plugin">
      <div class="flex items-start gap-4 mb-4">
        <img
            v-if="plugin.icon_url"
            :src="plugin.icon_url"
            :alt="plugin.name"
            class="w-16 h-16 rounded-lg"
        />
        <i v-else class="fa-solid fa-puzzle-piece text-5xl text-gray-400"></i>

        <div class="flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <h2 class="text-xl font-bold">{{ plugin.name }}</h2>
            <n-tag v-if="plugin.installed" type="success" size="small">
              {{ trans('plugins.already_installed') }}
            </n-tag>
          </div>

          <div v-if="plugin.labels?.length > 0" class="flex gap-1 mt-1">
            <n-tag
                v-for="label in plugin.labels"
                :key="label.id"
                size="tiny"
                round
                :style="label.color ? { backgroundColor: label.color, color: '#fff' } : {}"
            >
              {{ label.name }}
            </n-tag>
          </div>

          <div class="flex items-center gap-2 mt-2">
            <span class="text-yellow-500 text-lg">{{ renderStars(plugin.rating_avg) }}</span>
            <span class="text-gray-500">({{ plugin.rating_count || 0 }} {{ trans('plugins.reviews') }})</span>
          </div>
        </div>
      </div>

      <div v-if="plugin.summary" class="mb-4 text-gray-600 dark:text-gray-400">
        {{ plugin.summary }}
      </div>

      <div v-if="plugin.description" class="mb-4">
        <h3 class="font-semibold mb-2">{{ trans('plugins.description') }}</h3>
        <div class="text-sm whitespace-pre-wrap">{{ plugin.description }}</div>
      </div>

      <n-divider />

      <div class="grid grid-cols-2 gap-4 text-sm mb-4">
        <div v-if="plugin.author">
          <span class="text-gray-500">{{ trans('plugins.author') }}:</span>
          <span class="ml-2">{{ plugin.author.username }}</span>
        </div>
        <div v-if="plugin.category">
          <span class="text-gray-500">{{ trans('plugins.category') }}:</span>
          <span class="ml-2">{{ plugin.category.name }}</span>
        </div>
        <div v-if="plugin.license">
          <span class="text-gray-500">{{ trans('plugins.license') }}:</span>
          <span class="ml-2">{{ plugin.license }}</span>
        </div>
        <div v-if="plugin.download_count !== undefined">
          <span class="text-gray-500">{{ trans('plugins.downloads') }}:</span>
          <span class="ml-2">{{ formatNumber(plugin.download_count) }}</span>
        </div>
        <div v-if="plugin.min_gameap_version">
          <span class="text-gray-500">{{ trans('plugins.min_gameap_version') }}:</span>
          <span class="ml-2">{{ plugin.min_gameap_version }}</span>
        </div>
        <div v-if="plugin.min_plugin_api_version">
          <span class="text-gray-500">{{ trans('plugins.min_plugin_api') }}:</span>
          <span class="ml-2">{{ plugin.min_plugin_api_version }}</span>
        </div>
        <div v-if="plugin.repository_url" class="col-span-2">
          <span class="text-gray-500">{{ trans('plugins.repository') }}:</span>
          <a :href="plugin.repository_url" target="_blank" class="ml-2 text-blue-500 hover:underline">
            {{ plugin.repository_url }}
          </a>
        </div>
        <div v-if="plugin.published_at">
          <span class="text-gray-500">{{ trans('plugins.published_at') }}:</span>
          <span class="ml-2">{{ formatDate(plugin.published_at) }}</span>
        </div>
      </div>

      <div v-if="plugin.installed" class="mb-4 p-3 bg-gray-100 dark:bg-gray-800 rounded">
        <div class="flex justify-between items-center">
          <div>
            <span class="text-gray-500">{{ trans('plugins.installed_version') }}:</span>
            <span class="ml-2 font-medium">{{ plugin.installed_version }}</span>
          </div>
          <div>
            <span class="text-gray-500">{{ trans('plugins.latest_version') }}:</span>
            <span class="ml-2 font-medium">{{ plugin.latest_version }}</span>
            <n-tag v-if="hasUpdate" type="warning" size="tiny" class="ml-2">
              {{ trans('plugins.update') }}
            </n-tag>
          </div>
        </div>
      </div>

      <n-divider />

      <div class="mb-4">
        <h3 class="font-semibold mb-2">{{ trans('plugins.select_version') }}</h3>
        <n-select
            v-model:value="selectedVersion"
            :options="versionOptions"
            :placeholder="trans('plugins.select_version')"
        />

        <div v-if="selectedVersionData && selectedVersionData.changelog" class="mt-3 p-3 bg-gray-50 dark:bg-gray-800 rounded">
          <h4 class="font-medium mb-1">{{ trans('plugins.changelog') }}</h4>
          <div class="text-sm whitespace-pre-wrap text-gray-600 dark:text-gray-400 max-h-40 overflow-auto">
            {{ selectedVersionData.changelog }}
          </div>
        </div>
      </div>

      <div class="flex gap-2 justify-end">
        <n-button @click="$emit('close')">
          {{ trans('main.close') }}
        </n-button>
        <GButton
            v-if="!plugin.installed"
            color="green"
            @click="$emit('install', selectedVersion)"
        >
          <i class="fa-solid fa-download mr-1"></i>
          {{ trans('plugins.install') }}
        </GButton>
        <GButton
            v-if="plugin.installed && hasUpdate"
            color="blue"
            @click="$emit('update', selectedVersion)"
        >
          <i class="fa-solid fa-sync mr-1"></i>
          {{ trans('plugins.update') }}
        </GButton>
        <GButton
            v-if="plugin.installed"
            color="red"
            @click="$emit('uninstall')"
        >
          <i class="fa-solid fa-trash mr-1"></i>
          {{ trans('plugins.uninstall') }}
        </GButton>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch } from 'vue'
import { trans } from '@/i18n/i18n'
import { Loading } from '@gameap/ui'
import GButton from '@/components/GButton.vue'
import { NTag, NSelect, NDivider, NButton } from 'naive-ui'

const props = defineProps({
  plugin: {
    type: Object,
    default: null
  },
  versions: {
    type: Array,
    default: () => []
  },
  loading: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['install', 'update', 'uninstall', 'close'])

const selectedVersion = ref(null)

const hasUpdate = computed(() => {
  if (!props.plugin?.installed) return false
  return props.plugin.installed_version !== props.plugin.latest_version
})

const versionOptions = computed(() => {
  return props.versions.map(v => ({
    label: v.version,
    value: v.version
  }))
})

const selectedVersionData = computed(() => {
  if (!selectedVersion.value) return null
  return props.versions.find(v => v.version === selectedVersion.value)
})

// Set default selected version
watch(() => props.versions, (versions) => {
  if (versions?.length > 0 && !selectedVersion.value) {
    // Select latest stable version or just the first one
    const stable = versions.find(v => v.is_stable)
    selectedVersion.value = stable ? stable.version : versions[0].version
  }
}, { immediate: true })

watch(() => props.plugin, () => {
  // Reset selected version when plugin changes
  selectedVersion.value = null
})

function renderStars(rating) {
  const fullStars = Math.floor(rating || 0)
  const hasHalf = (rating || 0) - fullStars >= 0.5
  const emptyStars = 5 - fullStars - (hasHalf ? 1 : 0)

  return '★'.repeat(fullStars) + (hasHalf ? '½' : '') + '☆'.repeat(emptyStars)
}

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

function formatDate(dateString) {
  if (!dateString) return ''
  const date = new Date(dateString)
  return date.toLocaleDateString()
}
</script>
