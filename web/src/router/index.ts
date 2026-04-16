import { createRouter, createWebHistory } from 'vue-router'
import MapPage from '../pages/MapPage.vue'

export default createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', component: MapPage },
    { path: '/about', component: () => import('../pages/AboutPage.vue') },
    { path: '/source', component: () => import('../pages/SourcePage.vue') },
  ]
})
