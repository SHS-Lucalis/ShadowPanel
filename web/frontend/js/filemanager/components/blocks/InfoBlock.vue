<template>
    <div class="fm-info-block d-flex justify-content-between grid grid-cols-3">
        <div class="col-auto text-xs">
            <span v-show="selectedCount">
                {{ `${lang.info.selected} ${selectedCount}` }}
                {{ `${lang.info.selectedSize} ${selectedFilesSize}` }}
            </span>
            <span v-show="!selectedCount">
                {{ `${lang.info.directories} ${directoriesCount}` }}
                {{ `${lang.info.files} ${filesCount}` }}
                {{ `${lang.info.size} ${filesSize}` }}
            </span>
        </div>
        <div class="col-4"></div>
        <div class="col-auto text-right text-xs">
            <div class="spinner-border spinner-border-sm text-info" role="status" v-show="loadingSpinner">
                <span class="visually-hidden">Loading...</span>
            </div>
            <span
                v-show="clipboardType"
                v-on:click="showModal('ClipboardModal')"
                v-bind:title="[lang.clipboard.title + ' - ' + lang.clipboard[clipboardType]]"
            >
                <GIcon name="clipboard" />
            </span>
            <span
                v-on:click="showModal('StatusModal')"
                v-bind:class="[hasErrors ? 'text-danger' : 'text-success']"
                v-bind:title="lang.modal.status.title"
            >
                <GIcon name="info" />
            </span>
        </div>
    </div>
</template>

<script setup>
import { computed } from 'vue'
import { GIcon } from '@gameap/ui'
import { useFileManagerStore } from '../../stores/useFileManagerStore.js'
import { useMessagesStore } from '../../stores/useMessagesStore.js'
import { useModalStore } from '../../stores/useModalStore.js'
import { useTranslate } from '../../composables/useTranslate.js'
import { useHelper } from '../../composables/useHelper.js'

const fm = useFileManagerStore()
const messages = useMessagesStore()
const modal = useModalStore()
const { lang } = useTranslate()
const { bytesToHuman } = useHelper()

const activeManager = computed(() => fm.activeManager)
const hasErrors = computed(() => !!messages.errors.length)
const filesCount = computed(() => fm.getFilesCount(activeManager.value))
const directoriesCount = computed(() => fm.getDirectoriesCount(activeManager.value))
const filesSize = computed(() => bytesToHuman(fm.getFilesSize(activeManager.value)))
const selectedCount = computed(() => fm.getSelectedCount(activeManager.value))
const selectedFilesSize = computed(() => bytesToHuman(fm.getSelectedFilesSize(activeManager.value)))
const clipboardType = computed(() => fm.clipboard.type)
const loadingSpinner = computed(() => messages.loading)

function showModal(modalName) {
    modal.setModalState({
        modalName,
        show: true,
    })
}
</script>

<style lang="scss">
.fm-info-block {
    @apply border-t dark:border-stone-700 hidden sm:flex;
    flex: 0 0 auto;
    padding-top: 0.2rem;

    .text-right > span {
        padding-left: 0.5rem;
        cursor: pointer;
    }
}

@media (max-height: 500px) {
    .fm-info-block {
        display: none !important;
    }
}
</style>
