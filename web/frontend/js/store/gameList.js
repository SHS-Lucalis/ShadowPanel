import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from '../config/axios'

export const useGameListStore = defineStore('games', () => {
    // State
    const games = ref([])
    const gameMods = ref([])
    const allGameMods = ref([])
    const apiProcesses = ref(0)

    // From legacy gameMods.js
    const selectedGameModId = ref(0)
    const gameModsList = ref([])

    // Getters
    const loading = computed(() => apiProcesses.value > 0)

    // Actions
    async function fetchGames() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/games')
            games.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchGameMods(gameId) {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/games/' + gameId + '/mods')
            gameMods.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function fetchAllGameMods() {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/game_mods')
            allGameMods.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function createGame(game) {
        apiProcesses.value++
        try {
            const response = await axios.post('/api/games', game)
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function createGameMod(mod) {
        apiProcesses.value++
        try {
            const response = await axios.post('/api/game_mods', mod)
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function upgradeGames() {
        apiProcesses.value++
        try {
            await axios.post(
                '/api/games/upgrade',
                {headers: {'X-Http-Method-Override': 'PATCH'}},
            )
        } finally {
            apiProcesses.value--
        }
    }

    async function deleteGameByCode(code) {
        apiProcesses.value++
        try {
            await axios.delete('/api/games/' + code)
        } finally {
            apiProcesses.value--
        }
    }

    async function deleteModById(id) {
        apiProcesses.value++
        try {
            await axios.delete('/api/game_mods/' + id)
        } finally {
            apiProcesses.value--
        }
    }

    async function importPelicanEgg(eggJson) {
        apiProcesses.value++
        try {
            const response = await axios.post('/api/games/import/pelican-egg', eggJson)
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function importGameAP(yamlContent) {
        apiProcesses.value++
        try {
            const response = await axios.post('/api/games/import/gameap', yamlContent, {
                headers: {
                    'Content-Type': 'application/x-yaml',
                },
            })
            return response.data
        } finally {
            apiProcesses.value--
        }
    }

    async function exportGame(gameCode) {
        apiProcesses.value++
        try {
            const response = await axios.get('/api/games/' + gameCode + '/export', {
                responseType: 'blob',
            })

            const contentDisposition = response.headers['content-disposition']
            let filename = gameCode + '.gameap.yaml'
            if (contentDisposition) {
                const match = contentDisposition.match(/filename="(.+)"/)
                if (match) {
                    filename = match[1]
                }
            }

            const url = window.URL.createObjectURL(new Blob([response.data]))
            const link = document.createElement('a')
            link.href = url
            link.setAttribute('download', filename)
            document.body.appendChild(link)
            link.click()
            link.remove()
            window.URL.revokeObjectURL(url)
        } finally {
            apiProcesses.value--
        }
    }

    // From legacy gameMods.js
    async function fetchGameModsList(gameCode) {
        if (!gameCode) {
            return
        }

        apiProcesses.value++
        try {
            const response = await axios.get('/api/game_mods/get_list_for_game/' + gameCode)
            gameModsList.value = response.data
        } finally {
            apiProcesses.value--
        }
    }

    function setSelectedGameMod(gameMod) {
        selectedGameModId.value = gameMod
    }

    return {
        // State
        games,
        gameMods,
        allGameMods,
        apiProcesses,
        selectedGameModId,
        gameModsList,

        // Getters
        loading,

        // Actions
        fetchGames,
        fetchGameMods,
        fetchAllGameMods,
        createGame,
        createGameMod,
        upgradeGames,
        deleteGameByCode,
        deleteModById,
        importPelicanEgg,
        importGameAP,
        exportGame,
        fetchGameModsList,
        setSelectedGameMod,
    }
})
