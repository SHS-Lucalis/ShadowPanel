export { default as GBreadcrumbs } from './components/GBreadcrumbs.vue'
export { default as GDeletableList } from './components/GDeletableList.vue'
export { default as GStatusBadge } from './components/GStatusBadge.vue'
export { default as Loading } from './components/Loading.vue'
export { default as Progressbar } from './components/Progressbar.vue'

export function install(app) {
  app.component('GBreadcrumbs', GBreadcrumbs)
  app.component('GDeletableList', GDeletableList)
  app.component('GStatusBadge', GStatusBadge)
  app.component('Loading', Loading)
  app.component('Progressbar', Progressbar)
}

export default { install }
