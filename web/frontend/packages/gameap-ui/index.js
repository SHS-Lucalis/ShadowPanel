export { default as GBreadcrumbs } from './components/GBreadcrumbs.vue'
export { default as GDeletableList } from './components/GDeletableList.vue'
export { default as GStatusBadge } from './components/GStatusBadge.vue'
export { default as Loading } from './components/Loading.vue'
export { default as Progressbar } from './components/Progressbar.vue'
export { default as GMenu } from './components/GMenu.vue'
export { default as GMenuButton } from './components/GMenuButton.vue'
export { default as GMenuItems } from './components/GMenuItems.vue'
export { default as GMenuItem } from './components/GMenuItem.vue'

export function install(app) {
  app.component('GBreadcrumbs', GBreadcrumbs)
  app.component('GDeletableList', GDeletableList)
  app.component('GStatusBadge', GStatusBadge)
  app.component('Loading', Loading)
  app.component('Progressbar', Progressbar)
  app.component('GMenu', GMenu)
  app.component('GMenuButton', GMenuButton)
  app.component('GMenuItems', GMenuItems)
  app.component('GMenuItem', GMenuItem)
}

export default { install }
