<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { ArrowLeft } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { type Baseline, CHANGES, type Change, compareBaselines } from '@/api/schedule'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Project } from '@/types/work'

// What changed between two versions of the baseline, or one and today
// (spec 024). The sides live in the address so the link can be shared (RF-8).
const props = defineProps<{ project: Project }>()
const { t, n, locale } = useI18n()
const errorMessage = useErrorMessage()
const route = useRoute()
const router = useRouter()

const TODAY = 'today'
const asked = computed(() => ({ from: route.query.from as string | undefined, to: route.query.to as string | undefined }))
const same = computed(() => !!asked.value.from && asked.value.from === asked.value.to) // RF-6

const { data, error, isFetching } = useQuery({
  queryKey: computed(() => ['baseline-compare', props.project.id, same.value ? '' : asked.value.from, same.value ? '' : asked.value.to]),
  queryFn: () => (same.value ? compareBaselines(props.project.id) : compareBaselines(props.project.id, asked.value.from, asked.value.to)),
  placeholderData: keepPreviousData,
})

const from = computed(() => asked.value.from ?? (data.value ? String(data.value.before.version) : ''))
const to = computed(() => asked.value.to ?? (data.value ? (data.value.after ? String(data.value.after.version) : TODAY) : ''))
function pick(side: 'from' | 'to', value: string) {
  router.replace({ query: { ...route.query, from: from.value, to: to.value, [side]: value } })
}

// Added, removed and moved at first; unchanged hidden (RF-13, RF-14).
const shown = ref<Set<Change>>(new Set(['added', 'removed', 'moved']))
function toggle(c: Change) {
  const next = new Set(shown.value)
  if (next.has(c)) next.delete(c)
  else next.add(c)
  shown.value = next
}
const rows = computed(() => (data.value?.items ?? []).filter((it) => shown.value.has(it.change)))

const date = (d: string | null) =>
  d ? new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(d + 'T00:00:00')) : t('schedule.compare.noDate')
const days = (v: number | null) => (v === null ? '' : t('schedule.compare.days', { n: (v > 0 ? '+' : v < 0 ? '−' : '') + n(Math.abs(v)) }))
const stamp = (d: string) => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(d))
const version = (b: Baseline) => t('schedule.compare.version', { version: b.version, date: stamp(b.created_at) })
const describe = (b: Baseline | null) =>
  b ? `${t('schedule.baseline.current', { version: b.version, date: stamp(b.created_at), author: b.author.name })} — ${b.reason || t('schedule.baseline.noReason')}` : t('schedule.compare.todayHelp')
const shiftClass = (v: number | null) => (v === null || v === 0 ? '' : v > 0 ? 'text-destructive' : 'text-emerald-700 dark:text-emerald-400')
const back = computed(() => ({ query: { tab: 'overview' } }))
</script>

<template>
  <section class="flex flex-col gap-4 text-sm" aria-labelledby="cmp-title" data-testid="baseline-compare">
    <div class="flex flex-wrap items-center gap-2">
      <Button as-child variant="ghost" size="sm">
        <RouterLink :to="back" data-testid="compare-back"><ArrowLeft class="size-4" aria-hidden="true" />{{ t('schedule.compare.back') }}</RouterLink>
      </Button>
      <h2 id="cmp-title" class="text-base font-semibold">{{ t('schedule.compare.title') }}</h2>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <template v-if="data">
      <!-- The two sides (RF-2, RF-7). -->
      <div class="grid gap-3 rounded-xl border bg-background p-4 sm:grid-cols-2">
        <label v-for="side in (['from', 'to'] as const)" :key="side" class="flex flex-col gap-1">
          <span class="font-medium">{{ t(`schedule.compare.${side}`) }}</span>
          <select
            :value="side === 'from' ? from : to"
            class="h-9 rounded-md border bg-background px-2"
            :data-testid="`compare-${side}`"
            @change="pick(side, ($event.target as HTMLSelectElement).value)"
          >
            <option v-for="v in data.versions" :key="v.version" :value="String(v.version)">{{ version(v) }}</option>
            <option :value="TODAY">{{ t('schedule.compare.today') }}</option>
          </select>
        </label>
        <template v-if="!same">
          <p class="text-muted-foreground" data-testid="compare-before">{{ t('schedule.compare.before') }}: {{ describe(data.before) }}</p>
          <p class="text-muted-foreground" data-testid="compare-after">{{ t('schedule.compare.after') }}: {{ describe(data.after) }}</p>
        </template>
      </div>

      <Alert v-if="same" role="alert" data-testid="compare-same">
        <AlertDescription>{{ t('schedule.compare.same') }}</AlertDescription>
      </Alert>

      <template v-else>
        <!-- Summary (RF-9 to RF-11). -->
        <div class="grid grid-cols-2 gap-2 sm:grid-cols-5" :class="isFetching && 'opacity-60'" data-testid="compare-summary">
          <div v-for="c in CHANGES" :key="c" class="rounded-xl border bg-background p-3">
            <p class="text-muted-foreground">{{ t(`schedule.compare.changes.${c}`) }}</p>
            <p class="text-2xl font-semibold tabular-nums" :data-testid="`compare-count-${c}`">{{ n(data.summary[c]) }}</p>
          </div>
          <div class="col-span-2 rounded-xl border bg-background p-3 sm:col-span-1">
            <p class="text-muted-foreground">{{ t('schedule.compare.end') }}</p>
            <p class="font-medium">{{ date(data.summary.before_end) }} → {{ date(data.summary.after_end) }}</p>
            <p v-if="data.summary.end_shift !== null" class="font-semibold tabular-nums" :class="shiftClass(data.summary.end_shift)" data-testid="compare-end-shift">
              {{ days(data.summary.end_shift) }}
            </p>
          </div>
        </div>

        <!-- Filter by kind of change (RF-13). -->
        <div class="flex flex-wrap items-center gap-2" role="group" :aria-label="t('schedule.compare.filter')">
          <span class="text-muted-foreground">{{ t('schedule.compare.filter') }}</span>
          <Button
            v-for="c in CHANGES"
            :key="c"
            size="sm"
            :variant="shown.has(c) ? 'secondary' : 'outline'"
            :aria-pressed="shown.has(c)"
            :data-testid="`compare-filter-${c}`"
            @click="toggle(c)"
          >
            {{ t(`schedule.compare.changes.${c}`) }} ({{ n(data.summary[c]) }})
          </Button>
        </div>

        <!-- One row per work item (RF-12, RF-15 to RF-23). -->
        <div class="overflow-x-auto rounded-xl border bg-background">
          <table class="w-full min-w-[44rem]">
            <thead class="border-b text-left text-muted-foreground">
              <tr>
                <th class="px-3 py-2 font-medium">{{ t('schedule.compare.task') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('schedule.compare.change') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('schedule.compare.start') }}</th>
                <th class="px-3 py-2 font-medium">{{ t('schedule.compare.due') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="(it, i) in rows" :key="it.id ?? `${it.key}-${i}`" class="border-b last:border-0" data-testid="compare-row">
                <td class="px-3 py-2">
                  <span v-if="it.hidden && !it.title" class="text-muted-foreground">{{ t('schedule.compare.otherProject') }}</span>
                  <RouterLink v-else-if="it.id" :to="`/work-items/${it.key}`" class="hover:underline">
                    <span class="text-muted-foreground tabular-nums">{{ it.key }}</span> {{ it.title }}
                  </RouterLink>
                  <span v-else><span class="text-muted-foreground tabular-nums">{{ it.key }}</span> {{ it.title }}</span>
                </td>
                <td class="px-3 py-2">
                  <div class="flex flex-wrap gap-1">
                    <Badge variant="outline">{{ t(`schedule.compare.changes.${it.change}`) }}</Badge>
                    <Badge v-if="it.deleted" variant="secondary">{{ t('schedule.compare.deleted') }}</Badge>
                    <Badge v-if="it.elsewhere" variant="secondary">{{ t('schedule.compare.elsewhere') }}</Badge>
                  </div>
                </td>
                <td v-for="k in (['start', 'due'] as const)" :key="k" class="px-3 py-2 tabular-nums">
                  <template v-if="it.change === 'added'">{{ date(it[`after_${k}`]) }}</template>
                  <template v-else-if="it.change === 'removed'">{{ date(it[`before_${k}`]) }}</template>
                  <template v-else>
                    {{ date(it[`before_${k}`]) }}
                    <template v-if="it[`before_${k}`] !== it[`after_${k}`]"> → {{ date(it[`after_${k}`]) }}</template>
                    <span v-if="it[`${k}_shift`]" class="ml-1 font-medium" :class="shiftClass(it[`${k}_shift`])">
                      {{ days(it[`${k}_shift`]) }}
                    </span>
                  </template>
                </td>
              </tr>
              <tr v-if="!rows.length">
                <td colspan="4" class="px-3 py-6 text-center text-muted-foreground" data-testid="compare-empty">{{ t('schedule.compare.empty') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>
    </template>
  </section>
</template>
