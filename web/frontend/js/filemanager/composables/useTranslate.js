import { computed } from 'vue'
import { useSettingsStore } from '../stores/useSettingsStore.js'

function isPlainObject(value) {
    return Object.prototype.toString.call(value) === '[object Object]'
}

function deepMerge(base, override) {
    if (!isPlainObject(base)) return override === undefined ? base : override

    const result = {}
    const keys = new Set([...Object.keys(base), ...Object.keys(override || {})])
    for (const key of keys) {
        const a = base[key]
        const b = override ? override[key] : undefined
        if (isPlainObject(a) && isPlainObject(b)) {
            result[key] = deepMerge(a, b)
        } else {
            result[key] = b !== undefined ? b : a
        }
    }

    return result
}

/**
 * Composable for translations.
 *
 * Returns the active language deep-merged on top of English so that any
 * key missing from the active language falls back to the English value
 * instead of rendering as `undefined`.
 */
export function useTranslate() {
    const settings = useSettingsStore()

    const lang = computed(() => {
        const en = settings.translations.en || {}
        const active = Object.prototype.hasOwnProperty.call(settings.translations, settings.lang)
            ? settings.translations[settings.lang]
            : en
        if (active === en) return en

        return deepMerge(en, active)
    })

    return {
        lang,
    }
}
