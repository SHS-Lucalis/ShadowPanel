<template>
    <div>
        <div class="grid grid-cols-3 gap-4 my-3 hover:bg-stone-100 dark:hover:bg-stone-800 rounded p-1">
            <div><strong>{{ lang.modal.properties.disk }}:</strong></div>
            <div>{{ selectedDisk }}</div>
            <div class="text-right cursor-pointer">
                <GIcon
                    @click="copyToClipboard(selectedDisk)"
                    :title="lang.clipboard.copy"
                    name="copy"
                />
            </div>
        </div>
        <div class="grid grid-cols-3 gap-4 my-3 hover:bg-stone-100 dark:hover:bg-stone-800 rounded p-1">
            <div><strong>{{ lang.modal.properties.name }}:</strong></div>
            <div class="break-all">{{ selectedItem.basename }}</div>
            <div class="text-right cursor-pointer">
                <GIcon
                    @click="copyToClipboard(selectedItem.basename)"
                    :title="lang.clipboard.copy"
                    name="copy"
                />
            </div>
        </div>
        <div class="grid grid-cols-3 gap-4 my-3 hover:bg-stone-100 dark:hover:bg-stone-800 rounded p-1">
            <div><strong>{{ lang.modal.properties.path }}:</strong></div>
            <div class="break-all">{{ selectedItem.path }}</div>
            <div class="text-right cursor-pointer">
                <GIcon
                    @click="copyToClipboard(selectedItem.path)"
                    :title="lang.clipboard.copy"
                    name="copy"
                />
            </div>
        </div>
        <template v-if="selectedItem.type === 'file'">
            <div class="grid grid-cols-3 gap-4 my-3 hover:bg-stone-100 dark:hover:bg-stone-800 rounded p-1">
                <div><strong>{{ lang.modal.properties.size }}:</strong></div>
                <div>{{ bytesToHuman(selectedItem.size) }}</div>
                <div class="text-right cursor-pointer">
                    <GIcon
                        @click="copyToClipboard(bytesToHuman(selectedItem.size))"
                        :title="lang.clipboard.copy"
                        name="copy"
                    />
                </div>
            </div>
        </template>
        <template v-if="selectedItem.hasOwnProperty('timestamp')">
            <div class="grid grid-cols-3 gap-4 my-3 hover:bg-stone-100 dark:hover:bg-stone-800 rounded p-1">
                <div><strong>{{ lang.modal.properties.modified }}:</strong></div>
                <div>{{ timestampToDate(selectedItem.timestamp) }}</div>
                <div class="text-right cursor-pointer">
                    <GIcon
                        @click="copyToClipboard(timestampToDate(selectedItem.timestamp))"
                        :title="lang.clipboard.copy"
                        name="copy"
                    />
                </div>
            </div>
        </template>
        <template v-if="selectedItem.hasOwnProperty('acl')">
            <div class="grid grid-cols-3 gap-4 my-3 p-1">
                <div>{{ lang.modal.properties.access }}:</div>
                <div>{{ lang.modal.properties['access_' + selectedItem.acl] }}</div>
            </div>
        </template>
    </div>
</template>

<script setup>
import { computed } from 'vue'
import { GIcon } from '@gameap/ui'
import { useFileManagerStore } from '../../../stores/useFileManagerStore.js'
import { useTranslate } from '../../../composables/useTranslate.js'
import { useHelper } from '../../../composables/useHelper.js'
import { useModal } from '../../../composables/useModal.js'
import { notification } from '@/parts/dialogs.js'

const fm = useFileManagerStore()
const { lang } = useTranslate()
const { bytesToHuman, timestampToDate } = useHelper()
const { hideModal } = useModal()

const selectedDisk = computed(() => fm.selectedDisk)
const selectedItem = computed(() => fm.selectedItems[0])

function copyToClipboard(text) {
    const copy = () => {
        if (navigator.clipboard && navigator.clipboard.writeText) {
            return navigator.clipboard.writeText(text)
        } else {
            const textArea = document.createElement('textarea')
            textArea.value = text
            textArea.style.position = 'fixed'
            textArea.style.left = '-9999px'
            document.body.appendChild(textArea)
            textArea.select()
            document.execCommand('copy')
            document.body.removeChild(textArea)
            return Promise.resolve()
        }
    }
    copy().then(() => {
        notification({
            content: lang.value.notifications.copyToClipboard,
            type: 'success',
        })
    })
}

defineExpose({
    footerButtons: computed(() => [
        { label: lang.value.btn.cancel, color: 'black', icon: 'close', action: hideModal },
    ]),
})
</script>
