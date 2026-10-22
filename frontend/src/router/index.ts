import { createRouter, createWebHistory, type RouteLocationNormalized, type Router, type RouterHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/types/api'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    permissions?: Permission[]
  }
}

/** Only relative paths inside Kairos are valid return targets. */
export function safeRedirect(value: unknown): string {
  return typeof value === 'string' && value.startsWith('/') && !value.startsWith('//') ? value : '/'
}

export const routes = [
  { path: '/login', name: 'login', component: () => import('@/pages/LoginPage.vue'), meta: { public: true } },
  { path: '/register', name: 'register', component: () => import('@/pages/RegisterPage.vue'), meta: { public: true } },
  {
    path: '/',
    component: AppLayout,
    children: [
      { path: '', name: 'status', component: () => import('@/pages/StatusPage.vue') },
      { path: 'users', name: 'users', component: () => import('@/pages/UsersPage.vue') },
      {
        path: 'users/new',
        name: 'user-new',
        component: () => import('@/pages/UserNewPage.vue'),
        meta: { permissions: ['users:create:all', 'users:create:team'] as Permission[] },
      },
      { path: 'users/:id', name: 'user', component: () => import('@/pages/UserDetailPage.vue'), props: true },
      { path: 'account', name: 'account', component: () => import('@/pages/AccountPage.vue') },
    ],
  },
  { path: '/:pathMatch(.*)*', redirect: '/' },
]

/**
 * Every route except login and register needs a session; without one the
 * visitor goes to the login with the original path to come back to (RF-121).
 */
export async function authGuard(to: RouteLocationNormalized) {
  const auth = useAuthStore()
  if (!auth.loaded) await auth.load()
  if (to.meta.public) {
    return auth.me && to.name === 'login' ? safeRedirect(to.query.redirect) : true
  }
  if (!auth.me) return { name: 'login', query: { redirect: to.fullPath } }
  if (to.meta.permissions && !auth.can(...to.meta.permissions)) return { name: 'status' }
  return true
}

export function createAppRouter(history: RouterHistory = createWebHistory()): Router {
  const router = createRouter({ history, routes })
  router.beforeEach(authGuard)
  return router
}

export const router = createAppRouter()
