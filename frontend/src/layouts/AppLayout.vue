<script setup lang="ts">
import { computed, ref, watch, type Component } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import {
  Activity,
  ArrowLeftRight,
  Award,
  BriefcaseBusiness,
  ChartGantt,
  ChevronDown,
  FolderKanban,
  LayoutDashboard,
  Library,
  LogOut,
  Menu,
  Network,
  PanelLeftClose,
  PanelLeftOpen,
  ScrollText,
  Settings,
  ServerCog,
  SlidersHorizontal,
  Target,
  Trophy,
  UserCog,
  Users,
  UsersRound,
  X,
} from '@lucide/vue'
import { logout } from '@/api/auth'
import PointsNotice from '@/features/gamification/PointsNotice.vue'
import NotificationBell from '@/features/notifications/NotificationBell.vue'
import NotificationToasts from '@/features/notifications/NotificationToasts.vue'
import { useFullWidth } from '@/features/pages/useFullWidth'
import { features } from '@/lib/features'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/types/api'

type Link = { to: string; label: string; icon: Component; perm?: Permission | Permission[]; also?: string[] }

const { t } = useI18n()
const auth = useAuthStore()
const router = useRouter()
const route = useRoute()
// A work item page in full width hides the navigation (RF-40).
const fullWidth = useFullWidth()
const wide = computed(() => fullWidth.on.value && !!route.meta.fullWidth)

const allowed = (l: Link) => !l.perm || auth.can(...[l.perm].flat())
// The work of the organization.
const work = computed(() =>
  (
    [
      { to: '/', label: t('nav.home'), icon: LayoutDashboard, perm: 'work:read' },
      { to: '/projects', label: t('nav.projects'), icon: FolderKanban, perm: 'work:read', also: ['/milestones', '/work-items'] },
      // Spec 023, RF-17a: administrators, Dirección and leads.
      { to: '/portfolio', label: t('nav.portfolio'), icon: BriefcaseBusiness, perm: 'oversight:view' },
      { to: '/tracking', label: t('nav.tracking'), icon: ChartGantt, perm: 'oversight:view' },
      // Spec 021, RF-19.
      { to: '/my-team', label: t('nav.myTeam'), icon: UsersRound, perm: ['dashboard:others:all', 'dashboard:others:team'] },
      { to: '/ranking', label: t('nav.ranking'), icon: Trophy },
      ...(features.objectives ? [{ to: '/objectives', label: t('nav.objectives'), icon: Target, perm: 'work:read' }] : []),
    ] as Link[]
  ).filter(allowed),
)
// One place for settings: the caller's own and, by permission, the administration.
const settings = computed(() =>
  (
    [
      { to: '/account', label: t('nav.account'), icon: UserCog },
      { to: '/users', label: t('nav.users'), icon: Users },
      { to: '/teams', label: t('nav.teams'), icon: Network },
      { to: '/attributes', label: t('nav.attributes'), icon: SlidersHorizontal, perm: 'attributes:manage' },
      { to: '/idp-mappings', label: t('nav.idpMappings'), icon: ArrowLeftRight, perm: 'idp_mappings:manage' },
      { to: '/admin/catalog', label: t('nav.catalog'), icon: Library, perm: 'catalog:manage' },
      { to: '/admin/gamification', label: t('nav.gamification'), icon: Award, perm: 'gamification:manage' },
      { to: '/admin/system', label: t('nav.system'), icon: ServerCog, perm: 'system:manage' },
      { to: '/audit', label: t('nav.audit'), icon: ScrollText, perm: 'audit:read' },
      { to: '/status', label: t('nav.status'), icon: Activity },
    ] as Link[]
  ).filter(allowed),
)

const under = (path: string, to: string) => path === to || path.startsWith(to + '/')
const isActive = (l: Link) =>
  l.to === '/' ? route.path === '/' : [l.to, ...(l.also ?? [])].some((p) => under(route.path, p))

// Per-browser conveniences: a narrow rail and the settings group folded.
function remembered(key: string, fallback: boolean) {
  let v = fallback
  try {
    const s = localStorage.getItem(key)
    if (s !== null) v = s === '1'
  } catch {
    // Blocked storage: the default.
  }
  const r = ref(v)
  watch(r, (x) => {
    try {
      localStorage.setItem(key, x ? '1' : '0')
    } catch {
      // Not remembered this time.
    }
  })
  return r
}
const collapsed = remembered('kairos.nav.collapsed', false)
const settingsOpen = remembered('kairos.nav.settings', true)
// Landing on a settings page unfolds its group.
watch(() => settings.value.some(isActive), (inside) => inside && (settingsOpen.value = true), { immediate: true })
const showSettings = computed(() => collapsed.value || settingsOpen.value)

// The drawer on small screens closes on every navigation.
const drawer = ref(false)
watch(() => route.fullPath, () => (drawer.value = false))

const initials = computed(() =>
  (auth.me?.name ?? '')
    .split(/\s+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase(),
)

async function signOut() {
  let target = '/login'
  try {
    target = (await logout()).redirect_url
  } catch {
    // The cookie may already be gone; the login screen is the right place anyway.
  }
  auth.set(null)
  // An OIDC session also ends at the IdP, which sends the browser back (RF-52).
  if (target.startsWith('http')) window.location.assign(target)
  else await router.push(target)
}

const item = (active: boolean) => [
  'flex items-center gap-3 rounded-md px-2.5 py-2 text-sm transition-colors',
  active ? 'bg-primary/10 font-medium text-foreground' : 'text-muted-foreground hover:bg-muted hover:text-foreground',
  collapsed.value ? 'lg:justify-center lg:px-0' : '',
]
</script>

<template>
  <div class="min-h-svh bg-muted/30 lg:flex">
    <!-- Top bar on small screens: the brand and the menu button. -->
    <header v-if="!wide" class="sticky top-0 z-30 flex items-center gap-2 border-b bg-background px-4 py-2 lg:hidden">
      <button
        type="button"
        class="grid size-9 place-items-center rounded-md hover:bg-muted"
        :aria-label="t('nav.open')"
        :aria-expanded="drawer"
        aria-controls="app-sidebar"
        data-testid="nav-open"
        @click="drawer = true"
      >
        <Menu class="size-5" />
      </button>
      <span class="font-heading text-lg font-semibold">{{ t('app.name') }}</span>
    </header>

    <div v-if="!wide && drawer" class="fixed inset-0 z-40 bg-black/40 lg:hidden" aria-hidden="true" @click="drawer = false" />

    <aside
      v-if="!wide"
      id="app-sidebar"
      class="fixed inset-y-0 left-0 z-50 flex w-64 flex-col border-r bg-background transition-transform lg:sticky lg:top-0 lg:h-svh lg:translate-x-0"
      :class="[drawer ? 'translate-x-0' : '-translate-x-full', collapsed ? 'lg:w-16' : 'lg:w-60']"
      data-testid="sidebar"
    >
      <div class="flex h-14 shrink-0 items-center gap-2 border-b px-3" :class="collapsed && 'lg:justify-center lg:px-0'">
        <span class="grid size-8 shrink-0 place-items-center rounded-lg bg-primary text-sm font-bold text-primary-foreground" aria-hidden="true">
          {{ t('app.name').slice(0, 1) }}
        </span>
        <span class="font-heading text-lg font-semibold" :class="collapsed && 'lg:hidden'">{{ t('app.name') }}</span>
        <button
          type="button"
          class="ml-auto grid size-8 place-items-center rounded-md hover:bg-muted lg:hidden"
          :aria-label="t('nav.close')"
          @click="drawer = false"
        >
          <X class="size-4" />
        </button>
      </div>

      <nav class="flex flex-1 flex-col gap-4 overflow-y-auto p-2" :aria-label="t('nav.label')">
        <ul class="flex flex-col gap-0.5">
          <li v-for="l in work" :key="l.to">
            <RouterLink
              :to="l.to"
              :class="item(isActive(l))"
              :aria-current="isActive(l) ? 'page' : undefined"
              :title="collapsed ? l.label : undefined"
              data-testid="nav-link"
            >
              <component :is="l.icon" class="size-4 shrink-0" aria-hidden="true" />
              <span :class="collapsed && 'lg:sr-only'">{{ l.label }}</span>
            </RouterLink>
          </li>
        </ul>

        <div class="flex flex-col gap-0.5 border-t pt-3" data-testid="nav-settings">
          <button
            type="button"
            class="flex items-center gap-3 rounded-md px-2.5 py-1.5 text-xs font-semibold uppercase tracking-wide text-muted-foreground hover:text-foreground"
            :class="collapsed && 'lg:hidden'"
            :aria-expanded="showSettings"
            aria-controls="nav-settings-list"
            data-testid="nav-settings-toggle"
            @click="settingsOpen = !showSettings"
          >
            <Settings class="size-4 shrink-0" aria-hidden="true" />
            <span>{{ t('nav.settings') }}</span>
            <ChevronDown class="ml-auto size-4 transition-transform" :class="!showSettings && '-rotate-90'" aria-hidden="true" />
          </button>
          <ul v-show="showSettings" id="nav-settings-list" class="flex flex-col gap-0.5">
            <li v-for="l in settings" :key="l.to">
              <RouterLink
                :to="l.to"
                :class="item(isActive(l))"
                :aria-current="isActive(l) ? 'page' : undefined"
                :title="collapsed ? l.label : undefined"
                data-testid="nav-link"
              >
                <component :is="l.icon" class="size-4 shrink-0" aria-hidden="true" />
                <span :class="collapsed && 'lg:sr-only'">{{ l.label }}</span>
              </RouterLink>
            </li>
          </ul>
        </div>
      </nav>

      <div class="flex shrink-0 flex-col gap-1 border-t p-2">
        <NotificationBell :collapsed="collapsed" />
        <RouterLink
          to="/account"
          class="flex items-center gap-3 rounded-md p-1.5 hover:bg-muted"
          :class="collapsed && 'lg:justify-center'"
          :title="collapsed ? auth.me?.name : undefined"
          data-testid="account-link"
        >
          <span class="grid size-8 shrink-0 place-items-center rounded-full bg-muted text-xs font-semibold" aria-hidden="true">{{ initials }}</span>
          <span class="min-w-0" :class="collapsed && 'lg:sr-only'">
            <span class="block truncate text-sm font-medium">{{ auth.me?.name }}</span>
            <span class="block truncate text-xs text-muted-foreground">{{ auth.me?.email }}</span>
          </span>
        </RouterLink>
        <div class="flex gap-1" :class="collapsed && 'lg:flex-col'">
          <button
            type="button"
            class="flex flex-1 items-center gap-2 rounded-md px-2.5 py-2 text-sm text-muted-foreground hover:bg-muted hover:text-foreground"
            :class="collapsed && 'lg:justify-center lg:px-0'"
            :title="collapsed ? t('auth.logout') : undefined"
            data-testid="logout"
            @click="signOut"
          >
            <LogOut class="size-4 shrink-0" aria-hidden="true" />
            <span :class="collapsed && 'lg:sr-only'">{{ t('auth.logout') }}</span>
          </button>
          <button
            type="button"
            class="hidden size-9 shrink-0 place-items-center rounded-md text-muted-foreground hover:bg-muted hover:text-foreground lg:grid"
            :class="collapsed && 'lg:w-full'"
            :aria-label="collapsed ? t('nav.expand') : t('nav.collapse')"
            :title="collapsed ? t('nav.expand') : t('nav.collapse')"
            data-testid="nav-collapse"
            @click="collapsed = !collapsed"
          >
            <PanelLeftOpen v-if="collapsed" class="size-4" />
            <PanelLeftClose v-else class="size-4" />
          </button>
        </div>
      </div>
    </aside>

    <main class="min-w-0 flex-1" :class="wide ? 'px-4 py-4 sm:px-8' : 'p-4 lg:p-6'" data-testid="main">
      <div :class="wide ? '' : 'mx-auto max-w-6xl'">
        <RouterView />
      </div>
    </main>
    <PointsNotice v-if="auth.me" />
    <NotificationToasts v-if="auth.me" />
  </div>
</template>
