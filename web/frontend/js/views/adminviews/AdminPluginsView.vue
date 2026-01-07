<template>
  <GBreadcrumbs :items="breadcrumbs"></GBreadcrumbs>

  <n-tabs v-model:value="activeTab" type="line" animated @update:value="onTabChange">
    <n-tab-pane name="installed" :tab="trans('plugins.installed')">
      <div v-if="updatablePlugins.length > 0" class="mb-6">
        <h3 class="text-lg font-semibold mb-2">{{ trans('plugins.updates_available') }}</h3>
        <n-data-table
            :bordered="false"
            :single-line="true"
            :columns="installedColumns"
            :data="updatablePlugins"
            :loading="loading"
            size="small"
        >
          <template #loading>
            <Loading />
          </template>
        </n-data-table>
      </div>

      <n-data-table
          :bordered="false"
          :single-line="true"
          :columns="installedColumns"
          :data="installedPluginsSorted"
          :loading="loading"
          :pagination="installedPagination"
      >
        <template #loading>
          <Loading />
        </template>
        <template #empty>
          <n-empty :description="trans('plugins.no_plugins')"></n-empty>
        </template>
      </n-data-table>
    </n-tab-pane>

    <n-tab-pane name="store" :tab="trans('plugins.store')">
      <n-data-table
          :bordered="false"
          :single-line="true"
          :columns="storeColumns"
          :data="plugins"
          :loading="loading"
          :pagination="false"
      >
        <template #loading>
          <Loading />
        </template>
        <template #empty>
          <n-empty :description="trans('plugins.no_plugins')"></n-empty>
        </template>
      </n-data-table>

      <div class="flex justify-center mt-4" v-if="lastPage > 1">
        <n-pagination
            v-model:page="storePage"
            :page-count="lastPage"
            @update:page="onStorePageChange"
        />
      </div>
    </n-tab-pane>
  </n-tabs>

  <n-modal
      v-model:show="detailsModalVisible"
      class="custom-card"
      preset="card"
      :title="currentPlugin?.name || ''"
      :bordered="false"
      style="width: 700px; max-width: 90vw;"
      :segmented="{content: 'soft', footer: 'soft'}"
  >
    <n-spin :show="actionLoading">
      <PluginDetailsModal
          v-if="currentPlugin"
          :plugin="currentPlugin"
          :versions="currentPluginVersions"
          :loading="loading"
          @install="onInstall"
          @update="onUpdate"
          @uninstall="onUninstall"
          @close="closeDetailsModal"
      />
    </n-spin>
  </n-modal>
</template>

<script setup>
import { GBreadcrumbs, Loading } from "@gameap/ui"
import { computed, ref, onMounted, h } from "vue"
import { trans } from "../../i18n/i18n"
import GButton from "../../components/GButton.vue"
import { usePluginStoreStore } from "../../store/pluginStore"
import { errorNotification, notification } from "../../parts/dialogs"
import {
  NEmpty,
  NDataTable,
  NModal,
  NTabs,
  NTabPane,
  NPagination,
  NSpin,
  NTag,
} from "naive-ui"
import { storeToRefs } from "pinia"
import PluginDetailsModal from "./forms/PluginDetailsModal.vue"

const pluginStore = usePluginStoreStore()

const {
  plugins,
  lastPage,
  currentPlugin,
  currentPluginVersions,
  loading,
  installedPlugins,
  updatablePlugins,
} = storeToRefs(pluginStore)

const breadcrumbs = computed(() => {
  return [
    {'route':'/', 'text':'GameAP', 'icon': 'gicon gicon-gameap'},
    {'route':{name: 'admin.plugins.index'}, 'text':trans('plugins.plugins')},
  ]
})

const activeTab = ref('installed')
const detailsModalVisible = ref(false)
const actionLoading = ref(false)
const storePage = ref(1)

const installedPluginsSorted = computed(() => {
  return [...installedPlugins.value].sort((a, b) => a.name.localeCompare(b.name))
})

const installedPagination = {
  pageSize: 15,
}

const createInstalledColumns = () => {
  return [
    {
      title: trans('plugins.name'),
      key: 'name',
      render(row) {
        return h('div', { class: 'flex items-center gap-2' }, [
          row.icon_url
              ? h('img', { src: row.icon_url, class: 'w-8 h-8 rounded', alt: row.name })
              : h('i', { class: 'fa-solid fa-puzzle-piece text-2xl text-gray-400' }),
          h('div', { class: 'flex flex-col' }, [
            h('span', { class: 'font-medium' }, row.name),
            row.labels?.length > 0
                ? h('div', { class: 'flex gap-1 mt-1' },
                    row.labels.map(label =>
                        h(NTag, {
                          size: 'tiny',
                          round: true,
                          style: label.color ? { backgroundColor: label.color, color: '#fff' } : {}
                        }, () => label.name)
                    )
                )
                : null
          ])
        ])
      },
    },
    {
      title: trans('plugins.version'),
      key: 'installed_version',
      width: 100,
    },
    {
      title: trans('plugins.category'),
      key: 'category',
      width: 120,
      render(row) {
        return row.category?.name || ''
      }
    },
    {
      title: trans('plugins.rating'),
      key: 'rating_avg',
      width: 140,
      render(row) {
        return h('div', { class: 'flex items-center gap-1' }, [
          h('span', { class: 'text-yellow-500' }, renderStars(row.rating_avg)),
          h('span', { class: 'text-sm text-gray-500' }, `(${row.rating_count || 0})`)
        ])
      }
    },
    {
      title: trans('plugins.downloads'),
      key: 'download_count',
      width: 100,
      render(row) {
        return formatNumber(row.download_count)
      }
    },
    {
      title: trans('main.actions'),
      key: 'actions',
      width: 180,
      render(row) {
        const isUpdatable = row.installed_version && row.latest_version && row.installed_version !== row.latest_version
        return h('div', { class: 'flex gap-1' }, [
          h(GButton, {
            color: 'green',
            size: 'small',
            onClick: () => onShowDetails(row.id)
          }, () => [h('i', { class: 'fa-solid fa-eye' })]),
          isUpdatable
              ? h(GButton, {
                color: 'blue',
                size: 'small',
                onClick: () => onShowDetailsForUpdate(row.id)
              }, () => [h('i', { class: 'fa-solid fa-sync' })])
              : null,
          h(GButton, {
            color: 'red',
            size: 'small',
            onClick: () => onClickUninstall(row.id, row.name)
          }, () => [h('i', { class: 'fa-solid fa-trash' })]),
        ])
      },
    }
  ]
}

const createStoreColumns = () => {
  return [
    {
      title: trans('plugins.name'),
      key: 'name',
      render(row) {
        return h('div', { class: 'flex items-center gap-2' }, [
          row.icon_url
              ? h('img', { src: row.icon_url, class: 'w-8 h-8 rounded', alt: row.name })
              : h('i', { class: 'fa-solid fa-puzzle-piece text-2xl text-gray-400' }),
          h('div', { class: 'flex flex-col' }, [
            h('div', { class: 'flex items-center gap-2' }, [
              h('span', { class: 'font-medium' }, row.name),
              row.installed
                  ? h(NTag, { size: 'tiny', type: 'success' }, () => trans('plugins.already_installed'))
                  : null
            ]),
            row.labels?.length > 0
                ? h('div', { class: 'flex gap-1 mt-1' },
                    row.labels.map(label =>
                        h(NTag, {
                          size: 'tiny',
                          round: true,
                          style: label.color ? { backgroundColor: label.color, color: '#fff' } : {}
                        }, () => label.name)
                    )
                )
                : null
          ])
        ])
      },
    },
    {
      title: trans('plugins.version'),
      key: 'latest_version',
      width: 100,
    },
    {
      title: trans('plugins.category'),
      key: 'category',
      width: 120,
      render(row) {
        return row.category?.name || ''
      }
    },
    {
      title: trans('plugins.rating'),
      key: 'rating_avg',
      width: 140,
      render(row) {
        return h('div', { class: 'flex items-center gap-1' }, [
          h('span', { class: 'text-yellow-500' }, renderStars(row.rating_avg)),
          h('span', { class: 'text-sm text-gray-500' }, `(${row.rating_count || 0})`)
        ])
      }
    },
    {
      title: trans('plugins.downloads'),
      key: 'download_count',
      width: 100,
      render(row) {
        return formatNumber(row.download_count)
      }
    },
    {
      title: trans('main.actions'),
      key: 'actions',
      width: 150,
      render(row) {
        return h('div', { class: 'flex gap-1' }, [
          h(GButton, {
            color: 'green',
            size: 'small',
            onClick: () => onShowDetails(row.id)
          }, () => [h('i', { class: 'fa-solid fa-eye' })]),
          row.installed
              ? h(GButton, {
                color: 'gray',
                size: 'small',
                disabled: true,
              }, () => trans('plugins.already_installed'))
              : h(GButton, {
                color: 'blue',
                size: 'small',
                onClick: () => onShowDetailsForInstall(row.id)
              }, () => [h('i', { class: 'fa-solid fa-download mr-1' }), trans('plugins.install')]),
        ])
      },
    }
  ]
}

const installedColumns = ref(createInstalledColumns())
const storeColumns = ref(createStoreColumns())

function renderStars(rating) {
  const fullStars = Math.floor(rating || 0)
  const hasHalf = (rating || 0) - fullStars >= 0.5
  const emptyStars = 5 - fullStars - (hasHalf ? 1 : 0)

  return '★'.repeat(fullStars) + (hasHalf ? '½' : '') + '☆'.repeat(emptyStars)
}

function formatNumber(num) {
  if (!num) return '0'
  if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M'
  if (num >= 1000) return (num / 1000).toFixed(1) + 'K'
  return num.toString()
}

function onTabChange(tab) {
  if (tab === 'store' && plugins.value.length === 0) {
    fetchStorePlugins()
  }
}

function onStorePageChange(page) {
  storePage.value = page
  fetchStorePlugins()
}

function fetchStorePlugins() {
  pluginStore.fetchPlugins({
    page: storePage.value,
  }).catch(errorNotification)
}

function onShowDetails(id) {
  pluginStore.fetchPluginDetails(id).catch(errorNotification)
  pluginStore.fetchPluginVersions(id).catch(errorNotification)
  detailsModalVisible.value = true
}

function onShowDetailsForInstall(id) {
  onShowDetails(id)
}

function onShowDetailsForUpdate(id) {
  onShowDetails(id)
}

function closeDetailsModal() {
  detailsModalVisible.value = false
  pluginStore.clearCurrentPlugin()
}

function onInstall(version) {
  if (!currentPlugin.value) return

  actionLoading.value = true
  pluginStore.installPlugin(currentPlugin.value.id, version)
      .then(() => {
        notification({
          content: trans('plugins.install_success_msg'),
          type: 'success'
        })
        closeDetailsModal()
        refreshData()
      })
      .catch(errorNotification)
      .finally(() => {
        actionLoading.value = false
      })
}

function onUpdate(version) {
  if (!currentPlugin.value) return

  actionLoading.value = true
  pluginStore.updatePlugin(currentPlugin.value.id, version)
      .then(() => {
        notification({
          content: trans('plugins.update_success_msg'),
          type: 'success'
        })
        closeDetailsModal()
        refreshData()
      })
      .catch(errorNotification)
      .finally(() => {
        actionLoading.value = false
      })
}

function onUninstall() {
  if (!currentPlugin.value) return

  window.$dialog.warning({
    title: trans('plugins.uninstall_confirm_msg'),
    positiveText: trans('main.yes'),
    negativeText: trans('main.no'),
    closable: false,
    onPositiveClick: () => {
      actionLoading.value = true
      pluginStore.uninstallPlugin(currentPlugin.value.id)
          .then(() => {
            notification({
              content: trans('plugins.uninstall_success_msg'),
              type: 'success'
            })
            closeDetailsModal()
            refreshData()
          })
          .catch(errorNotification)
          .finally(() => {
            actionLoading.value = false
          })
    }
  })
}

function onClickUninstall(id, name) {
  window.$dialog.warning({
    title: trans('plugins.uninstall_confirm_msg'),
    positiveText: trans('main.yes'),
    negativeText: trans('main.no'),
    closable: false,
    onPositiveClick: () => {
      pluginStore.uninstallPlugin(id)
          .then(() => {
            notification({
              content: trans('plugins.uninstall_success_msg'),
              type: 'success'
            })
            refreshData()
          })
          .catch(errorNotification)
    }
  })
}

function refreshData() {
  pluginStore.fetchPlugins({ page: 1, perPage: 100 }).catch(errorNotification)
}

onMounted(() => {
  pluginStore.fetchPlugins({ page: 1, perPage: 100 }).catch(errorNotification)
})
</script>
