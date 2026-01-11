<template>
    <div class="w-full mt-5 p-3 border border-stone-200 bg-stone-50 rounded-lg sm:p-4 dark:bg-stone-800 dark:border-stone-700">
        <strong class="text-stone-900 dark:text-white">
            <i class="fas fa-puzzle-piece mr-1"></i>
            {{ trans('widget_title') }}
        </strong>
        <p class="mt-2 text-stone-600 dark:text-stone-400">
            {{ trans('widget_description') }}
        </p>
        <p v-if="isAdmin" class="mt-2 text-sm text-green-600 dark:text-green-400">
            {{ trans('admin_notice') }}
        </p>
    </div>
</template>

<script setup lang="ts">
import { providePluginTrans } from '@gameap/plugin-sdk';
import type { DashboardWidgetProps } from '@gameap/plugin-sdk';

const props = defineProps<DashboardWidgetProps>();

// Use providePluginTrans for slot components (tabs, widgets).
// This provides translation context to this component and all its children.
const { trans } = providePluginTrans(props.pluginId);

const isAdmin = props.isAdmin;
</script>
