<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Plus } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getPortfolio, setParent, type PortfolioCard } from '@/api/portfolio'
import { LEVELS } from '@/api/schedule'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AddProject from '@/features/portfolio/AddProject.vue'
import NewProgram from '@/features/portfolio/NewProgram.vue'
import ProgramRows from '@/features/portfolio/ProgramRows.vue'
import { DEFAULT_SORT, filterRows, levelOf, sortRows, type LevelFilter, type Sort } from '@/features/portfolio/rows'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { useLevels } from '@/features/schedule/useLevels'
import { useAuthStore } from '@/stores/auth'

// Every program with its projects and the projects without one, filtered by
// the traffic light and sorted (spec 010, RF-21 to RF-38).
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const levels = useLevels()
const { data, error } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })

// The administrator creates programs and hangs projects under them here
// (RF-41 to RF-48).
const auth = useAuthStore()
const qc = useQueryClient()
const canManage = computed(() => auth.can('portfolio:manage'))
const creating = ref(false)
function refresh() {
  qc.invalidateQueries({ queryKey: ['portfolio'] })
  qc.invalidateQueries({ queryKey: ['projects'] })
  qc.invalidateQueries({ queryKey: ['overview'] })
}
function created() {
  creating.value = false
  refresh()
}
const move = useMutation({
  mutationFn: (v: { id: string; parentId: string | null }) => setParent(v.id, v.parentId),
  onSuccess: (p) => {
    qc.invalidateQueries({ queryKey: ['project', p.id] })
    refresh()
  },
})

// The filter and the order are remembered in the browser (RF-37).
const KEY = 'portfolio.view'
function restore(): { levels: LevelFilter[]; sort: Sort } {
  try {
    const v = JSON.parse(localStorage.getItem(KEY) ?? 'null')
    if (v && Array.isArray(v.levels) && v.sort?.key) return v
  } catch {
    // nothing remembered
  }
  return { levels: [], sort: DEFAULT_SORT }
}
const saved = restore()
const chosen = ref<LevelFilter[]>(saved.levels)
const sort = ref<Sort>(saved.sort)
watch([chosen, sort], () => {
  try {
    localStorage.setItem(KEY, JSON.stringify({ levels: chosen.value, sort: sort.value }))
  } catch {
    // storage unavailable
  }
})
const options: LevelFilter[] = [...LEVELS, 'none']
const optionName = (l: LevelFilter) => (l === 'none' ? t('schedule.noPlan') : levels.name(l))
function toggle(l: LevelFilter) {
  chosen.value = chosen.value.includes(l) ? chosen.value.filter((x) => x !== l) : [...chosen.value, l]
}

// Cards with no row left under the filter are hidden (RF-33, RF-34).
const cards = computed(() => {
  const d = data.value
  if (!d) return []
  const all: PortfolioCard[] = [...d.programs, ...(d.loose.rows.length ? [d.loose] : [])]
  return all
    .map((c) => ({ ...c, rows: sortRows(filterRows(c.rows, chosen.value), sort.value) }))
    .filter((c) => !chosen.value.length || c.rows.length || (c.program && chosen.value.includes(levelOf(c.program))))
})
const today = computed(() => (data.value ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'long' }).format(new Date(data.value.today + 'T00:00:00')) : ''))
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="portfolio">
    <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1">
      <h1 class="font-heading text-xl font-semibold">{{ t('portfolio.title') }}</h1>
      <p v-if="data" class="text-sm text-muted-foreground" data-testid="portfolio-today">{{ t('portfolio.today', { date: today }) }}</p>
      <Button v-if="canManage && !creating" size="sm" class="ml-auto self-center" data-testid="portfolio-new" @click="creating = true">
        <Plus class="size-4" aria-hidden="true" />{{ t('portfolio.new.action') }}
      </Button>
    </div>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <NewProgram v-if="creating" @created="created" @cancel="creating = false" />
    <Alert v-if="move.error.value" variant="destructive" role="alert" data-testid="portfolio-move-error">
      <AlertDescription>{{ errorMessage(move.error.value) }}</AlertDescription>
    </Alert>

    <div class="flex flex-wrap items-center gap-2" role="group" :aria-label="t('portfolio.filter')">
      <span class="text-sm text-muted-foreground">{{ t('portfolio.filter') }}</span>
      <button
        v-for="l in options"
        :key="l"
        type="button"
        class="rounded-full border px-3 py-1 text-xs font-medium"
        :class="chosen.includes(l) ? (l === 'none' ? 'bg-muted' : levels.classes(l).badge) + ' border-transparent' : 'text-muted-foreground hover:bg-muted'"
        :aria-pressed="chosen.includes(l)"
        :data-testid="`portfolio-filter-${l}`"
        @click="toggle(l)"
      >
        {{ optionName(l) }}
      </button>
      <Button v-if="chosen.length" size="sm" variant="ghost" data-testid="portfolio-clear" @click="chosen = []">{{ t('portfolio.clear') }}</Button>
    </div>

    <template v-if="data">
      <div v-if="!cards.length" class="flex flex-col items-center gap-2 rounded-lg border border-dashed p-6 text-sm text-muted-foreground" data-testid="portfolio-empty">
        {{ chosen.length ? t('portfolio.noMatch') : t('portfolio.empty') }}
        <Button v-if="chosen.length" size="sm" variant="outline" @click="chosen = []">{{ t('portfolio.clear') }}</Button>
      </div>
      <section
        v-for="c in cards"
        :key="c.program?.id ?? 'loose'"
        class="flex flex-col gap-3 rounded-xl border bg-background p-4"
        :aria-label="c.program?.name ?? t('portfolio.loose')"
        data-testid="portfolio-card"
      >
        <div class="flex flex-wrap items-start gap-x-4 gap-y-2">
          <div class="min-w-0">
            <h2 class="text-base font-semibold">
              <RouterLink v-if="c.program" :to="`/projects/${c.program.id}`" class="hover:underline" data-testid="portfolio-program-link">{{ c.program.name }}</RouterLink>
              <template v-else>{{ t('portfolio.loose') }}</template>
            </h2>
            <p class="text-xs text-muted-foreground" data-testid="portfolio-counts">
              {{ t('portfolio.projects', { count: c.projects }, c.projects) }} · {{ t('portfolio.belowPlan', { count: c.below_plan }, c.below_plan) }}
            </p>
          </div>
          <div v-if="c.program" class="ml-auto flex flex-col items-end gap-1">
            <span class="text-2xl font-bold tabular-nums">{{ c.program.figures.real.toLocaleString('es', { maximumFractionDigits: 1 }) }} %</span>
            <LevelBadge v-if="c.program.figures.has_plan" :figures="c.program.figures" />
            <span v-else class="text-xs text-muted-foreground">{{ t('schedule.noPlan') }}</span>
          </div>
        </div>
        <ProgramRows
          v-if="c.rows.length"
          :rows="c.rows"
          :sort="sort"
          :removable="canManage && !!c.program"
          :busy="move.isPending.value"
          @sort="sort = $event"
          @remove="(r) => move.mutate({ id: r.id, parentId: null })"
        />
        <p v-else class="text-sm text-muted-foreground">{{ t('portfolio.noRows') }}</p>
        <AddProject
          v-if="canManage && c.program && c.program.lifecycle === 'activo'"
          :program-id="c.program.id"
          :exclude="c.rows.map((r) => r.id)"
          :disabled="move.isPending.value"
          @pick="(id) => move.mutate({ id, parentId: c.program!.id })"
        />
      </section>
    </template>
  </div>
</template>
