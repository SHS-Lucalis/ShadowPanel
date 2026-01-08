<template>
    <div>
        <div class="mb-3 deletable" v-for="(item, index) in items" :key="index">
            <div class="flex flex-wrap ">
                <div class="md:w-full pr-4 pl-4 relative flex items-stretch w-full">
                  <n-input type="text" v-model:value="items[index]"></n-input>

                  <GButton color="red" size="small" class="ml-1" v-on:click="removeItem(index)">
                    <GIcon name="close" />
                  </GButton>
                </div>
            </div>
        </div>

        <div class="flex justify-center mt-2">
          <GButton color="green" size="small" v-on:click="addItem">
            <GIcon name="add" />
          </GButton>
        </div>
    </div>
</template>

<script setup>
import { defineProps, defineModel } from 'vue'
import GButton from "../GButton.vue"
import { GIcon } from '@gameap/ui'
import {
  NInput,
} from "naive-ui"

const props = defineProps({
  label: String,
  name: String
});

const items = defineModel()

const removeItem = (index) => {
  items.value.splice(index, 1);
};

const addItem = () => {
  if (!items.value) {
    items.value = ['']
  } else {
    items.value.push('');
  }
};
</script>
