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
        <i v-else class="fa-solid fa-puzzle-piece text-5xl text-stone-400"></i>

        <div class="flex-1">
          <div class="flex items-center gap-2 flex-wrap">
            <h2 class="text-xl font-bold whitespace-nowrap">{{ plugin.name }}</h2>
            <span v-if="plugin.installed" class="hidden md:inline px-2 py-0.5 text-xs font-medium rounded-full bg-lime-100 text-lime-800 dark:bg-lime-900 dark:text-lime-300 whitespace-nowrap">
              {{ trans('plugins.already_installed') }}
            </span>
          </div>

          <div v-if="plugin.summary" class="mb-4 text-stone-600 dark:text-stone-400">
            {{ plugin.summary }}
          </div>

          <div v-if="plugin.labels?.length > 0" class="hidden md:flex gap-1 mt-1">
            <span
                v-for="label in plugin.labels"
                :key="label.id"
                class="px-2 py-0.5 text-xs font-medium rounded-full"
                :style="label.color ? { backgroundColor: label.color, color: '#fff' } : {}"
                :class="!label.color ? 'bg-stone-100 text-stone-800 dark:bg-stone-700 dark:text-stone-300' : ''"
            >
              {{ label.name }}
            </span>
          </div>

          <div class="flex items-center gap-2 mt-2">
            <span class="text-orange-500 text-lg">{{ renderStars(plugin.rating_avg) }}</span>
            <a v-if="plugin.url" :href="plugin.url + '/reviews'" target="_blank" class="text-stone-500 hover:text-blue-500 hover:underline">
              ({{ plugin.rating_count || 0 }} {{ trans('plugins.reviews') }})
            </a>
            <span v-else class="text-stone-500">({{ plugin.rating_count || 0 }} {{ trans('plugins.reviews') }})</span>
          </div>
        </div>

        <div class="flex flex-col md:flex-row items-end md:items-center gap-2">
          <n-select
              v-if="!plugin.installed || hasUpdate"
              v-model:value="selectedVersion"
              :options="versionOptions"
              :placeholder="trans('plugins.select_version')"
              style="width: 120px;"
              :size="isSmallScreen ? 'small' : 'large'"
          />
          <GButton
              v-if="!plugin.installed"
              color="green"
              :size="isSmallScreen ? 'small' : 'medium'"
              @click="$emit('install', selectedVersion)"
          >
            <i class="fa-solid fa-download mr-1"></i>
            {{ trans('plugins.install') }}
          </GButton>
          <GButton
              v-if="plugin.installed && hasUpdate"
              color="blue"
              :size="isSmallScreen ? 'small' : 'medium'"
              @click="$emit('update', selectedVersion)"
          >
            <i class="fa-solid fa-sync mr-1"></i>
            {{ trans('plugins.update') }}
          </GButton>
          <GButton
              v-if="plugin.installed"
              color="red"
              :size="isSmallScreen ? 'small' : 'medium'"
              @click="$emit('uninstall')"
          >
            <i class="fa-solid fa-xmark mr-1"></i>
            {{ trans('plugins.uninstall') }}
          </GButton>
        </div>
      </div>

      <div class="flex justify-around py-4 border-t border-b border-stone-200 dark:border-stone-700 mb-4 divide-x divide-stone-100 dark:divide-stone-700">
        <div v-if="plugin.author" class="flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-user text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.author.username }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.author') }}</span>
        </div>

        <div v-if="plugin.category" class="flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-folder text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.category.name }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.category') }}</span>
        </div>

        <div v-if="plugin.license" class="flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-scale-balanced text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.license }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.license') }}</span>
        </div>

        <div v-if="plugin.download_count !== undefined" class="hidden md:flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-download text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ formatNumber(plugin.download_count) }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.downloads') }}</span>
        </div>

        <div v-if="plugin.published_at" class="hidden md:flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-calendar text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ formatDate(plugin.published_at) }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.published_at') }}</span>
        </div>

        <div v-if="plugin.min_gameap_version" class="flex flex-col items-center text-center px-4">
          <i class="fa-solid fa-gamepad text-xl text-stone-500 dark:text-stone-400 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.min_gameap_version }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.min_gameap_version') }}</span>
        </div>

        <a v-if="plugin.url" :href="plugin.url" target="_blank" class="flex flex-col items-center text-center px-4 hover:text-blue-500 transition-colors">
          <i class="fa-solid fa-external-link text-xl text-blue-500 mb-1"></i>
          <span class="text-sm font-medium text-blue-500">{{ trans('plugins.plugin_page') }}</span>
        </a>
      </div>

      <div v-if="plugin.description" class="mb-4">
        <h3 class="font-semibold mb-2">{{ trans('plugins.description') }}</h3>
        <div class="text-sm whitespace-pre-wrap">{{ plugin.description }}</div>
      </div>

      <div v-if="plugin.installed" class="flex justify-center gap-6 mb-4 p-3 bg-stone-100 dark:bg-stone-800 rounded-lg">
        <div class="flex flex-col items-center text-center">
          <i class="fa-solid fa-box text-xl text-lime-500 mb-1"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.installed_version }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.installed_version') }}</span>
        </div>
        <div class="flex flex-col items-center text-center">
          <i class="fa-solid fa-arrow-up text-xl mb-1" :class="hasUpdate ? 'text-orange-500' : 'text-stone-400'"></i>
          <span class="text-sm font-medium whitespace-nowrap">{{ plugin.latest_version }}</span>
          <span class="text-xs text-stone-500">{{ trans('plugins.latest_version') }}</span>
        </div>
        <span v-if="hasUpdate" class="self-center px-2 py-0.5 text-xs font-medium rounded-full bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-300">
          {{ trans('plugins.update') }}
        </span>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import { trans } from '@/i18n/i18n'
import { Loading } from '@gameap/ui'
import GButton from '@/components/GButton.vue'
import { NSelect } from 'naive-ui'

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

const emit = defineEmits(['install', 'update', 'uninstall'])

const selectedVersion = ref(null)
const isSmallScreen = ref(window.innerWidth < 768)

const handleResize = () => {
  isSmallScreen.value = window.innerWidth < 768
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
})

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

// Set default selected version to latest
watch([() => props.versions, () => props.plugin], ([versions, plugin]) => {
  if (plugin?.latest_version) {
    selectedVersion.value = plugin.latest_version
  } else if (versions?.length > 0) {
    selectedVersion.value = versions[0].version
  }
}, { immediate: true })

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
