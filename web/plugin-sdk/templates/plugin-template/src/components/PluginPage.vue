<template>
    <div class="p-4">
        <GBreadcrumbs :items="breadcrumbs" class="mb-4" />

        <h1 class="text-xl font-bold mb-4 text-stone-900 dark:text-white">
            {{ trans('title') }}
        </h1>
        <p class="text-stone-600 dark:text-stone-400 mb-4">
            {{ trans('welcome') }}
        </p>

        <GCard :title="trans('user_info')">
            <GTable>
                <tbody>
                    <tr>
                        <td class="font-semibold">{{ trans('logged_in_as') }}</td>
                        <td>
                            <GIcon name="user" class="mr-2" />
                            {{ user.login }}
                        </td>
                    </tr>
                    <tr>
                        <td class="font-semibold">{{ trans('admin') }}</td>
                        <td>
                            <GStatusBadge
                                :status="user.isAdmin ? 'success' : 'waiting'"
                                :text="user.isAdmin ? trans('yes') : trans('no')"
                            />
                        </td>
                    </tr>
                </tbody>
            </GTable>
        </GCard>
    </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import {
    useCurrentUser,
    usePluginTrans,
    GBreadcrumbs,
    GCard,
    GTable,
    GIcon,
    GStatusBadge,
} from '@gameap/plugin-sdk';

const user = useCurrentUser();
const { trans } = usePluginTrans();

const breadcrumbs = computed(() => [
    { text: 'GameAP', route: '/', icon: 'gicon gicon-gameap' },
    { text: trans('title') },
]);
</script>
