<template>
    <component :is="routeComponent" v-bind="$attrs" />
</template>

<script setup>
import { providePluginContext } from '@/plugins'

defineProps({
    routeComponent: {
        type: [Object, Function],
        required: true
    }
})

providePluginContext()
</script>
