<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { Tracking, TrackingTask } from '@/api/tracking'

// The leaf work items in four columns with their counters (spec 011, RF-30
// to RF-42).
const props = defineProps<{ data: Tracking }>()
const { t, locale } = useI18n()

const projectName = computed(() => new Map(props.data.projects.map((p) => [p.id, p.name])))
const date = (d: string | null) => (d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }).format(new Date(d + 'T00:00:00Z')) : t('common.none'))
type Key = keyof Tracking['columns']
const columns: { key: Key; tone: string }[] = [
  { key: 'late', tone: 'border-t-red-500' },
  { key: 'in_progress', tone: 'border-t-amber-500' },
  { key: 'done', tone: 'border-t-green-600' },
  { key: 'upcoming', tone: 'border-t-blue-500' },
]
function detail(key: Key, it: TrackingTask) {
  switch (key) {
    case 'late':
      return t('tracking.lateDetail', { ratio: it.ratio, due: date(it.due_date), count: it.late_days ?? 0 }, it.late_days ?? 0)
    case 'in_progress':
      return t('tracking.progressDetail', { ratio: it.ratio, due: date(it.due_date) })
    case 'done':
      return t('tracking.doneDetail', { date: date(it.closed_on) })
    default:
      return t('tracking.upcomingDetail', { start: date(it.start_date), due: date(it.due_date) })
  }
}
</script>

<template>
  <section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4" :aria-label="t('tracking.tasks')" data-testid="tracking-columns">
    <div v-for="c in columns" :key="c.key" class="flex min-w-0 flex-col rounded-xl border border-t-4 bg-background" :class="c.tone" :data-testid="`tracking-col-${c.key}`">
      <h2 class="flex items-baseline justify-between gap-2 border-b px-3 py-2 text-sm font-semibold">
        {{ t(`tracking.columns.${c.key}`) }}
        <span class="text-lg tabular-nums" data-testid="tracking-col-total">{{ data.columns[c.key].total }}</span>
      </h2>
      <ul class="flex max-h-[28rem] flex-col divide-y overflow-y-auto">
        <li v-for="it in data.columns[c.key].items" :key="it.id" class="px-3 py-2 text-sm">
          <RouterLink :to="`/work-items/${it.key}`" class="hover:underline" data-testid="tracking-task">
            <span class="font-mono text-xs text-muted-foreground">{{ it.key }}</span> {{ it.title }}
          </RouterLink>
          <p class="truncate text-xs text-muted-foreground">
            {{ projectName.get(it.project_id) }}<template v-if="it.parent_title"> · {{ it.parent_title }}</template>
          </p>
          <p class="text-xs">{{ detail(c.key, it) }}</p>
        </li>
        <li v-if="!data.columns[c.key].items.length" class="px-3 py-2 text-xs text-muted-foreground">{{ t('tracking.noTasks') }}</li>
      </ul>
      <p v-if="data.columns[c.key].total > data.columns[c.key].items.length" class="border-t px-3 py-1.5 text-xs text-muted-foreground" data-testid="tracking-more">
        {{ t('tracking.more', { n: data.columns[c.key].total - data.columns[c.key].items.length }) }}
      </p>
    </div>
  </section>
</template>
