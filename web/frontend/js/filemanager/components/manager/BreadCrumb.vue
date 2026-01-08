<template>
    <div class="fm-breadcrumb">
        <nav aria-label="breadcrumb" class="flex px-3 py-1.5 rounded text-stone-700 bg-stone-100 dark:bg-stone-800 dark:border-stone-700">
            <ol class="inline-flex items-center space-x-1 md:space-x-2 rtl:space-x-reverse" v-bind:class="[manager === activeManager ? 'active-manager' : 'bg-light']">
                <li class="breadcrumb-item" v-on:click="selectMainDirectory">
                    <span class="badge bg-secondary dark:text-stone-400">
                        <GIcon name="hard-drive" />
                    </span>
                </li>
                <li
                    class="breadcrumb-item text-truncate dark:text-stone-400"
                    v-for="(item, index) in breadcrumb"
                    v-bind:key="index"
                    v-bind:class="[breadcrumb.length === index + 1 ? 'active' : '']"
                    v-on:click="handleSelectDirectory(index)"
                >
                    <div class="flex items-center">
                        <span class="mx-2 text-stone-400">/</span>
                        <span>{{ item }}</span>
                    </div>
                </li>
            </ol>
        </nav>
    </div>
</template>

<script setup>
import { computed } from 'vue'
import { GIcon } from '@gameap/ui'
import { useFileManagerStore } from '../../stores/useFileManagerStore.js'
import { useManager } from '../../composables/useManager.js'

const props = defineProps({
    manager: { type: String, required: true },
})

const fm = useFileManagerStore()
const { selectedDisk, selectedDirectory, breadcrumb, selectDirectory } = useManager(props.manager)

const activeManager = computed(() => fm.activeManager)

function handleSelectDirectory(index) {
    const path = breadcrumb.value.slice(0, index + 1).join('/')
    if (path !== selectedDirectory.value) {
        selectDirectory(path, true)
    }
}

function selectMainDirectory() {
    if (selectedDirectory.value) {
        selectDirectory(null, true)
    }
}
</script>

<style lang="scss">
.fm-breadcrumb {
    @apply mb-1;

    .breadcrumb-item {
        @apply inline-flex items-center;
    }

    .breadcrumb-item:not(.active):hover {
        cursor: pointer;
        font-weight: normal;
        color: #6c757d;
    }
}
</style>
