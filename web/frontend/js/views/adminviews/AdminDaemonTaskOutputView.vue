<template>
  <GBreadcrumbs :items="breadcrumbs"></GBreadcrumbs>

  <Loading v-if="loading"></Loading>
  <div :class="loading ? 'hidden' : ''">
    <GTable>
      <tbody>
      <tr>
        <td><strong>{{ trans('gdaemon_tasks.task') }}:</strong></td>
        <td>{{ trans('gdaemon_tasks.'+task.task) }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('gdaemon_tasks.status') }}:</strong></td>
        <td><GStatusBadge v-if="task.status" :status="task.status" /></td>
      </tr>
      <tr>
        <td><strong>{{ trans('gdaemon_tasks.created') }}:</strong></td>
        <td>{{ task.created_at }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('gdaemon_tasks.created') }}:</strong></td>
        <td>{{ task.updated_at }}</td>
      </tr>
      </tbody>
    </GTable>

    <div class="w-full mt-3">
      <div class="coding inverse-toggle px-5 pt-4 shadow-lg text-stone-100 text-sm font-mono subpixel-antialiased
              bg-stone-800 dark:bg-stone-900 pb-6 pt-4 rounded-lg leading-normal overflow-hidden">
        <div class="top mb-2 flex">
          <div class="h-3 w-3 bg-red-500 rounded-full"></div>
          <div class="ml-2 h-3 w-3 bg-orange-300 rounded-full"></div>
          <div class="ml-2 h-3 w-3 bg-green-500 rounded-full"></div>
        </div>
        <div class="whitespace-pre-wrap mt-4 flex ">
          {{ output }}
        </div>
      </div>
    </div>
  </div>

</template>

<script setup>
import { GBreadcrumbs, GStatusBadge, Loading, GTable } from "@gameap/ui"
import {computed, onMounted, onUnmounted, watch} from "vue"
import {trans} from "../../i18n/i18n"
import {errorNotification, notification} from "../../parts/dialogs"
import {useRoute} from "vue-router"
import {storeToRefs} from "pinia"
import {useDaemonTaskStore} from "../../store/daemonTask";
import { replace } from "lodash-es";

const daemonTaskStore = useDaemonTaskStore()
const route = useRoute()

const POLL_INTERVAL = 3000
let pollIntervalId = null

function shouldPoll(status) {
  return status === 'waiting' || status === 'working'
}

function startPolling() {
  if (pollIntervalId) return

  pollIntervalId = setInterval(() => {
    daemonTaskStore.fetchTaskOutput(true).catch((error) => {
      errorNotification(error)
      stopPolling()
    })
  }, POLL_INTERVAL)
}

function stopPolling() {
  if (pollIntervalId) {
    clearInterval(pollIntervalId)
    pollIntervalId = null
  }
}

const breadcrumbs = computed(() => {
  let result = [
    {'route':'/', 'text':'GameAP', 'icon': 'gicon gicon-gameap'},
    {'route':{name: 'admin.gdaemon_tasks.index'}, 'text':trans('gdaemon_tasks.gdaemon_tasks')},
  ]

  if (task.value.id) {
    result.push({
      route: {name: 'admin.gdaemon_tasks.output', params: {id: task.value.id}},
      text:trans('gdaemon_tasks.'+task.value.task),
    })
  }

  return result
})

const {loading, task} = storeToRefs(daemonTaskStore)

onMounted(() => {
  daemonTaskStore.setTaskId(route.params.id)
  daemonTaskStore.fetchTaskOutput()
    .then(() => {
      if (shouldPoll(task.value.status)) {
        startPolling()
      }
    })
    .catch((error) => {
      errorNotification(error)
    })
})

watch(() => task.value.status, (newStatus) => {
  if (!shouldPoll(newStatus)) {
    stopPolling()
  }
})

onUnmounted(() => {
  stopPolling()
})

const output = computed(() => {
  if (!task.value.output) {
    return ''
  }

  return replace(task.value.output, /(\r\n|\n|\r)/gm, "\n")
})
</script>