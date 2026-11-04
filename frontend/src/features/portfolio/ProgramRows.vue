<script setup lang="ts">
import { ArrowDown, ArrowUp, X } from '@lucide/vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import type { PortfolioRow } from '@/api/portfolio'
import LevelBadge from '@/features/schedule/LevelBadge.vue'
import { signed } from '@/features/schedule/useLevels'
import type { Sort, SortKey } from './rows'

// The rows of a card: each project with a bar of its real progress marking
// its plan, and the figures of the traffic light (spec 010, RF-25 to RF-27,
// RF-30, RF-35), and taking each out of its program (RF-46).
defineProps<{ rows: PortfolioRow[]; sort: Sort; removable?: boolean; busy?: boolean }>()
const emit = defineEmits<{ sort: [Sort]; remove: [PortfolioRow] }>()
const { t } = useI18n()
const pct = (n: number) => `${n.toLocaleString('es', { maximumFractionDigits: 1 })} %`
const columns: SortKey[] = ['plan', 'real', 'gap']

function by(sort: Sort, key: SortKey) {
  emit('sort', { key, dir: sort.key === key && sort.dir === 'asc' ? 'desc' : 'asc' })
}
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full min-w-[640px] text-sm">
      <thead>
        <tr class="text-left text-xs text-muted-foreground">
          <th class="py-2 pr-2 font-medium" :aria-sort="sort.key === 'name' ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'">
            <button type="button" class="inline-flex items-center gap-1 hover:text-foreground" data-testid="sort-name" @click="by(sort, 'name')">
              {{ t('portfolio.project') }}
              <component :is="sort.dir === 'asc' ? ArrowUp : ArrowDown" v-if="sort.key === 'name'" class="size-3" aria-hidden="true" />
            </button>
          </th>
          <th v-for="c in columns" :key="c" class="p-2 text-right font-medium" :aria-sort="sort.key === c ? (sort.dir === 'asc' ? 'ascending' : 'descending') : 'none'">
            <button type="button" class="inline-flex items-center gap-1 hover:text-foreground" :data-testid="`sort-${c}`" @click="by(sort, c)">
              {{ t(`portfolio.columns.${c}`) }}
              <component :is="sort.dir === 'asc' ? ArrowUp : ArrowDown" v-if="sort.key === c" class="size-3" aria-hidden="true" />
            </button>
          </th>
          <th class="p-2 font-medium">{{ t('portfolio.columns.level') }}</th>
          <th v-if="removable" class="w-8 p-2"><span class="sr-only">{{ t('portfolio.remove.column') }}</span></th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rows" :key="r.id" class="border-t" data-testid="portfolio-row">
          <td class="py-2 pr-2">
            <RouterLink :to="`/projects/${r.id}`" class="font-medium hover:underline" data-testid="portfolio-row-link">{{ r.name }}</RouterLink>
            <span v-if="r.is_program" class="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{{ t('portfolio.program') }}</span>
            <span v-if="r.lifecycle === 'cerrado'" class="ml-2 text-xs text-muted-foreground">{{ t('projects.lifecycles.cerrado') }}</span>
            <!-- The real progress with the plan marked on it (RF-26). -->
            <div class="relative mt-1.5 h-1.5 w-full max-w-72 rounded-full bg-muted" aria-hidden="true">
              <div class="h-full rounded-full bg-primary" :style="{ width: `${r.figures.real}%` }" />
              <div v-if="r.figures.has_plan" class="absolute -top-0.5 h-2.5 w-0.5 bg-foreground" :style="{ left: `${r.figures.plan}%` }" />
            </div>
          </td>
          <td class="p-2 text-right tabular-nums text-muted-foreground">{{ r.figures.has_plan ? pct(r.figures.plan) : '—' }}</td>
          <td class="p-2 text-right font-medium tabular-nums">{{ pct(r.figures.real) }}</td>
          <td class="p-2 text-right tabular-nums">{{ r.figures.has_plan ? t('schedule.gap', { gap: signed(r.figures.gap) }) : '—' }}</td>
          <td class="p-2">
            <LevelBadge v-if="r.figures.has_plan" :figures="r.figures" compact />
            <span v-else class="text-xs text-muted-foreground">{{ t('schedule.noPlan') }}</span>
          </td>
          <td v-if="removable" class="p-2 text-right">
            <button
              type="button"
              class="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground disabled:opacity-50"
              :title="t('portfolio.remove.action', { name: r.name })"
              :aria-label="t('portfolio.remove.action', { name: r.name })"
              :disabled="busy"
              data-testid="portfolio-remove"
              @click="emit('remove', r)"
            >
              <X class="size-4" aria-hidden="true" />
            </button>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
