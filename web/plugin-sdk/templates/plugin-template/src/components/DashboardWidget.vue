<template>
    <GCard>
        <template #header>
            <div class="flex items-center">
                <GIcon name="plugin" class="mr-2" />
                <strong>{{ trans('widget_title') }}</strong>
            </div>
        </template>

        <p class="text-stone-600 dark:text-stone-400">
            {{ trans('widget_description') }}
        </p>
        <p v-if="isAdmin" class="mt-2 text-sm text-green-600 dark:text-green-400">
            <GIcon name="check" class="mr-1" />
            {{ trans('admin_notice') }}
        </p>
    </GCard>
</template>

<script setup lang="ts">
import { providePluginTrans, GCard, GIcon } from '@gameap/plugin-sdk';
import type { DashboardWidgetProps } from '@gameap/plugin-sdk';

const props = defineProps<DashboardWidgetProps>();

// Use providePluginTrans for slot components (tabs, widgets).
// This provides translation context to this component and all its children.
const { trans } = providePluginTrans(props.pluginId);

const isAdmin = props.isAdmin;
</script>
