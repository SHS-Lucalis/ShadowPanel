<template>
  <GEmpty v-if="!settings || settings.length === 0" />
  <n-form
      v-else
      label-placement="left"
      label-width="auto"
      class="settings-form"
      ref="settingsRef"
      :model="settingsForm"
  >
      <n-form-item v-for="setting in settings" :label="setting.label">
        <GSwitch
            v-if="setting.type === 'bool'"
            v-model:value="settingsForm[setting.name]"
        />
        <n-input
            v-if="setting.type === 'string'"
            v-model:value="settingsForm[setting.name]"
            type="text"
        />
      </n-form-item>

      <GFixedBottomBar>
        <GButton color="green" v-on:click="saveSettings()">
          <GIcon name="save" />
          <span class="inline">{{ trans('main.save') }}</span>
        </GButton>
      </GFixedBottomBar>
  </n-form>
</template>

<script setup>
import {trans} from "@/i18n/i18n"
import {useServerStore} from "@/store/server"
import {onMounted, ref} from "vue"
import {storeToRefs} from "pinia"
import {
  NForm,
  NFormItem,
  NInput,
} from "naive-ui"
import { GIcon, GEmpty, GSwitch } from '@gameap/ui'
import GButton from '@/components/GButton.vue'
import GFixedBottomBar from '@/components/GFixedBottomBar.vue'
import {errorNotification, notification} from "@/parts/dialogs";

const serverStore = useServerStore()

const settingsRef = ref({})
const settingsForm = ref({})

const {settings} = storeToRefs(serverStore)

onMounted(() => {
  serverStore.fetchSettings().
    catch((error) => {
      errorNotification(error)
    }).
    then(() => {
      for(const setting of settings.value) {
        if (setting.type === 'bool') {
          settingsForm.value[setting.name] = (
              setting.value === true ||
              setting.value === 1 || setting.value === '1' ||
              setting.value === 'true' || setting.value === 'True' || setting.value === 'TRUE' ||
              setting.value === 'on' || setting.value === 'On' || setting.value === 'ON'
          )
        } else {
          settingsForm.value[setting.name] = setting.value
        }
      }
    })
});

function saveSettings() {
  let settings = []
  for (const [key, value] of Object.entries(settingsForm.value)) {
    settings.push({
      name: key,
      value: value
    })
  }

  serverStore.saveSettings(settings).
    then(() => {
      notification({
        content: trans('servers.settings_update_success_msg'),
        type: 'success',
      }, () => {
        fetchSettings()
      })
    }).catch((error) => {
      errorNotification(error)
    })
}

function fetchSettings() {
  serverStore.fetchSettings().catch((error) => {
    errorNotification(error)
  })
}
</script>

<style scoped>
.settings-form :deep(.n-form-item-label) {
  max-width: 60%;
}
</style>