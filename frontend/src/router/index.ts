import { createRouter, createWebHistory, type RouteLocationNormalized, type Router, type RouterHistory } from 'vue-router'
import AppLayout from '@/layouts/AppLayout.vue'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/types/api'

declare module 'vue-router' {
  interface RouteMeta {
    public?: boolean
    permissions?: Permission[]
    /** The page can take the whole window, without the navigation (spec 005, RF-40). */
    fullWidth?: boolean
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
      { path: 'teams', name: 'teams', component: () => import('@/pages/TeamsPage.vue') },
      {
        path: 'attributes',
        name: 'attributes',
        component: () => import('@/pages/AttributesPage.vue'),
        meta: { permissions: ['attributes:manage'] as Permission[] },
      },
      {
        path: 'idp-mappings',
        name: 'idp-mappings',
        component: () => import('@/pages/IdpMappingsPage.vue'),
        meta: { permissions: ['idp_mappings:manage'] as Permission[] },
      },
      {
        path: 'audit',
        name: 'audit',
        component: () => import('@/pages/AuditPage.vue'),
        meta: { permissions: ['audit:read'] as Permission[] },
      },
      { path: 'account', name: 'account', component: () => import('@/pages/AccountPage.vue') },
      { path: 'projects', name: 'projects', component: () => import('@/pages/ProjectsPage.vue') },
      {
        path: 'projects/new',
        name: 'project-new',
        component: () => import('@/pages/ProjectNewPage.vue'),
        meta: { permissions: ['projects:create:all', 'projects:create:team'] as Permission[] },
      },
      { path: 'projects/:id', name: 'project', component: () => import('@/pages/ProjectPage.vue'), props: true },
      { path: 'milestones/:id', name: 'milestone', component: () => import('@/pages/MilestonePage.vue'), props: true },
      {
        path: 'work-items/:wikey',
        name: 'work-item',
        component: () => import('@/pages/WorkItemPage.vue'),
        props: true,
        meta: { fullWidth: true },
      },
      { path: 'objectives', name: 'objectives', component: () => import('@/pages/ObjectivesPage.vue') },
      { path: 'objectives/:id', name: 'objective', component: () => import('@/pages/ObjectivePage.vue'), props: true },
      {
        path: 'admin/catalog',
        name: 'catalog',
        component: () => import('@/pages/CatalogPage.vue'),
        meta: { permissions: ['catalog:manage'] as Permission[] },
      },
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
