<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { DashboardProject } from '@/api/dashboard'
import LevelBadge from '@/features/schedule/LevelBadge.vue'

// The projects the caller works on, the most late first (spec 013, RF-23 to RF-27).
defineProps<{ projects: DashboardProject[] }>()
const { t } = useI18n()
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
</script>

<template>
  <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="my-projects" data-testid="my-projects">
    <h2 id="my-projects" class="text-base font-semibold">{{ t('dashboard.projects.title') }}</h2>
    <p v-if="!projects.length" class="text-sm text-muted-foreground" data-testid="my-projects-empty">{{ t('dashboard.projects.empty') }}</p>
    <ul v-else class="flex flex-col divide-y">
      <li v-for="p in projects" :key="p.id" class="flex flex-col gap-1.5 py-2.5 first:pt-0 last:pb-0" data-testid="my-project">
        <div class="flex items-center gap-2">
          <RouterLink :to="`/projects/${p.id}`" class="min-w-0 truncate text-sm font-medium hover:underline">
            <span class="font-mono text-[11px] text-muted-foreground">{{ p.identifier }}</span> {{ p.name }}
          </RouterLink>
          <span class="ml-auto shrink-0 rounded bg-muted px-1.5 py-0.5 text-[11px] text-muted-foreground" data-testid="my-project-role">{{ t(`dashboard.projects.roles.${p.role}`) }}</span>
        </div>
        <div class="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
          <span>{{ t('dashboard.projects.open', { n: p.open }) }}</span>
          <span v-if="p.late" class="font-semibold text-destructive">{{ t('dashboard.projects.late', { n: p.late }) }}</span>
          <span class="ml-auto">{{ t('projectViews.real', { real: pct(p.figures.real) }) }}</span>
          <LevelBadge v-if="p.figures.has_plan" :figures="p.figures" compact />
          <span v-else>{{ t('schedule.noPlan') }}</span>
        </div>
      </li>
    </ul>
  </section>
</template>
