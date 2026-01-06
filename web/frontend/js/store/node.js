import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from '../config/axios'

export const useNodeStore = defineStore('node', () => {
    // State
    const nodeId = ref(0)
    const node = ref({})
    const daemonInfo = ref({})
    const apiProcesses = ref(0)

    // From legacy dedicatedServers.js
    const ipList = ref([])
    const busyPorts = ref([])

    // Getters
    const loading = computed(() => apiProcesses.value > 0)

    // Actions
    function setNodeId(id) {
        nodeId.value = id
    }

    function resetNodeId() {
        nodeId.value = 0
        ipList.value = []
        busyPorts.value = []
    }

    async function fetchNode() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/dedicated_servers/' + nodeId.value)
            node.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchDaemonInfo() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/dedicated_servers/' + nodeId.value + '/daemon')
            daemonInfo.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function saveNode(nodeData) {
        apiProcesses.value++
        try {
            await axios.put('/api/dedicated_servers/' + nodeId.value, nodeData)
        } finally {
            apiProcesses.value--
        }
    }

    // From legacy dedicatedServers.js
    async function fetchIpList() {
        if (!nodeId.value || nodeId.value <= 0) {
            return
        }

        apiProcesses.value++
        try {
            const response = await axios.get('/api/dedicated_servers/' + nodeId.value + '/ip_list')
            ipList.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchBusyPorts(callback = null) {
        if (!nodeId.value || nodeId.value <= 0) {
            return
        }

        apiProcesses.value++
        try {
            const response = await axios.get('/api/dedicated_servers/' + nodeId.value + '/busy_ports')
            busyPorts.value = response.data

            if (typeof callback === 'function') {
                callback()
            }
        } finally {
            apiProcesses.value--
        }
    }

    return {
        // State
        nodeId,
        node,
        daemonInfo,
        apiProcesses,
        ipList,
        busyPorts,

        // Getters
        loading,

        // Actions
        setNodeId,
        resetNodeId,
        fetchNode,
        fetchDaemonInfo,
        saveNode,
        fetchIpList,
        fetchBusyPorts,
    }
})
