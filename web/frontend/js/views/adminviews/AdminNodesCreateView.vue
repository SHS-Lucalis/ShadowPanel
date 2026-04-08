<template>
  <GBreadcrumbs :items="breadcrumbs"></GBreadcrumbs>

  <CreateNodeModal/>
</template>

<script setup>
import { GBreadcrumbs } from "@gameap/ui"
import {computed} from "vue"
import {trans} from "@/i18n/i18n"
import CreateNodeModal from "@/components/blocks/CreateNodeModal.vue";

const breadcrumbs = computed(() => {
  return [
    {route:'/', text:'GameAP', icon: 'gicon gicon-gameap'},
    {route:{name: 'admin.nodes.index'}, text:trans('dedicated_servers.dedicated_servers')},
    {text: trans('dedicated_servers.create')},
  ]
})
</script>
