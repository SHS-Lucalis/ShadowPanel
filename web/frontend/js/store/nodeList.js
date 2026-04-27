import { defineStore } from 'pinia'
import axios from '../config/axios'

export const useNodeListStore = defineStore("nodeList",{
    state: () => ({
        nodes: [],
        summary: {},

        autoSetupData: {
            link: '',
            token: '',
            host: '',
            grpc_enabled: false,
            connect_url: '',
            linux_cmd: '',
            windows_cmd: '',
            setup_link: '',
        },

        apiProcesses: 0,
    }),
    getters: {
        loading: (state) => state.apiProcesses > 0,
        summaryById: (state) => {
            const map = new Map()
            const online = state.summary?.onlineNodes || []
            const offline = state.summary?.offlineNodes || []
            for (const n of online) {
                map.set(String(n.id), { ...n, online: true })
            }
            for (const n of offline) {
                map.set(String(n.id), { ...n, online: false })
            }
            return map
        },
    },
    actions: {
        async fetchNodesByFilter(filter) {
            this.apiProcesses++
            try {
                const response = await axios.get('/api/nodes')
                this.nodes = response.data;
            } catch (error) {
                throw error
            } finally {
                this.apiProcesses--
            }
        },
        async fetchNodesSummary() {
            this.apiProcesses++
            try {
                const response = await axios.get('/api/nodes/summary')
                this.summary = response.data;
            } catch (error) {
                throw error
            } finally {
                this.apiProcesses--
            }
        },
        async deleteNode(id) {
            this.apiProcesses++
            try {
                await axios.delete('/api/nodes/'+id)
            } catch (error) {
                throw error
            } finally {
                this.apiProcesses--
            }
        },
        async fetchAutoSetupData() {
            this.apiProcesses++
            try {
                const response = await axios.get('/api/nodes/setup')
                this.autoSetupData = response.data
            } catch (error) {
                throw error
            } finally {
                this.apiProcesses--
            }
        },
    },
})