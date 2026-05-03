<template>
    <div class="fm-navbar">
        <div class="grid grid-cols-2 gap-4">
            <div class="mb-2">
                <div class="btn-group mr-4" role="group">
                    <button
                        type="button"
                        class="btn btn-small btn-secondary rounded"
                        v-on:click="refreshAll()"
                        v-bind:title="lang.btn.refresh"
                    >
                        <GIcon name="refresh" />
                    </button>
                </div>
                <div class="btn-group mr-4" role="group">
                    <button
                        type="button"
                        class="btn btn-small btn-secondary rounded-s border-r"
                        v-on:click="showModal('NewFileModal')"
                        v-bind:title="lang.btn.file"
                    >
                        <GIcon name="file" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary border-r"
                        v-on:click="showModal('NewFolderModal')"
                        v-bind:title="lang.btn.folder"
                    >
                        <GIcon name="folder" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary border-r"
                        disabled
                        v-if="uploading"
                        v-bind:title="lang.btn.upload"
                    >
                        <GIcon name="upload" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary border-r"
                        v-else
                        v-on:click="showModal('UploadModal')"
                        v-bind:title="lang.btn.upload"
                    >
                        <GIcon name="upload" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary border-r"
                        v-bind:disabled="archiveDownloading"
                        v-on:click="handleDownloadDirectory"
                        v-bind:title="lang.btn.downloadDir"
                    >
                        <GIcon name="folder-download" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary rounded-e"
                        v-bind:disabled="!isAnyItemSelected"
                        v-on:click="showModal('DeleteModal')"
                        v-bind:title="lang.btn.delete"
                    >
                        <GIcon name="delete" />
                    </button>
                </div>
                <div class="btn-group mr-4" role="group">
                    <button
                        type="button"
                        class="btn btn-small btn-secondary rounded-s border-r"
                        v-bind:disabled="!isAnyItemSelected"
                        v-bind:title="lang.btn.copy"
                        v-on:click="handleToClipboard('copy')"
                    >
                        <GIcon name="copy" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary border-r"
                        v-bind:disabled="!isAnyItemSelected"
                        v-bind:title="lang.btn.cut"
                        v-on:click="handleToClipboard('cut')"
                    >
                        <GIcon name="cut" />
                    </button>
                    <button
                        type="button"
                        class="btn btn-small btn-secondary rounded-e"
                        v-bind:disabled="!clipboardType"
                        v-bind:title="lang.btn.paste"
                        v-on:click="handlePaste"
                    >
                        <GIcon name="paste" />
                    </button>
                </div>
            </div>
        </div>
    </div>
</template>

<script setup>
import { computed } from 'vue'
import { GIcon } from '@gameap/ui'
import { notification } from '@/parts/dialogs.js'
import { useFileManagerStore } from '../../stores/useFileManagerStore.js'
import { useMessagesStore } from '../../stores/useMessagesStore.js'
import { useModalStore } from '../../stores/useModalStore.js'
import { useSettingsStore } from '../../stores/useSettingsStore.js'
import { useTranslate } from '../../composables/useTranslate.js'

const fm = useFileManagerStore()
const messages = useMessagesStore()
const modal = useModalStore()
const settings = useSettingsStore()
const { lang } = useTranslate()

// Computed
const activeManager = computed(() => fm.activeManager)

const backDisabled = computed(() => !fm.getManager(activeManager.value).historyPointer)

const forwardDisabled = computed(() => {
    const manager = fm.getManager(activeManager.value)
    return manager.historyPointer === manager.history.length - 1
})

const isAnyItemSelected = computed(() => {
    const manager = fm.getManager(activeManager.value)
    return manager.selected.files.length > 0 || manager.selected.directories.length > 0
})

const uploading = computed(() => messages.actionProgress > 0)

const archiveDownloading = computed(() => {
    const status = messages.archiveDownload?.status
    return status === 'preparing' || status === 'downloading'
})

const clipboardType = computed(() => fm.clipboard.type)

const fullScreen = computed(() => fm.fullScreen)

const hiddenFiles = computed(() => settings.hiddenFiles)

// Methods
function refreshAll() {
    fm.refreshAll()
}

function historyBack() {
    fm.historyBack(activeManager.value)
}

function historyForward() {
    fm.historyForward(activeManager.value)
}

function handleToClipboard(type) {
    fm.toClipboard(type)

    if (type === 'cut') {
        notification({
            content: lang.value.notifications.cutToClipboard,
            type: 'success',
        })
    } else if (type === 'copy') {
        notification({
            content: lang.value.notifications.copyToClipboard,
            type: 'success',
        })
    }
}

function handlePaste() {
    fm.paste()
}

function handleDownloadDirectory() {
    fm.downloadCurrentDirectory()
}

function toggleHidden() {
    settings.toggleHiddenFiles()
}

function showModal(modalName) {
    modal.setModalState({ modalName, show: true })
}

function screenToggle() {
    const fmEl = document.getElementsByClassName('fm')[0]

    if (!fullScreen.value) {
        if (fmEl.requestFullscreen) {
            fmEl.requestFullscreen()
        } else if (fmEl.mozRequestFullScreen) {
            fmEl.mozRequestFullScreen()
        } else if (fmEl.webkitRequestFullscreen) {
            fmEl.webkitRequestFullscreen()
        } else if (fmEl.msRequestFullscreen) {
            fmEl.msRequestFullscreen()
        }
    } else if (document.exitFullscreen) {
        document.exitFullscreen()
    } else if (document.webkitExitFullscreen) {
        document.webkitExitFullscreen()
    } else if (document.mozCancelFullScreen) {
        document.mozCancelFullScreen()
    } else if (document.msExitFullscreen) {
        document.msExitFullscreen()
    }

    fm.screenToggle()
}
</script>

<style lang="scss">
.fm-navbar {
    flex: 0 0 auto;

    .col-auto > .btn-group:not(:last-child) {
        margin-right: 0.4rem;
    }

    .btn-group {
        @apply inline-flex;

        .btn {
            @apply border-stone-300 dark:border-stone-800 dark:text-stone-400 dark:disabled:text-stone-700;
        }
    }
}

.btn.btn-small {
  @apply py-1.5 px-2.5;
}

</style>
