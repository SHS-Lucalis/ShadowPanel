<template>
    <div v-if="visible" class="fm-progress-block">
        <div class="flex items-center gap-3 px-3 py-2">
            <GIcon name="download" class="text-sky-500 shrink-0" />
            <div class="flex-1 min-w-0">
                <div class="flex items-center justify-between mb-1">
                    <span class="text-xs truncate" :title="label">{{ label }}</span>
                    <span class="text-xs text-stone-500 shrink-0 ml-2">{{ progressBar }}%</span>
                </div>
                <n-progress
                    type="line"
                    :percentage="progressBar"
                    :show-indicator="false"
                    :height="6"
                    :border-radius="3"
                    processing
                />
            </div>
        </div>
    </div>
</template>

<script setup>
import { computed } from 'vue'
import { GIcon } from '@gameap/ui'
import { useMessagesStore } from '../../stores/useMessagesStore.js'

const messages = useMessagesStore()

const progressBar = computed(() => messages.actionProgress)
const label = computed(() => messages.progressLabel)
const visible = computed(() => progressBar.value > 0 || label.value)
</script>

<style lang="scss">
.fm-progress-block {
    @apply border-t dark:border-stone-700 bg-stone-50 dark:bg-stone-800/50;
    flex: 0 0 auto;
}
</style>
