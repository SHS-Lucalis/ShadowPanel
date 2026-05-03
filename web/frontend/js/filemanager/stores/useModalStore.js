import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useModalStore = defineStore('fm-modal', () => {
    const showModal = ref(false)
    const modalName = ref(null)
    const pluginEditorState = ref(null)
    const payload = ref(null)

    function setModalState({ show, modalName: name, payload: nextPayload = null }) {
        showModal.value = show
        modalName.value = name
        payload.value = nextPayload
    }

    function openPluginEditor({ pluginId, editor, file }) {
        pluginEditorState.value = { pluginId, editor, file }
        showModal.value = true
        modalName.value = 'PluginEditorModal'
    }

    function consumePayload() {
        const value = payload.value
        payload.value = null

        return value
    }

    function clearModal() {
        showModal.value = false
        modalName.value = null
        pluginEditorState.value = null
        payload.value = null
    }

    return {
        showModal,
        modalName,
        pluginEditorState,
        payload,
        setModalState,
        openPluginEditor,
        consumePayload,
        clearModal,
    }
})
