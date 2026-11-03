<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { BarChart3, ChevronDown, ChevronRight } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getMetrics, type Share } from '@/api/metrics'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useFolded } from './useFolded'

// The metrics of a work item, between its properties and its comments (spec
// 007, RF-5): folded at first, the choice remembered (RF-6, RF-7). They load
// again with every new version of the work item (RF-8).
const props = defineProps<{ workItemId: string; version: number }>()
const { t, locale } = useI18n()
const n = (v: number) => new Intl.NumberFormat(locale.value, { maximumFractionDigits: 1 }).format(v)
const errorMessage = useErrorMessage()
const open = useFolded('metrics')

const { data: m, error } = useQuery({
  queryKey: computed(() => ['metrics', props.workItemId, props.version]),
  queryFn: () => getMetrics(props.workItemId),
  enabled: open,
})

const days = (v: number | null | undefined, missing = false) =>
  v === null || v === undefined ? (missing ? t('metrics.noData') : t('common.none')) : t('metrics.days', { n: n(v) })
const share = (s: Share) => (s.no_weight ? t('metrics.noWeight') : t('metrics.share', { percent: n(s.percent), points: n(s.points) }))
const stateClass = { ahead: 'text-green-700 dark:text-green-400', on_track: 'text-foreground', behind: 'text-destructive', '': '' }
const due = computed(() => {
  const d = m.value?.due
  if (!d) return ''
  if (d.state === 'pending') return d.days === 0 ? t('metrics.due.today') : t('metrics.due.pending', { count: d.days }, d.days)
  if (d.state === 'overdue' || d.state === 'late') return t(`metrics.due.${d.state}`, { count: d.days }, d.days)
  return t(`metrics.due.${d.state}`)
})
</script>

<template>
  <section class="flex flex-col gap-3" :aria-label="t('metrics.title')" data-testid="metrics">
    <button
      type="button"
      class="flex w-fit items-center gap-2 rounded-md text-sm font-medium hover:text-foreground"
      :aria-expanded="open"
      data-testid="metrics-toggle"
      @click="open = !open"
    >
      <component :is="open ? ChevronDown : ChevronRight" class="size-4 text-muted-foreground" aria-hidden="true" />
      <BarChart3 class="size-4 text-muted-foreground" aria-hidden="true" />
      {{ t('metrics.title') }}
    </button>
    <template v-if="open">
      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <dl v-else-if="m" class="grid gap-x-6 gap-y-3 rounded-lg border bg-background p-4 text-sm sm:grid-cols-2" data-testid="metrics-list">
        <div v-if="m.schedule" class="flex flex-col gap-0.5" data-testid="metric-schedule">
          <dt class="text-muted-foreground">{{ t('metrics.schedule') }}</dt>
          <dd v-if="!m.schedule.has_plan">{{ t('metrics.noPlan') }}</dd>
          <dd v-else>
            <span class="font-medium" :class="stateClass[m.schedule.state]">{{ t(`metrics.states.${m.schedule.state}`) }}</span>
            · {{ t('metrics.progressVsExpected', { actual: m.schedule.actual, expected: m.schedule.expected }) }}
            ({{ m.schedule.deviation > 0 ? '+' : '' }}{{ t('metrics.points', { n: m.schedule.deviation }) }})
          </dd>
        </div>
        <div class="flex flex-col gap-0.5" data-testid="metric-contribution">
          <dt class="text-muted-foreground">{{ t('metrics.contribution') }}</dt>
          <dd v-if="m.contribution.tracking">{{ t('metrics.trackingShare') }}</dd>
          <template v-else>
            <dd v-if="m.contribution.parent">{{ t('metrics.toParent') }}: {{ share(m.contribution.parent) }}</dd>
            <dd>{{ t('metrics.toProject') }}: {{ share(m.contribution.project) }}</dd>
          </template>
        </div>
        <div class="flex flex-col gap-0.5" data-testid="metric-times">
          <dt class="text-muted-foreground">{{ t('metrics.times') }}</dt>
          <template v-if="m.times.closed">
            <dd>{{ t('metrics.lead') }}: {{ days(m.times.lead_days) }}</dd>
            <dd>{{ t('metrics.cycle') }}: {{ days(m.times.cycle_days, m.times.missing) }}</dd>
          </template>
          <template v-else>
            <dd>{{ t('metrics.age') }}: {{ days(m.times.age_days) }}</dd>
            <dd>{{ t('metrics.inProgress') }}: {{ days(m.times.in_progress_days, m.times.missing) }}</dd>
          </template>
          <dd>{{ t('metrics.inStatus') }}: {{ days(m.times.in_status_days, m.times.missing) }}</dd>
        </div>
        <div class="flex flex-col gap-0.5" data-testid="metric-due">
          <dt class="text-muted-foreground">{{ t('metrics.dueTitle') }}</dt>
          <dd :class="(m.due.state === 'overdue' || m.due.state === 'late') && 'text-destructive'">{{ due }}</dd>
        </div>
        <div class="flex flex-col gap-0.5" data-testid="metric-reopens">
          <dt class="text-muted-foreground">{{ t('metrics.reopens') }}</dt>
          <dd>{{ m.reopens === null ? t('metrics.noData') : m.reopens }}</dd>
        </div>
        <div class="flex flex-col gap-0.5" data-testid="metric-blocked">
          <dt class="text-muted-foreground">{{ t('metrics.blocks') }}</dt>
          <dd v-if="m.blocked.now">
            <span class="font-medium text-destructive">{{ t('metrics.blockedBy') }}</span>
            <template v-for="(b, i) in m.blocked.blockers" :key="i">
              {{ i ? ', ' : ' ' }}
              <RouterLink v-if="!b.hidden" :to="`/work-items/${b.key}`" class="font-mono hover:underline">{{ b.key }}</RouterLink>
              <span v-else class="text-muted-foreground">{{ t('metrics.hiddenBlocker') }}</span>
            </template>
          </dd>
          <dd v-else>{{ t('metrics.notBlocked') }}</dd>
          <dd>{{ t('metrics.blockedTotal') }}: {{ days(m.blocked.total_days, true) }}</dd>
        </div>
      </dl>
    </template>
  </section>
</template>
