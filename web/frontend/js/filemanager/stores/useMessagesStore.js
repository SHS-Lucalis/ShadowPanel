import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export const useMessagesStore = defineStore('fm-messages', () => {
    const actionResult = ref({
        status: null,
        message: null,
    })
    const actionProgress = ref(0)
    const progressLabel = ref('')
    const loadingCount = ref(0)
    const errors = ref([])
    const uploadProgress = ref({ files: [] })

    // Getters
    const loading = computed(() => loadingCount.value > 0)

    // Actions
    function setActionResult({ status, message }) {
        actionResult.value.status = status
        actionResult.value.message = message
    }

    function clearActionResult() {
        actionResult.value.status = null
        actionResult.value.message = null
    }

    function setProgress(progress, label) {
        actionProgress.value = progress
        if (label !== undefined) {
            progressLabel.value = label
        }
    }

    function clearProgress() {
        actionProgress.value = 0
        progressLabel.value = ''
    }

    function initUploadProgress(files) {
        uploadProgress.value = {
            files: files.map((f) => ({
                name: f.name,
                size: f.size,
                phase: 'pending',
                loaded: 0,
                error: null,
            })),
        }
    }

    function setFilePhase({ index, phase }) {
        const file = uploadProgress.value.files[index]
        if (file) file.phase = phase
    }

    function setFileProgress({ index, loaded }) {
        const file = uploadProgress.value.files[index]
        if (file) file.loaded = loaded
    }

    function setFileError({ index, error }) {
        const file = uploadProgress.value.files[index]
        if (file) {
            file.phase = 'error'
            file.error = error
        }
    }

    function clearUploadProgress() {
        uploadProgress.value = { files: [] }
    }

    function addLoading() {
        loadingCount.value += 1
    }

    function subtractLoading() {
        loadingCount.value -= 1
    }

    function clearLoading() {
        loadingCount.value = 0
    }

    function setError(error) {
        errors.value.push(error)
    }

    function clearErrors() {
        errors.value = []
    }

    return {
        // State
        actionResult,
        actionProgress,
        progressLabel,
        loadingCount,
        errors,
        uploadProgress,
        // Getters
        loading,
        // Actions
        setActionResult,
        clearActionResult,
        setProgress,
        clearProgress,
        addLoading,
        subtractLoading,
        clearLoading,
        setError,
        clearErrors,
        initUploadProgress,
        setFilePhase,
        setFileProgress,
        setFileError,
        clearUploadProgress,
    }
})
