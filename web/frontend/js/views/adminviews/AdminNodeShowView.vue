<template>
  <GBreadcrumbs :items="breadcrumbs"></GBreadcrumbs>

  <GButton color="blue" size="middle" class="mb-5 mr-1" :route="{name: 'admin.nodes.edit', params: {id: route.params.id}}">
    <i class="fa-solid fa-edit mr-0.5"></i>
    <span>{{ trans('main.edit')}}</span>
  </GButton>

  <GButton color="orange" size="middle" class="mb-5 mr-1" :disabled="downloading" @click="downloadLogs">
    <i class="fa-solid fa-download mr-0.5"></i>
    <span>{{ trans('dedicated_servers.download_logs')}}</span>
  </GButton>

  <GButton color="green" size="middle" class="mb-5 mr-1" :disabled="downloading" @click="downloadCertificates">
    <i class="fa-solid fa-download mr-0.5"></i>
    <span>{{ trans('dedicated_servers.download_certificates')}}</span>
  </GButton>

  <div v-if="downloading" class="mb-5">
    <Progressbar :progress="downloadProgress" />
  </div>

  <n-card
      size="small"
      class="mb-3"
      header-class="g-card-header"
      :segmented="{
                            content: true,
                            footer: 'soft'
                          }"
  >
    <Loading v-if="loading"></Loading>
    <n-table v-else :bordered="false" :single-line="true">
      <tbody>
      <tr>
        <td><strong>ID:</strong></td>
        <td>{{ daemonInfo.id }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.name') }}:</strong></td>
        <td>{{ daemonInfo.name }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_api_key') }}:</strong></td>
        <td>{{ daemonInfo.api_key }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_version') }}:</strong></td>
        <td>{{
            daemonInfo.version && daemonInfo.version.version
                ? daemonInfo.version.version + ' (' + daemonInfo.version.compile_date + ')'
                : trans('dedicated_servers.gdaemon_empty_info')
          }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_uptime') }}:</strong></td>
        <td>{{
            daemonInfo.base_info && daemonInfo.base_info.uptime
                ? daemonInfo.base_info.uptime
                : trans('dedicated_servers.gdaemon_empty_info')
          }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_online_servers_count') }}:</strong></td>
        <td>{{
            daemonInfo.base_info && daemonInfo.base_info.online_servers_count
                ? daemonInfo.base_info.online_servers_count
                : trans('dedicated_servers.gdaemon_empty_info')
          }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_working_tasks_count') }}:</strong></td>
        <td>{{
            daemonInfo.base_info && daemonInfo.base_info.working_tasks_count
                ? daemonInfo.base_info.working_tasks_count
                : trans('dedicated_servers.gdaemon_empty_info')
          }}</td>
      </tr>
      <tr>
        <td><strong>{{ trans('dedicated_servers.gdaemon_waiting_tasks_count') }}:</strong></td>
        <td>{{
            daemonInfo.base_info && daemonInfo.base_info.waiting_tasks_count
                ? daemonInfo.base_info.waiting_tasks_count
                : trans('dedicated_servers.gdaemon_empty_info')
          }}</td>
      </tr>
      </tbody>
    </n-table>
  </n-card>
</template>

<script setup>
import { GBreadcrumbs, Loading, Progressbar } from "@gameap/ui"
import {computed, onMounted, ref} from "vue"
import {trans} from "../../i18n/i18n"
import {
  NCard,
  NTable,
} from "naive-ui"
import {useNodeStore} from "../../store/node"
import {errorNotification} from "../../parts/dialogs"
import {storeToRefs} from "pinia"
import {useRoute} from "vue-router"
import GButton from "../../components/GButton.vue"
import axios from "../../config/axios"

const route = useRoute()

const nodeStore = useNodeStore()

const { daemonInfo, loading } = storeToRefs(nodeStore)

const downloading = ref(false)
const downloadProgress = ref(0)

async function downloadFile(url, filename) {
  downloading.value = true
  downloadProgress.value = 0

  try {
    const response = await axios.get(url, {
      responseType: 'blob',
      onDownloadProgress: (progressEvent) => {
        if (progressEvent.total) {
          downloadProgress.value = Math.round((progressEvent.loaded * 100) / progressEvent.total)
        }
      }
    })

    const blob = new Blob([response.data])
    const link = document.createElement('a')
    link.href = window.URL.createObjectURL(blob)
    link.setAttribute('download', filename)
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(link.href)
  } catch (error) {
    errorNotification(error)
  } finally {
    downloading.value = false
    downloadProgress.value = 0
  }
}

function downloadLogs() {
  downloadFile(`/api/dedicated_servers/${route.params.id}/logs.zip`, 'logs.zip')
}

function downloadCertificates() {
  downloadFile('/api/dedicated_servers/certificates.zip', 'certificates.zip')
}

const breadcrumbs = computed(() => {
  let result = [
    {route:'/', text:'GameAP', icon: 'gicon gicon-gameap'},
    {route:{name: 'admin.nodes.index'}, text:trans('dedicated_servers.dedicated_servers')},
  ]

  if (daemonInfo.value.name) {
    result.push({text: daemonInfo.value.name})
  }

  return result
})

onMounted(() => {
  nodeStore.setNodeId(route.params.id)
  nodeStore.fetchDaemonInfo().catch((error) => {
    errorNotification(error)
  })
})



</script>