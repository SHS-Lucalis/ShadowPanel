import { reactive } from 'vue'
import { defaultIconMap } from './iconMap.js'

const iconRegistry = reactive({ ...defaultIconMap })

export function registerIcons(icons) {
  Object.assign(iconRegistry, icons)
}

export function getIcon(name) {
  return iconRegistry[name]
}

export function hasIcon(name) {
  return name in iconRegistry
}

export { iconRegistry }
