import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from '../config/axios'
import { getCurrentLanguage } from '../i18n/i18n'

export const usePluginStoreStore = defineStore('pluginStore', () => {
    // State
    const plugins = ref([])
    const currentPage = ref(1)
    const total = ref(0)
    const lastPage = ref(1)
    const perPage = ref(15)

    const categories = ref([])
    const labels = ref([])

    const currentPlugin = ref(null)
    const currentPluginVersions = ref([])
    const versionsCurrentPage = ref(1)
    const versionsLastPage = ref(1)

    const loadedPlugins = ref([])
    const uploadResult = ref(null)
    const uploadFile = ref(null)

    const apiProcesses = ref(0)

    // Getters
    const loading = computed(() => apiProcesses.value > 0)

    const installedPlugins = computed(() =>
        plugins.value.filter(p => p.installed === true)
    )

    const updatablePlugins = computed(() =>
        plugins.value.filter(p =>
            p.installed === true &&
            p.installed_version &&
            p.latest_version &&
            p.installed_version !== p.latest_version
        )
    )

    const categoryOptions = computed(() =>
        categories.value.map(c => ({
            label: c.name,
            value: c.slug
        }))
    )

    const labelOptions = computed(() =>
        labels.value.map(l => ({
            label: l.name,
            value: l.slug,
            color: l.color
        }))
    )

    // Helper to add lang param
    const withLang = (params = {}) => ({
        ...params,
        lang: getCurrentLanguage()
    })

    // Actions
    async function fetchCategories() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/plugin-store/categories', {
                params: withLang()
            })
            categories.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchLabels() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/plugin-store/labels', {
                params: withLang()
            })
            labels.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchPlugins(filter = {}) {
        apiProcesses.value++
        try {
            const params = withLang({
                page: filter.page || currentPage.value,
                per_page: filter.perPage || perPage.value,
            })

            if (filter.category) params.category = filter.category
            if (filter.label) params.label = filter.label
            if (filter.search) params.search = filter.search

            const response = await axios.get('/api/plugin-store/plugins', { params })

            plugins.value = response.data.data
            currentPage.value = response.data.current_page
            total.value = response.data.total
            lastPage.value = response.data.last_page
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchPluginDetails(id) {
        apiProcesses.value++
        try {
            const response = await axios.get(`/api/plugin-store/plugins/${id}`, {
                params: withLang()
            })
            currentPlugin.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchPluginVersions(id, page = 1) {
        apiProcesses.value++
        try {
            const response = await axios.get(`/api/plugin-store/plugins/${id}/versions`, {
                params: withLang({
                    page: page,
                    per_page: 20
                })
            })
            currentPluginVersions.value = response.data.data
            versionsCurrentPage.value = response.data.current_page
            versionsLastPage.value = response.data.last_page
        } finally {
            apiProcesses.value--
        }
    }

    async function installPlugin(id, version = null) {
        apiProcesses.value++
        try {
            const body = version ? { version } : {}
            const response = await axios.post(`/api/plugin-store/plugins/${id}/install`, body, {
                params: withLang()
            })
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function updatePlugin(id, version = null) {
        apiProcesses.value++
        try {
            const body = version ? { version } : {}
            const response = await axios.post(`/api/plugin-store/plugins/${id}/update`, body, {
                params: withLang()
            })
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function uninstallPlugin(id) {
        apiProcesses.value++
        try {
            const response = await axios.delete(`/api/admin/plugins/${id}`, {
                params: withLang()
            })
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    function clearCurrentPlugin() {
        currentPlugin.value = null
        currentPluginVersions.value = []
        versionsCurrentPage.value = 1
        versionsLastPage.value = 1
    }

    async function fetchLoadedPlugins() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/admin/plugins/loaded')
            loadedPlugins.value = response.data.data
        } finally {
            apiProcesses.value--
        }
    }

    async function dryRunUpload(file) {
        apiProcesses.value++
        try {
            const formData = new FormData()
            formData.append('file', file)
            const response = await axios.post('/api/admin/plugins/upload/dry-run', formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            })
            uploadResult.value = response.data
            uploadFile.value = file
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function installFromFile(file) {
        apiProcesses.value++
        try {
            const formData = new FormData()
            formData.append('file', file)
            const response = await axios.post('/api/admin/plugins/upload/install', formData, {
                headers: { 'Content-Type': 'multipart/form-data' }
            })
            uploadResult.value = null
            uploadFile.value = null
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    function clearUpload() {
        uploadResult.value = null
        uploadFile.value = null
    }

    return {
        // State
        plugins,
        currentPage,
        total,
        lastPage,
        perPage,
        categories,
        labels,
        currentPlugin,
        currentPluginVersions,
        versionsCurrentPage,
        versionsLastPage,
        loadedPlugins,
        uploadResult,
        uploadFile,
        apiProcesses,

        // Getters
        loading,
        installedPlugins,
        updatablePlugins,
        categoryOptions,
        labelOptions,

        // Actions
        fetchCategories,
        fetchLabels,
        fetchPlugins,
        fetchPluginDetails,
        fetchPluginVersions,
        installPlugin,
        updatePlugin,
        uninstallPlugin,
        clearCurrentPlugin,
        fetchLoadedPlugins,
        dryRunUpload,
        installFromFile,
        clearUpload,
    }
})
