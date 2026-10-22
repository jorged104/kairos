import { createRouter, createWebHistory } from 'vue-router'
import StatusPage from '@/pages/StatusPage.vue'

export const router = createRouter({
  history: createWebHistory(),
  routes: [{ path: '/', name: 'status', component: StatusPage }],
})
