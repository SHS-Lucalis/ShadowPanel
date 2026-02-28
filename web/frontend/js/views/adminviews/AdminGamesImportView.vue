<template>
  <GBreadcrumbs :items="breadcrumbs" />

  <div class="max-w-2xl">
    <n-tabs v-model:value="activeTab" type="line" animated>
      <n-tab-pane name="pelican">
        <template #tab>Pelican/Pterodactyl</template>
        <ImportPelicanEggForm @import="onImportPelicanEgg" />
      </n-tab-pane>
    </n-tabs>
  </div>
</template>

<script setup>
import { GBreadcrumbs } from "@gameap/ui"
import { computed, ref } from "vue"
import { trans } from "../../i18n/i18n"
import { NTabs, NTabPane } from "naive-ui"
import { useRouter } from "vue-router"
import { useGameListStore } from "../../store/gameList"
import { errorNotification, notification } from "../../parts/dialogs"
import ImportPelicanEggForm from "./forms/ImportPelicanEggForm.vue"

const router = useRouter()
const gamesStore = useGameListStore()

const activeTab = ref('pelican')

const breadcrumbs = computed(() => {
  return [
    { route: '/', text: 'GameAP', icon: 'gicon gicon-gameap' },
    { route: { name: 'admin.games.index' }, text: trans('games.games') },
    { route: { name: 'admin.games.import' }, text: trans('games.title_import') },
  ]
})

const onImportPelicanEgg = (eggJson) => {
  gamesStore.importPelicanEgg(eggJson).then((response) => {
    const msg = trans('games.import_pelican_egg_success_msg')
        .replace(':game_name', response.game_name)
        .replace(':game_code', response.game_code)

    notification({
      content: msg,
      type: "success",
    })

    router.push({ name: 'admin.games.index' })
  }).catch((error) => {
    errorNotification(error)
  })
}
</script>
