<template>
  <div>
    <p class="mb-4 text-gray-600 dark:text-gray-400">
      {{ trans('games.import_pelican_egg_description') }}
    </p>

    <n-form
        label-placement="top"
        label-width="auto"
        ref="formRef"
    >
      <n-form-item :label="trans('games.pelican_egg_file')">
        <n-upload
            accept=".json"
            :max="1"
            :default-upload="false"
            @change="onFileChange"
        >
          <n-button>{{ trans('main.upload_file') }}</n-button>
        </n-upload>
      </n-form-item>

      <div v-if="errorMessage" class="mb-4 p-3 bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300 rounded">
        {{ errorMessage }}
      </div>

      <div v-if="eggPreview" class="mb-4 p-4 bg-gray-100 dark:bg-gray-800 rounded">
        <h4 class="font-semibold mb-2">{{ eggPreview.name }}</h4>
        <p v-if="eggPreview.author" class="text-sm text-gray-600 dark:text-gray-400">
          {{ trans('games.author') }}: {{ eggPreview.author }}
        </p>
        <p v-if="eggPreview.description" class="text-sm text-gray-600 dark:text-gray-400 mt-1">
          {{ eggPreview.description }}
        </p>
      </div>
    </n-form>

    <GButton
        color="blue"
        :disabled="!eggPreview || importing"
        :loading="importing"
        v-on:click="onClickImport"
    >
      <GIcon name="download" />
      <span>&nbsp;{{ trans('games.import') }}</span>
    </GButton>
  </div>
</template>

<script setup>
import { GIcon } from "@gameap/ui"
import { ref } from "vue"
import { trans } from "../../../i18n/i18n"
import GButton from "../../../components/GButton.vue"
import {
  NForm,
  NFormItem,
  NUpload,
  NButton,
} from "naive-ui"

const formRef = ref({})
const errorMessage = ref('')
const eggPreview = ref(null)
const eggJson = ref(null)
const importing = ref(false)

const emits = defineEmits(['import'])

const onFileChange = ({ file }) => {
  errorMessage.value = ''
  eggPreview.value = null
  eggJson.value = null

  if (!file || file.status === 'removed') {
    return
  }

  if (!file.file) {
    return
  }

  if (!file.name.endsWith('.json')) {
    errorMessage.value = trans('games.invalid_file_type_json')
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const parsed = JSON.parse(e.target.result)

      if (!parsed.name) {
        errorMessage.value = trans('games.pelican_egg_missing_name')
        return
      }

      if (!parsed.startup) {
        errorMessage.value = trans('games.pelican_egg_missing_startup')
        return
      }

      eggPreview.value = {
        name: parsed.name,
        author: parsed.author || '',
        description: parsed.description || '',
      }
      eggJson.value = parsed
    } catch {
      errorMessage.value = trans('games.invalid_json_format')
    }
  }
  reader.onerror = () => {
    errorMessage.value = trans('games.file_read_error')
  }
  reader.readAsText(file.file)
}

const onClickImport = () => {
  if (!eggJson.value) {
    return
  }

  importing.value = true
  emits('import', eggJson.value)
}

const resetForm = () => {
  errorMessage.value = ''
  eggPreview.value = null
  eggJson.value = null
  importing.value = false
}

defineExpose({
  resetForm,
  setImporting: (value) => { importing.value = value },
})
</script>
