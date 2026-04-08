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