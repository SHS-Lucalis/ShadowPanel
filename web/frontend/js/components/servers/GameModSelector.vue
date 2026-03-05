<template>
    <div>
        <div class="mb-3">
            <input type="hidden" name="game_id" v-model="gameModel">

            <n-form-item :label="trans('labels.game_id')" :path="gamePath">
              <div class="flex items-center gap-2 w-full">
                <n-select
                    class="flex-1"
                    filterable
                    :disabled="gameSelectDisabled"
                    v-model:value="gameModel"
                    :options="gamesOptions"
                    :render-label="renderGameLabel"
                />
                <slot name="game-actions"></slot>
              </div>
            </n-form-item>

        </div>

        <div class="mb-3">
            <input type="hidden" name="game_mod_id" v-model="gameModModel">

            <n-form-item :label="trans('labels.game_mod_id')" :path="gameModPath">
              <div class="flex items-center gap-2 w-full">
                <n-select
                    class="flex-1"
                    filterable
                    v-model:value="gameModModel"
                    :disabled="!gameModel"
                    :options="gameModOptions"
                />
                <slot name="mod-actions"></slot>
              </div>
            </n-form-item>
        </div>
    </div>
</template>

<script setup>
  import { computed, h, watch, defineModel, onUnmounted } from 'vue'
  import { storeToRefs } from 'pinia'
  import { useGameStore } from '@/store/game'
  import { useGameListStore } from '@/store/gameList'
  import { trans } from '@/i18n/i18n'
  import { NFormItem } from 'naive-ui'
  import { GGameIcon } from '@gameap/ui'

  const props = defineProps({
    games: Object,
    gamePath: "game",
    gameModPath: "gameMod",
    gameSelectDisabled: false,
  });

  const gameModel = defineModel('game')
  const gameModModel = defineModel('gameMod')

  const gameStore = useGameStore()
  const gameListStore = useGameListStore()
  const { gameModsList } = storeToRefs(gameListStore)

  onUnmounted(() => {
    gameListStore.setSelectedGameMod(null)
    gameStore.setGameCode(null)
  });

  const renderGameLabel = (option) => {
    return [
      h(GGameIcon, {game: option.value, class: 'mr-2'}),
      option.label,
    ]
  }

  const gamesOptions = computed(() => {
    return Object.entries(props.games).map(([gameCode, gameName]) => ({ value: gameCode, label: gameName }));
  });

  const gameModOptions = computed(() => {
    return gameModsList.value.map((gameMod) => ({ value: Number(gameMod.id), label: gameMod.name }));
  });

  watch(gameModel, () => {
    gameStore.setGameCode(gameModel.value)
    gameListStore.fetchGameModsList(gameModel.value)
  });

  watch(gameModModel, () => {
    gameListStore.setSelectedGameMod(gameModModel.value)
  });

  watch(gameModsList, (val) => {
    let mod = null
    if (val.length > 0) {
      mod = val[0].id
    }
    const found = val.find((element) => element.id === gameModModel.value);
    if (found) {
      mod = gameModModel.value;
    }

    gameModModel.value = mod;
  });
</script>

