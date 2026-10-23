<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, RouterView, useRouter } from 'vue-router'
import { LogOut } from '@lucide/vue'
import { logout } from '@/api/auth'
import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth'
import type { Permission } from '@/types/api'

const { t } = useI18n()
const auth = useAuthStore()
const router = useRouter()

const links = computed(() =>
  [
    { to: '/', label: t('nav.status') },
    { to: '/users', label: t('nav.users') },
    { to: '/teams', label: t('nav.teams') },
    { to: '/attributes', label: t('nav.attributes'), perm: 'attributes:manage' as Permission },
    { to: '/idp-mappings', label: t('nav.idpMappings'), perm: 'idp_mappings:manage' as Permission },
    { to: '/audit', label: t('nav.audit'), perm: 'audit:read' as Permission },
    { to: '/admin/catalog', label: t('nav.catalog'), perm: 'catalog:manage' as Permission },
  ].filter((l) => !l.perm || auth.can(l.perm)),
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
</script>

<template>
  <div class="min-h-svh bg-muted/30">
    <header class="border-b bg-background">
      <div class="mx-auto flex max-w-6xl flex-wrap items-center gap-x-6 gap-y-2 px-4 py-3">
        <span class="font-heading text-lg font-semibold">{{ t('app.name') }}</span>
        <nav class="flex flex-wrap gap-1 text-sm" :aria-label="t('nav.label')">
          <RouterLink
            v-for="link in links"
            :key="link.to"
            :to="link.to"
            class="rounded-md px-2 py-1 text-muted-foreground hover:bg-muted hover:text-foreground"
            exact-active-class="bg-muted text-foreground"
          >
            {{ link.label }}
          </RouterLink>
        </nav>
        <div class="ml-auto flex items-center gap-2 text-sm">
          <RouterLink to="/account" class="text-muted-foreground hover:text-foreground" data-testid="account-link">
            {{ auth.me?.name }}
          </RouterLink>
          <Button variant="ghost" size="sm" data-testid="logout" @click="signOut">
            <LogOut />
            {{ t('auth.logout') }}
          </Button>
        </div>
      </div>
    </header>
    <main class="mx-auto max-w-6xl p-4">
      <RouterView />
    </main>
  </div>
</template>
