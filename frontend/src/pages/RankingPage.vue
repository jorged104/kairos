<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getRanking, type RankingPeriod } from '@/api/gamification'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { useAuthStore } from '@/stores/auth'

// The ranking of each team the person belongs to directly, by the XP of a
// period (spec 014, RF-43 to RF-52).
const { t, n } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const teams = computed(() => auth.me?.teams ?? [])
const teamId = ref(teams.value[0]?.id ?? '')
watch(teams, (ts) => {
  if (!ts.some((x) => x.id === teamId.value)) teamId.value = ts[0]?.id ?? ''
})
const period = ref<RankingPeriod>('month') // RF-48
const periods: RankingPeriod[] = ['week', 'month', 'all']

const { data, error } = useQuery({
  queryKey: computed(() => ['ranking', teamId.value, period.value]),
  queryFn: () => getRanking(teamId.value, period.value),
  enabled: computed(() => !!teamId.value),
})
const range = computed(() => {
  const r = data.value
  if (!r?.from || !r.to) return ''
  const f = new Intl.DateTimeFormat('es', { day: 'numeric', month: 'short', timeZone: 'UTC' })
  return t('gamification.ranking.range', { from: f.format(new Date(r.from + 'T00:00:00Z')), to: f.format(new Date(r.to + 'T00:00:00Z')) })
})
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="ranking">
    <h1 class="font-heading text-xl font-semibold">{{ t('gamification.ranking.title') }}</h1>
    <p v-if="!teams.length" class="text-sm text-muted-foreground" data-testid="ranking-no-team">{{ t('gamification.ranking.noTeam') }}</p>
    <template v-else>
      <div class="flex flex-wrap items-end gap-3">
        <label class="flex flex-col gap-1 text-xs text-muted-foreground">
          {{ t('gamification.ranking.team') }}
          <NativeSelect v-model="teamId" class="w-56" data-testid="ranking-team">
            <option v-for="tm in teams" :key="tm.id" :value="tm.id">{{ tm.name }}</option>
          </NativeSelect>
        </label>
        <div class="flex rounded-lg border p-0.5" role="group" :aria-label="t('gamification.ranking.period')">
          <button
            v-for="p in periods"
            :key="p"
            type="button"
            class="rounded-md px-3 py-1 text-sm"
            :class="p === period ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
            :aria-pressed="p === period"
            :data-testid="`ranking-period-${p}`"
            @click="period = p"
          >
            {{ t(`gamification.ranking.periods.${p}`) }}
          </button>
        </div>
        <span v-if="range" class="text-xs text-muted-foreground" data-testid="ranking-range">{{ range }}</span>
      </div>
      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>
      <ol v-if="data" class="flex flex-col divide-y rounded-xl border bg-background">
        <li
          v-for="r in data.items"
          :key="r.user.id"
          class="flex items-center gap-3 px-4 py-2.5"
          :class="r.user.id === auth.me?.id && 'bg-primary/5'"
          data-testid="ranking-row"
        >
          <span class="w-6 text-right text-sm font-semibold tabular-nums text-muted-foreground" data-testid="ranking-position">{{ r.position }}</span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium">
              {{ r.user.name }}<template v-if="r.user.id === auth.me?.id"> · {{ t('gamification.ranking.you') }}</template>
            </span>
            <span class="block text-xs text-muted-foreground">{{ t('gamification.level', { number: r.level.number, name: r.level.name }) }}</span>
          </span>
          <span class="text-sm font-semibold tabular-nums" data-testid="ranking-xp">{{ t('gamification.xp', { n: n(r.xp) }) }}</span>
        </li>
      </ol>
    </template>
  </div>
</template>
