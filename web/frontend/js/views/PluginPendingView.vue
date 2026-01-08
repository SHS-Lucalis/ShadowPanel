<template>
    <div v-if="loading" class="flex items-center justify-center h-64">
        <div class="text-center">
            <GIcon name="loading" class="animate-spin text-4xl text-stone-400" />
            <p class="mt-4 text-stone-600 dark:text-stone-400">Loading plugin...</p>
        </div>
    </div>
    <Error404View v-else />
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { GIcon } from '@gameap/ui'
import { usePluginsStore } from '../store/plugins'
import Error404View from './errors/Error404View.vue'

const route = useRoute()
const router = useRouter()
const pluginsStore = usePluginsStore()

const loading = ref(!pluginsStore.initialized)

function tryNavigateToPluginRoute() {
    const pluginId = route.params.pluginId
    const hasPluginRoutes = pluginsStore.registeredRoutes.some(
        name => name.startsWith(`plugin.${pluginId}.`)
    )

    if (hasPluginRoutes) {
        router.removeRoute('plugin.pending')
        router.replace(route.fullPath)
    } else {
        loading.value = false
    }
}

watch(() => pluginsStore.initialized, (initialized) => {
    if (initialized) {
        tryNavigateToPluginRoute()
    }
})

onMounted(() => {
    if (pluginsStore.initialized) {
        tryNavigateToPluginRoute()
    }
})
</script>
