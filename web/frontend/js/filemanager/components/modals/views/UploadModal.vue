<template>
    <div class="fm-modal-upload">
        <div class="fm-btn-wrapper relative overflow-hidden mb-4" v-show="!uploading">
            <n-button class="w-full">{{ lang.btn.uploadSelect }}</n-button>
            <input
                type="file"
                multiple
                name="myfile"
                class="absolute left-0 top-0 opacity-0 cursor-pointer text-[100px] h-full w-full"
                @change="selectFiles($event)"
            />
        </div>

        <div class="fm-upload-list" v-if="hasFiles && !uploading">
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
        </div>
        <div v-else-if="!uploading">
            <p>{{ lang.modal.upload.noSelected }}</p>
        </div>

        <div class="fm-upload-progress my-2" v-if="uploading">
            <div
                v-for="(item, index) in progressFiles"
                :key="index"
                class="flex flex-col gap-1 my-3"
            >
                <div class="flex items-center gap-2">
                    <GIcon :name="phaseIcon(item)" :class="phaseIconClass(item)" />
                    <span class="truncate flex-1" :title="item.name">{{ item.name }}</span>
                    <n-tag :type="phaseTagType(item)" size="small" round>
                        {{ phaseLabel(item) }}
                    </n-tag>
                    <span class="text-xs text-stone-500 shrink-0 w-12 text-right">
                        {{ item.phase === 'error' ? '' : `${filePercent(item)}%` }}
                    </span>
                </div>
                <n-progress
                    type="line"
                    :percentage="filePercent(item)"
                    :show-indicator="false"
                    :height="6"
                    :border-radius="3"
                    :status="progressStatus(item)"
                    :processing="item.phase === 'hashing' || item.phase === 'uploading' || item.phase === 'completing'"
                />
            </div>
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

const progressFiles = computed(() => messages.uploadProgress.files)
const uploading = computed(() => progressFiles.value.length > 0)
const hasFiles = computed(() => newFiles.value.length > 0)

const allFilesSize = computed(() => {
    let size = 0
    for (let i = 0; i < newFiles.value.length; i += 1) {
        size += newFiles.value[i].size
    }
    return bytesToHuman(size)
})

function selectFiles(event) {
    newFiles.value = event.target.files.length === 0 ? [] : Array.from(event.target.files)
}

function uploadFiles() {
    if (!hasFiles.value || uploading.value) return

    fm.upload({ files: newFiles.value }).then((response) => {
        if (response && response.data.result.status === 'success') {
            hideModal()
        }
    })
}

function filePercent(file) {
    if (!file.size) return 0
    return Math.min(Math.round((file.loaded * 100) / file.size), 100)
}

function phaseLabel(file) {
    const phases = lang.value.modal.upload.phase || {}
    if (file.phase === 'error') {
        const errors = lang.value.modal.upload.errors || {}
        return errors[file.error] || errors.unknown || 'Failed'
    }
    return phases[file.phase] || ''
}

function phaseIcon(file) {
    if (file.phase === 'error') return 'close'
    if (file.phase === 'done') return 'check'
    if (file.phase === 'hashing') return 'loading'
    if (file.phase === 'completing') return 'loading'
    if (file.phase === 'uploading') return 'upload'
    return 'file'
}

function phaseIconClass(file) {
    if (file.phase === 'error') return 'text-red-500'
    if (file.phase === 'done') return 'text-lime-500'
    if (file.phase === 'hashing' || file.phase === 'completing') return 'text-sky-500'
    return 'text-stone-500'
}

function phaseTagType(file) {
    if (file.phase === 'error') return 'error'
    if (file.phase === 'done') return 'success'
    return 'default'
}

function progressStatus(file) {
    if (file.phase === 'error') return 'error'
    if (file.phase === 'done') return 'success'
    return 'default'
}

defineExpose({
    footerButtons: computed(() =>
        uploading.value
            ? []
            : [
                  {
                      label: lang.value.btn.submit,
                      color: 'green',
                      icon: 'upload',
                      action: uploadFiles,
                      disabled: !hasFiles.value,
                  },
                  { label: lang.value.btn.cancel, color: 'black', icon: 'close', action: hideModal },
              ],
    ),
})
</script>

<style scoped>
.fm-btn-wrapper:hover :deep(.n-button) {
    background-color: var(--n-color-hover);
}
</style>
