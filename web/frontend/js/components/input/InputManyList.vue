<template>
  <div class="block w-full overflow-auto scrolling-touch" :class="$attrs.class">
    <div class="mb-3">
      <GButton color="green" size="small" v-on:click="addItem">
        <GIcon name="add" />
      </GButton>
    </div>

    <GDataTable :columns="columns" :data="items" />

    <div class="flex justify-center mt-2">
      <GButton color="green" size="small" v-on:click="addItem">
        <GIcon name="add" />&nbsp;{{ trans('main.add') }}
      </GButton>
    </div>

    <n-modal
      v-model:show="showTextareaModal"
      preset="card"
      :title="trans('main.edit')"
      :bordered="false"
      :segmented="{ content: 'soft', footer: 'soft' }"
      style="width: 600px; max-width: 90vw;"
    >
      <n-input
        v-model:value="textareaValue"
        type="textarea"
        :autosize="{ minRows: 6, maxRows: 20 }"
      />
      <template #footer>
        <div class="flex justify-end gap-2">
          <GButton color="black" @click="closeTextareaModal">
            <GIcon name="close" class="mr-1" />
            {{ trans('main.close') }}
          </GButton>
          <GButton color="green" @click="saveTextareaModal">
            <GIcon name="save" class="mr-1" />
            {{ trans('main.save') }}
          </GButton>
        </div>
      </template>
    </n-modal>
  </div>
</template>

<script setup>
import GButton from "../GButton.vue";
import { GIcon, GDataTable } from '@gameap/ui';
import {
  NInput,
  NSwitch,
  NInputGroup,
  NButton,
  NModal,
} from "naive-ui"
import { ref, reactive, computed, defineModel, h } from 'vue';
import {trans} from "@/i18n/i18n";

const props = defineProps({
  labels: Array,
  keys: Array,
  inputTypes: Array,
  name: String,
});

const items = defineModel()

const showTextareaModal = ref(false)
const editingRowIndex = ref(null)
const editingKey = ref(null)
const textareaValue = ref('')

const classes = reactive({
  'text': 'form-control',
  'checkbox': '',
});

// Methods
const removeItem = (index) => {
  items.value.splice(index, 1);
};

const addItem = () => {
  let emptyItem = {};
  props.keys.forEach((item) => {
    emptyItem[item] = '';
  });

  items.value.push(emptyItem);
};

const openTextareaModal = (index, key, currentValue) => {
  editingRowIndex.value = index
  editingKey.value = key
  textareaValue.value = currentValue || ''
  showTextareaModal.value = true
}

const saveTextareaModal = () => {
  if (editingRowIndex.value !== null && editingKey.value !== null) {
    items.value[editingRowIndex.value][editingKey.value] = textareaValue.value
  }
  closeTextareaModal()
}

const closeTextareaModal = () => {
  showTextareaModal.value = false
  editingRowIndex.value = null
  editingKey.value = null
  textareaValue.value = ''
}

const columns = computed(() => {
  let result = [];

  for (let i = 0; i < props.labels.length; i++) {
    result.push({
      title: props.labels[i],
      key: props.keys[i],
      render(row, index) {
        switch (props.inputTypes[i]) {
          case 'text':
            return h(NInputGroup, {}, {
              default: () => [
                h(NInput, {
                  value: row[props.keys[i]],
                  onUpdateValue(v) {
                    items.value[index][props.keys[i]] = v
                  }
                }),
                h(NButton, {
                  type: 'default',
                  ghost: true,
                  onClick: () => openTextareaModal(index, props.keys[i], row[props.keys[i]])
                }, {
                  default: () => h(GIcon, { name: 'maximize' })
                })
              ]
            })
          case 'checkbox':
            return h(NSwitch, {
              value: row[props.keys[i]],
              onUpdateValue(v) {
                items.value[index][props.keys[i]] = v;
              }
            });
        }
      }
    });
  }

  result.push({
    title: trans('main.actions'),
    render(row, index) {
      return [
        h(GButton, {
          color: 'red',
          size: 'small',
          text: trans('main.delete'),
          onClick: () => {
            removeItem(index)
          },
        }, { default: () => [
          h(GIcon, {name: 'close', class: 'mr-0.5'}),
          h("span", {class: 'hidden lg:inline'}, trans('main.delete')),
        ]}),
      ]
    },
  })

  return result;
});
</script>
