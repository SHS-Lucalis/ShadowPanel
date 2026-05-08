import { computed, toValue } from 'vue'
import { useFileManagerStore } from '../stores/useFileManagerStore.js'
import { useSettingsStore } from '../stores/useSettingsStore.js'
import { useModalStore } from '../stores/useModalStore.js'

/**
 * Composable for manager-specific operations
 * Replaces the manager.js mixin
 * @param {string|Ref<string>} managerName - 'left' or 'right'
 */
export function useManager(managerName) {
    const fm = useFileManagerStore()
    const settings = useSettingsStore()
    const modal = useModalStore()

    // Helper to get the manager name (supports refs)
    const getManagerName = () => toValue(managerName)

    // State (computed for reactivity)
    const manager = computed(() => fm.getManager(getManagerName()))
    const isActive = computed(() => fm.activeManager === getManagerName())

    const selectedDisk = computed(() => manager.value.selectedDisk)
    const selectedDirectory = computed(() => manager.value.selectedDirectory)
    const sort = computed(() => manager.value.sort)
    const selected = computed(() => manager.value.selected)
    const lastSelectedPath = computed(() => manager.value.lastSelectedPath)
    const history = computed(() => manager.value.history)
    const historyPointer = computed(() => manager.value.historyPointer)
    const loading = computed(() => fm.getManagerLoading(getManagerName()))
    const error = computed(() => fm.getManagerError(getManagerName()))

    // Getters
    const files = computed(() => fm.getFiles(getManagerName()))
    const directories = computed(() => fm.getDirectories(getManagerName()))
    const filesCount = computed(() => fm.getFilesCount(getManagerName()))
    const directoriesCount = computed(() => fm.getDirectoriesCount(getManagerName()))
    const filesSize = computed(() => fm.getFilesSize(getManagerName()))
    const selectedList = computed(() => fm.getSelectedList(getManagerName()))
    const selectedCount = computed(() => fm.getSelectedCount(getManagerName()))
    const selectedFilesSize = computed(() => fm.getSelectedFilesSize(getManagerName()))
    const breadcrumb = computed(() => fm.getBreadcrumb(getManagerName()))

    // Unified ordered list of items (folders first, then files) used for range selection.
    const flatVisible = computed(() => [
        ...directories.value.map((d) => ({ type: 'directories', path: d.path })),
        ...files.value.map((f) => ({ type: 'files', path: f.path })),
    ])

    const canHistoryBack = computed(() => historyPointer.value > 0)
    const canHistoryForward = computed(() => historyPointer.value < history.value.length - 1)

    // ACL check
    const acl = computed(() => settings.acl)

    // File callback
    const fileCallback = computed(() => fm.fileCallback)

    // Extension checks
    const imageExtensions = computed(() => settings.imageExtensions)
    const textExtensions = computed(() => settings.textExtensions)
    const audioExtensions = computed(() => settings.audioExtensions)
    const videoExtensions = computed(() => settings.videoExtensions)

    // Actions
    function selectDirectory(path, addHistory = true) {
        return fm.selectDirectory(getManagerName(), { path, history: addHistory })
    }

    function refreshDirectory() {
        return fm.refreshDirectory(getManagerName())
    }

    function clearError() {
        fm.setManagerError(getManagerName(), null)
    }

    function levelUp() {
        if (!selectedDirectory.value) return

        const pathParts = selectedDirectory.value.split('/')
        pathParts.pop()

        if (pathParts.length) {
            selectDirectory(pathParts.join('/'))
        } else {
            selectDirectory(null)
        }
    }

    function historyBack() {
        fm.historyBack(getManagerName())
    }

    function historyForward() {
        fm.historyForward(getManagerName())
    }

    function sortBy(field, direction) {
        fm.sortBy(getManagerName(), { field, direction })
    }

    // Selection actions
    function addSelection(type, path) {
        fm.addToSelection(getManagerName(), { type, path })
    }

    function removeSelection(type, path) {
        fm.removeFromSelection(getManagerName(), { type, path })
    }

    function toggleSelect(type, path) {
        const arr = selected.value[type]
        if (arr.includes(path)) {
            removeSelection(type, path)
        } else {
            addSelection(type, path)
        }
    }

    function singleSelect(type, path) {
        fm.changeSelected(getManagerName(), { type, path })
    }

    function clearSelection() {
        fm.clearSelection(getManagerName())
    }

    function setAnchor(type, path) {
        fm.setAnchor(getManagerName(), { type, path })
    }

    function selectRangeTo(type, path) {
        const anchor = lastSelectedPath.value
        if (!anchor) {
            fm.changeSelected(getManagerName(), { type, path })

            return
        }
        fm.selectRange(getManagerName(), {
            fromAnchor: anchor,
            toItem: { type, path },
            visible: flatVisible.value,
        })
    }

    function selectAllVisible() {
        fm.selectAllVisible(getManagerName())
    }

    // Check functions
    function isSelected(type, path) {
        return selected.value[type].includes(path)
    }

    function directoryExist(basename) {
        return fm.directoryExist(getManagerName(), basename)
    }

    function fileExist(basename) {
        return fm.fileExist(getManagerName(), basename)
    }

    // File type checks
    function isImage(extension) {
        return imageExtensions.value.includes(extension.toLowerCase())
    }

    function isText(extension) {
        return Object.prototype.hasOwnProperty.call(textExtensions.value, extension.toLowerCase())
    }

    function isAudio(extension) {
        return audioExtensions.value.includes(extension.toLowerCase())
    }

    function isVideo(extension) {
        return videoExtensions.value.includes(extension.toLowerCase())
    }

    // Modal helpers
    function openModal(name) {
        modal.setModalState({ show: true, modalName: name })
    }

    function closeModal() {
        modal.clearModal()
    }

    // Get URL for file
    function getUrl(disk, path) {
        return fm.url({ disk, path })
    }

    // Open PDF
    function openPDF(disk, path) {
        fm.openPDF({ disk, path })
    }

    return {
        // State
        manager,
        isActive,
        selectedDisk,
        selectedDirectory,
        sort,
        selected,
        lastSelectedPath,
        history,
        historyPointer,
        loading,
        error,
        // Getters
        files,
        directories,
        flatVisible,
        filesCount,
        directoriesCount,
        filesSize,
        selectedList,
        selectedCount,
        selectedFilesSize,
        breadcrumb,
        canHistoryBack,
        canHistoryForward,
        acl,
        fileCallback,
        imageExtensions,
        textExtensions,
        audioExtensions,
        videoExtensions,
        // Actions
        selectDirectory,
        refreshDirectory,
        clearError,
        levelUp,
        historyBack,
        historyForward,
        sortBy,
        addSelection,
        removeSelection,
        toggleSelect,
        singleSelect,
        clearSelection,
        setAnchor,
        selectRangeTo,
        selectAllVisible,
        isSelected,
        directoryExist,
        fileExist,
        isImage,
        isText,
        isAudio,
        isVideo,
        openModal,
        closeModal,
        getUrl,
        openPDF,
    }
}
