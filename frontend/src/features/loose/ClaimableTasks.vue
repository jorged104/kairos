<script setup lang="ts">
import { useQuery, useQueryClient } from '@tanstack/vue-query'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { claimTask, getClaimable, type Claimable } from '@/api/loose'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'

// «Para tomar en tu equipo» in Mi panel: the tasks of the caller's teams that
// nobody has; «La tomo» makes one theirs (spec 022, RF-18 to RF-21).
const { t, locale } = useI18n()
const route = useRoute()
const qc = useQueryClient()
const errorMessage = useErrorMessage()
const { data: tasks } = useQuery({ queryKey: ['claimable'], queryFn: getClaimable })

const error = ref<unknown>(null)
const busy = ref<string | null>(null)
async function claim(task: Claimable) {
  error.value = null
  busy.value = task.id
  try {
    await claimTask(task.id)
  } catch (e) {
    error.value = e // taken by somebody else meanwhile: the list is read again (RF-20)
  } finally {
    busy.value = null
    await Promise.all([qc.invalidateQueries({ queryKey: ['claimable'] }), qc.invalidateQueries({ queryKey: ['dashboard'] })])
  }
}

const today = new Date().toISOString().slice(0, 10)
const date = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z'))
</script>

<template>
  <section
    v-if="tasks?.length || error"
    class="flex flex-col gap-3 rounded-xl border border-sky-200 bg-sky-50 p-4 dark:border-sky-900 dark:bg-sky-950/40"
    aria-labelledby="claimable-title"
    data-testid="claimable"
  >
    <div class="flex flex-wrap items-baseline gap-x-2">
      <h2 id="claimable-title" class="text-base font-semibold">{{ t('loose.claimable.title') }}</h2>
      <span class="text-sm text-muted-foreground">{{ t('loose.claimable.count', { n: tasks?.length ?? 0 }, tasks?.length ?? 0) }}</span>
    </div>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <div class="grid gap-3 sm:grid-cols-[repeat(auto-fill,minmax(18rem,1fr))]">
      <article v-for="task in tasks ?? []" :key="task.id" class="flex flex-col gap-2 rounded-lg border bg-background p-3" data-testid="claimable-task">
        <p class="text-xs text-muted-foreground">
          <span class="font-mono">{{ task.key }}</span> · {{ task.team.name }} ·
          <span :class="task.due_date && task.due_date < today && 'font-semibold text-destructive'">{{ task.due_date ? t('loose.dueOn', { date: date(task.due_date) }) : t('loose.noDue') }}</span>
          · {{ t('loose.weightOf', { n: task.weight }) }}
        </p>
        <RouterLink :to="{ query: { ...route.query, item: task.key } }" class="text-sm font-semibold hover:underline">{{ task.title }}</RouterLink>
        <div class="flex items-center gap-2">
          <span class="text-xs text-muted-foreground">{{ t('loose.claimable.by', { name: task.author.name }) }}</span>
          <Button size="sm" class="ml-auto" :disabled="busy === task.id" :aria-label="t('loose.claimOf', { key: task.key })" data-testid="claim" @click="claim(task)">
            {{ t('loose.claim') }}
          </Button>
        </div>
      </article>
    </div>
  </section>
</template>
