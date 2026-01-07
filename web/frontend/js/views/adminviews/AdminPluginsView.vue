<template>
  <GBreadcrumbs :items="breadcrumbs"></GBreadcrumbs>
</template>

<script setup>
import {GBreadcrumbs} from "@gameap/ui";
import {computed} from "vue";
import {trans} from "@/i18n/i18n";

const breadcrumbs = computed(() => {
  return [
    {'route':'/', 'text':'GameAP', 'icon': 'gicon gicon-gameap'},
    {'route':{name: 'admin.plugins.index'}, 'text':trans('plugins.plugins')},
  ]
})
</script>