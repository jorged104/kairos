<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { Check, Circle } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { getTeamTasks, type TeamTask } from '@/api/loose'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'

// «Tareas del equipo» in Mi equipo: who took each «one person takes it» task
// or since when nobody has, and how far each «each person» one goes
// (spec 022, RF-33 to RF-35). With team, only that team's.
const props = defineProps<{ today: string; team?: string }>()
const { t, locale } = useI18n()
const route = useRoute()
const errorMessage = useErrorMessage()
const { data, error } = useQuery({ queryKey: ['team-tasks'], queryFn: getTeamTasks })

const shown = computed(() => (data.value ?? []).filter((x) => !props.team || x.team.id === props.team))
const claim = computed(() => shown.value.filter((x) => x.mode === 'claim'))
const each = computed(() => shown.value.filter((x) => x.mode === 'each'))

const DAY = 86400000
const waiting = (x: TeamTask) => Math.max(0, Math.floor((Date.parse(props.today) - Date.parse(x.created_at.slice(0, 10))) / DAY))
const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z'))
const late = (x: TeamTask) => !x.closed && !!x.due_date && x.due_date < props.today
const open = (x: TeamTask) => ({ query: { ...route.query, item: x.key } })
const pct = (x: TeamTask) => (x.total ? Math.round((x.done / x.total) * 100) : 0)
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="team-tasks">
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <p v-if="data && !shown.length" class="rounded-lg bg-muted p-4 text-center text-sm" data-testid="team-tasks-empty">{{ t('loose.teamTasks.empty') }}</p>

    <section v-if="claim.length" class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="tt-claim">
      <div class="flex flex-wrap items-baseline gap-x-2">
        <h2 id="tt-claim" class="text-base font-semibold">{{ t('loose.teamTasks.claimTitle') }}</h2>
        <span class="text-sm text-muted-foreground">{{ t('loose.teamTasks.claimHint') }}</span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full min-w-[640px] text-sm">
          <thead>
            <tr class="text-left text-xs text-muted-foreground">
              <th class="py-2 pr-2 font-medium">{{ t('loose.teamTasks.task') }}</th>
              <th class="w-36 p-2 font-medium">{{ t('loose.teamTasks.team') }}</th>
              <th class="w-28 p-2 font-medium">{{ t('loose.teamTasks.due') }}</th>
              <th class="w-56 p-2 font-medium">{{ t('loose.teamTasks.who') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="x in claim" :key="x.id" class="border-t" data-testid="team-task-claim">
              <td class="py-2 pr-2">
                <span class="font-mono text-xs text-muted-foreground">{{ x.key }}</span>
                <RouterLink :to="open(x)" class="ml-1.5 font-medium hover:underline">{{ x.title }}</RouterLink>
              </td>
              <td class="p-2">{{ x.team.name }}</td>
              <td class="p-2" :class="late(x) ? 'font-semibold text-destructive' : 'text-muted-foreground'">{{ x.due_date ? date(x.due_date) : t('loose.noDue') }}</td>
              <td class="p-2">
                <span v-if="x.closed" class="text-muted-foreground">{{ t('loose.teamTasks.closed') }}</span>
                <template v-else-if="x.people.length">
                  <RouterLink v-for="p in x.people" :key="p.id" :to="{ path: '/', query: { user: p.id } }" class="font-medium hover:underline">{{ p.name }}</RouterLink>
                </template>
                <span v-else class="rounded-full border border-orange-300 bg-orange-50 px-2 py-0.5 text-xs text-orange-800 dark:border-orange-900 dark:bg-orange-950 dark:text-orange-300" data-testid="team-task-untaken">
                  {{ t('loose.teamTasks.untakenDays', { count: waiting(x) }, waiting(x)) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>

    <section v-if="each.length" class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="tt-each">
      <div class="flex flex-wrap items-baseline gap-x-2">
        <h2 id="tt-each" class="text-base font-semibold">{{ t('loose.teamTasks.eachTitle') }}</h2>
        <span class="text-sm text-muted-foreground">{{ t('loose.teamTasks.eachHint') }}</span>
      </div>
      <article v-for="x in each" :key="x.id" class="flex flex-col gap-3 rounded-lg border p-3" data-testid="team-task-each">
        <div class="flex flex-wrap items-center gap-x-2 gap-y-1">
          <span class="font-mono text-xs text-muted-foreground">{{ x.key }}</span>
          <RouterLink :to="open(x)" class="font-semibold hover:underline">{{ x.title }}</RouterLink>
          <span class="text-sm text-muted-foreground">{{ x.team.name }}<template v-if="x.due_date"> · {{ t('loose.dueOn', { date: date(x.due_date) }) }}</template></span>
          <span class="ml-auto text-sm font-semibold" data-testid="team-task-progress">{{ t('loose.progress', { done: x.done, total: x.total }) }}</span>
        </div>
        <div class="h-2 overflow-hidden rounded-full bg-muted" role="progressbar" :aria-label="t('loose.progressLabel')" :aria-valuenow="pct(x)" aria-valuemin="0" aria-valuemax="100">
          <div class="h-full rounded-full bg-primary" :style="{ width: `${pct(x)}%` }" />
        </div>
        <ul class="grid gap-1.5 text-sm sm:grid-cols-[repeat(auto-fill,minmax(12rem,1fr))]">
          <li v-for="p in x.people" :key="p.id" class="flex items-center gap-2">
            <Check v-if="p.done" class="size-4 text-emerald-700 dark:text-emerald-400" role="img" :aria-label="t('loose.teamTasks.done')" />
            <Circle v-else class="size-4 text-muted-foreground" role="img" :aria-label="t('loose.teamTasks.pending')" />
            <RouterLink :to="{ path: '/', query: { user: p.id } }" class="hover:underline">{{ p.name }}</RouterLink>
          </li>
        </ul>
      </article>
    </section>
  </div>
</template>
