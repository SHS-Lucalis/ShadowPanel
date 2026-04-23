<template>
    <div>
        <div id="serverForm" class="mb-3">
          <label for="server-id" class="control-label">{{ trans('labels.game_server') }}</label>
          <select id="server-id" v-bind:name="serverIdFieldName" class="form-select" v-model="selectedServerId">
              <option v-for="server in serversList" v-bind:value="server.id">{{ server.name }}&nbsp;&nbsp;&nbsp;&nbsp;{{ server.server_ip }}:{{ server.server_port }}</option>
          </select>
        </div>
    </div>
</template>

<script setup>
import { computed, watch, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
import { useNodeStore } from '@/store/node'
import { useServerStore } from '@/store/server'
import { useServerListStore } from '@/store/serverList'
import { trans } from '@/i18n/i18n'

const props = defineProps({
    serverIdFieldName: {
        type: String,
        default: 'game_server_id',
    }
})

const nodeStore = useNodeStore()
const serverStore = useServerStore()
const serverListStore = useServerListStore()
const { nodeId: dsId } = storeToRefs(nodeStore)
const { servers: serversList } = storeToRefs(serverListStore)

const selectedServerId = computed({
    get() {
        return serverStore.serverId
    },
    set(serverId) {
        serverStore.setServerId(serverId)
    }
})

onMounted(() => {
    if (!selectedServerId.value) {
        selectedServerId.value = -1
    }
})

watch(dsId, () => {
    serverListStore.fetchServersByNode(dsId.value, {perPage: 1000})
})
</script>
