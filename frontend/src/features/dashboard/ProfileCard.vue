<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { Dashboard } from '@/api/dashboard'
import { useAuthStore } from '@/stores/auth'

// The caller's card: who they are and three figures of their load (spec 013, RF-5 to RF-7).
defineProps<{ profile: Dashboard['profile'] }>()
const { t } = useI18n()
const auth = useAuthStore()

const initials = computed(() =>
  (auth.me?.name ?? '')
    .split(/\s+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
    .toUpperCase(),
)
const roles = computed(() => (auth.me?.roles ?? []).map((r) => t(`roles.${r.role}`)).join(' · '))
const teams = computed(() => (auth.me?.teams ?? []).map((x) => x.name).join(', '))
</script>

<template>
  <section class="flex flex-col gap-4 rounded-xl border bg-background p-4" :aria-label="t('dashboard.profile.title')" data-testid="profile-card">
    <div class="flex items-center gap-3">
      <span class="grid size-12 shrink-0 place-items-center rounded-full bg-primary text-base font-semibold text-primary-foreground" aria-hidden="true">{{ initials }}</span>
      <div class="min-w-0">
        <p class="truncate font-semibold" data-testid="profile-name">{{ auth.me?.name }}</p>
        <p class="truncate text-xs text-muted-foreground">{{ roles }}</p>
        <p class="truncate text-xs text-muted-foreground">{{ teams || t('dashboard.profile.noTeams') }}</p>
      </div>
    </div>
    <dl class="grid grid-cols-3 gap-2 text-center">
      <div class="rounded-lg bg-muted/60 p-2">
        <dt class="text-xs text-muted-foreground">{{ t('dashboard.profile.open') }}</dt>
        <dd class="text-xl font-bold tabular-nums" data-testid="profile-open">{{ profile.open }}</dd>
      </div>
      <div class="rounded-lg bg-muted/60 p-2">
        <dt class="text-xs text-muted-foreground">{{ t('dashboard.profile.late') }}</dt>
        <dd class="text-xl font-bold tabular-nums" :class="profile.late ? 'text-destructive' : ''" data-testid="profile-late">{{ profile.late }}</dd>
      </div>
      <div class="rounded-lg bg-muted/60 p-2">
        <dt class="text-xs text-muted-foreground">{{ t('dashboard.profile.closedMonth') }}</dt>
        <dd class="text-xl font-bold tabular-nums" data-testid="profile-closed">{{ profile.closed_month }}</dd>
      </div>
    </dl>
    <RouterLink to="/account" class="text-sm font-medium hover:underline" data-testid="profile-account">{{ t('dashboard.profile.account') }}</RouterLink>
  </section>
</template>
