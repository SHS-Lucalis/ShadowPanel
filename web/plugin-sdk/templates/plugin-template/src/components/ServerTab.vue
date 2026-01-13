<template>
    <div class="p-4">
        <GCard :title="trans('tab_title')">
            <GTable>
                <tbody>
                    <tr>
                        <td class="font-semibold">{{ trans('server_id') }}</td>
                        <td>{{ serverId }}</td>
                    </tr>
                    <tr>
                        <td class="font-semibold">{{ trans('server_name') }}</td>
                        <td>{{ server?.name || 'Loading...' }}</td>
                    </tr>
                    <tr>
                        <td class="font-semibold">{{ trans('game') }}</td>
                        <td>
                            <GGameIcon :game="server?.game_id || ''" class="mr-2" />
                            {{ server?.game_id || 'N/A' }}
                        </td>
                    </tr>
                    <tr>
                        <td class="font-semibold">{{ trans('address') }}</td>
                        <td>{{ serverAddress }}</td>
                    </tr>
                    <tr>
                        <td class="font-semibold">{{ trans('status') }}</td>
                        <td>
                            <GStatusBadge :status="serverStatus" :text="statusText" />
                        </td>
                    </tr>
                </tbody>
            </GTable>

            <p class="mt-4 text-stone-600 dark:text-stone-400">
                {{ trans('tab_description') }}
            </p>
        </GCard>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { providePluginTrans, GCard, GTable, GGameIcon, GStatusBadge } from '@gameap/plugin-sdk';
import type { ServerTabProps } from '@gameap/plugin-sdk';

const props = defineProps<ServerTabProps>();

// Use providePluginTrans for slot components (tabs, widgets).
// This provides translation context to this component and all its children.
// For plugin routes/pages, use usePluginTrans() instead.
const { trans } = providePluginTrans(props.pluginId);

const serverAddress = computed(() => {
    if (!props.server) return 'N/A';
    return `${props.server.ip}:${props.server.port}`;
});

const serverStatus = computed(() => {
    if (!props.server) return 'waiting';
    if (props.server.process_active) return 'success';
    if (!props.server.enabled) return 'error';
    return 'canceled';
});

const statusText = computed(() => {
    if (!props.server) return trans('unknown');
    if (props.server.process_active) return trans('running');
    if (!props.server.enabled) return trans('disabled');
    return trans('stopped');
});
</script>
