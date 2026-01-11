<template>
    <div class="fm-modal-upload">
        <div class="fm-btn-wrapper relative overflow-hidden mb-4" v-show="!progressBar">
            <n-button class="w-full">{{ lang.btn.uploadSelect }}</n-button>
            <input
                type="file"
                multiple
                name="myfile"
                class="absolute left-0 top-0 opacity-0 cursor-pointer text-[100px] h-full w-full"
                @change="selectFiles($event)"
            />
        </div>
        <div class="fm-upload-list" v-if="countFiles">
            <div class="grid grid-cols-2 gap-4 my-4" v-for="(item, index) in newFiles" :key="index">
                <div class="truncate">
                    <GIcon :name="mimeToIcon(item.type)" />
                    {{ item.name }}
                </div>
                <div class="text-right">
                    {{ bytesToHuman(item.size) }}
                </div>
            </div>
            <GDivider />
            <div class="grid grid-cols-2 gap-4 my-4">
                <div>
                    <strong>{{ lang.modal.upload.selected }}</strong>
                    {{ newFiles.length }}
                </div>
                <div class="text-right">
                    <strong>{{ lang.modal.upload.size }}</strong>
                    {{ allFilesSize }}
                </div>
            </div>
            <GDivider />
            <div class="flex items-center gap-4 my-4">
                <div>
                    <strong>{{ lang.modal.upload.ifExist }}</strong>
                </div>
                <n-radio-group v-model:value="overwrite">
                    <n-radio :value="0">{{ lang.modal.upload.skip }}</n-radio>
                    <n-radio :value="1">{{ lang.modal.upload.overwrite }}</n-radio>
                </n-radio-group>
            </div>
            <GDivider />
        </div>
        <div v-else>
            <p>{{ lang.modal.upload.noSelected }}</p>
        </div>
        <div class="fm-upload-info my-4" v-show="countFiles">
            <n-progress
                type="line"
                :percentage="progressBar"
                :show-indicator="true"
            />
        </div>
    </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { GIcon, GDivider } from '@gameap/ui'
import { useFileManagerStore } from '../../../stores/useFileManagerStore.js'
import { useMessagesStore } from '../../../stores/useMessagesStore.js'
import { useTranslate } from '../../../composables/useTranslate.js'
import { useHelper } from '../../../composables/useHelper.js'
import { useModal } from '../../../composables/useModal.js'

const fm = useFileManagerStore()
const messages = useMessagesStore()
const { lang } = useTranslate()
const { bytesToHuman, mimeToIcon } = useHelper()
const { hideModal } = useModal()

const newFiles = ref([])
const overwrite = ref(0)

const progressBar = computed(() => messages.actionProgress)
const countFiles = computed(() => newFiles.value.length)

const allFilesSize = computed(() => {
    let size = 0
    for (let i = 0; i < newFiles.value.length; i += 1) {
        size += newFiles.value[i].size
    }
    return bytesToHuman(size)
})

function selectFiles(event) {
    if (event.target.files.length === 0) {
        newFiles.value = []
    } else {
        newFiles.value = event.target.files
    }
}

function uploadFiles() {
    if (countFiles.value) {
        fm.upload({
            files: newFiles.value,
            overwrite: overwrite.value,
        }).then((response) => {
            if (response.data.result.status === 'success') {
                hideModal()
            }
        })
    }
}

defineExpose({
    footerButtons: computed(() => [
        { label: lang.value.btn.submit, color: 'green', icon: 'upload', action: uploadFiles, disabled: !countFiles.value },
        { label: lang.value.btn.cancel, color: 'black', icon: 'close', action: hideModal },
    ]),
})
</script>

<style scoped>
.fm-btn-wrapper:hover :deep(.n-button) {
    background-color: var(--n-color-hover);
}
</style>
