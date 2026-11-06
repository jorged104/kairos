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
// period (spec 014, RF-43 to RF-52): the caller's position, a podium of the
// first three and the full standings.
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

const items = computed(() => data.value?.items ?? [])
const isMe = (id: string) => id === auth.me?.id
const initials = (name: string) =>
  name
    .split(/\s+/)
    .map((w) => w[0] ?? '')
    .join('')
    .slice(0, 2)
// Bars are relative to the leader; a period can end below zero (RF-49).
const top = computed(() => Math.max(0, items.value[0]?.xp ?? 0))
const bar = (xp: number) => (top.value > 0 ? `${Math.max(0, (100 * xp) / top.value)}%` : '0%')

// Second, first, third: the leader stands in the middle.
const podium = computed(() =>
  [1, 0, 2].flatMap((i) => {
    const r = items.value[i]
    return r ? [{ ...r, first: i === 0, step: ['h-20', 'h-14', 'h-11'][i] }] : []
  }),
)

const mine = computed(() => {
  const i = items.value.findIndex((r) => isMe(r.user.id))
  if (i < 0) return null
  const me = items.value[i]!
  let gap: string
  if (i === 0) {
    const second = items.value[1]
    gap = second
      ? t('gamification.ranking.leading', { xp: n(me.xp - second.xp), name: second.user.name })
      : t('gamification.ranking.leadingAlone')
  } else {
    const above = items.value[i - 1]!
    gap =
      above.xp === me.xp
        ? t('gamification.ranking.tied', { name: above.user.name })
        : t('gamification.ranking.behind', { xp: n(above.xp - me.xp), name: above.user.name })
  }
  return { ...me, gap }
})
</script>

<template>
  <div class="mx-auto flex w-full max-w-4xl flex-col gap-5" data-testid="ranking">
    <div class="flex flex-wrap items-end justify-between gap-4">
      <div class="flex flex-col gap-1">
        <h1 class="font-heading text-2xl font-bold tracking-tight">{{ t('gamification.ranking.title') }}</h1>
        <p v-if="teams.length" class="text-sm text-muted-foreground">
          <span v-if="range" data-testid="ranking-range">{{ range }}</span>
          <span v-else>{{ t('gamification.ranking.always') }}</span>
          · {{ t('gamification.ranking.subtitle') }}
        </p>
      </div>
      <div v-if="teams.length" class="flex flex-wrap items-center gap-2">
        <label class="flex items-center gap-1.5 text-sm text-muted-foreground">
          {{ t('gamification.ranking.team') }}
          <NativeSelect v-model="teamId" class="w-48" data-testid="ranking-team">
            <option v-for="tm in teams" :key="tm.id" :value="tm.id">{{ tm.name }}</option>
          </NativeSelect>
        </label>
        <div class="flex rounded-lg border bg-background p-0.5" role="group" :aria-label="t('gamification.ranking.period')">
          <button
            v-for="p in periods"
            :key="p"
            type="button"
            class="rounded-md px-3.5 py-1.5 text-sm font-medium transition-colors"
            :class="p === period ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
            :aria-pressed="p === period"
            :data-testid="`ranking-period-${p}`"
            @click="period = p"
          >
            {{ t(`gamification.ranking.periods.${p}`) }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="!teams.length" class="text-sm text-muted-foreground" data-testid="ranking-no-team">{{ t('gamification.ranking.noTeam') }}</p>
    <template v-else>
      <Alert v-if="error" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
      </Alert>

      <section
        v-if="mine"
        class="flex flex-wrap items-center gap-x-8 gap-y-4 rounded-2xl bg-primary px-6 py-5 text-primary-foreground"
        :aria-label="t('gamification.ranking.mine')"
        data-testid="ranking-mine"
      >
        <div class="flex items-center gap-4">
          <span class="grid size-16 place-items-center rounded-full border-[3px] border-primary-foreground/40 text-2xl font-bold tabular-nums" aria-hidden="true">
            {{ mine.position }}.º
          </span>
          <div class="flex flex-col">
            <span class="text-sm opacity-80">{{ t('gamification.ranking.mine') }}</span>
            <span class="text-xl font-bold tabular-nums">{{ t('gamification.ranking.positionOf', { position: mine.position, total: items.length }) }}</span>
          </div>
        </div>
        <div class="flex flex-col">
          <span class="text-sm opacity-80">{{ t('gamification.ranking.myXp') }}</span>
          <span class="text-xl font-bold tabular-nums">{{ t('gamification.xp', { n: n(mine.xp) }) }}</span>
        </div>
        <p class="min-w-60 flex-1 text-sm" data-testid="ranking-gap">{{ mine.gap }}</p>
      </section>

      <section v-if="items.length > 1" class="rounded-2xl border bg-background px-6 pt-6" :aria-label="t('gamification.ranking.podium')">
        <ol class="mx-auto grid max-w-xl grid-cols-3 items-end gap-3">
          <li v-for="r in podium" :key="r.user.id" class="flex min-w-0 flex-col items-center gap-1 text-center" data-testid="ranking-podium">
            <span
              class="grid place-items-center rounded-full font-bold"
              :class="[
                r.first ? 'size-16 text-xl ring-2 ring-primary ring-offset-2 ring-offset-background' : 'size-13 text-base',
                isMe(r.user.id) ? 'bg-primary text-primary-foreground' : 'bg-muted text-foreground',
              ]"
              aria-hidden="true"
            >
              {{ initials(r.user.name) }}
            </span>
            <span class="mt-1 max-w-full truncate text-sm font-semibold">{{ r.user.name }}</span>
            <span class="text-xs text-muted-foreground">{{ t('gamification.level', { number: r.level.number, name: r.level.name }) }}</span>
            <span class="mb-1 text-sm font-semibold tabular-nums">{{ t('gamification.xp', { n: n(r.xp) }) }}</span>
            <span
              class="flex w-full justify-center rounded-t-lg pt-2.5 text-xl font-bold tabular-nums"
              :class="[r.step, r.first ? 'bg-primary text-primary-foreground' : 'bg-muted text-foreground']"
            >
              {{ r.position }}
            </span>
          </li>
        </ol>
      </section>

      <section v-if="data" class="overflow-hidden rounded-2xl border bg-background" :aria-label="t('gamification.ranking.standings')">
        <div class="flex items-baseline justify-between gap-2 px-5 py-4">
          <h2 class="text-base font-semibold">{{ t('gamification.ranking.standings') }}</h2>
          <span class="text-sm text-muted-foreground">{{ t('gamification.ranking.people', { n: items.length }, items.length) }}</span>
        </div>
        <ol>
          <li
            v-for="r in items"
            :key="r.user.id"
            class="flex flex-wrap items-center gap-x-3.5 gap-y-2 border-t px-5 py-3"
            :class="isMe(r.user.id) && 'bg-primary/5'"
            data-testid="ranking-row"
          >
            <span class="w-7 text-right text-sm font-bold tabular-nums text-muted-foreground" data-testid="ranking-position">{{ r.position }}</span>
            <span
              class="grid size-9 shrink-0 place-items-center rounded-full text-xs font-semibold"
              :class="isMe(r.user.id) ? 'bg-primary text-primary-foreground' : 'bg-muted text-foreground'"
              aria-hidden="true"
            >
              {{ initials(r.user.name) }}
            </span>
            <span class="flex min-w-0 flex-[1_1_11rem] flex-col">
              <span class="truncate text-sm font-semibold">
                {{ r.user.name }}<template v-if="isMe(r.user.id)"> · {{ t('gamification.ranking.you') }}</template>
              </span>
              <span class="text-xs text-muted-foreground">{{ t('gamification.level', { number: r.level.number, name: r.level.name }) }}</span>
            </span>
            <span class="flex min-w-32 flex-[1_1_12rem] items-center gap-3">
              <span class="h-2 flex-1 overflow-hidden rounded-full bg-muted" aria-hidden="true">
                <span
                  class="block h-full rounded-full transition-[width] duration-500"
                  :class="isMe(r.user.id) ? 'bg-primary' : 'bg-muted-foreground/50'"
                  :style="{ width: bar(r.xp) }"
                />
              </span>
              <span class="w-20 text-right text-sm font-semibold tabular-nums" data-testid="ranking-xp">{{ t('gamification.xp', { n: n(r.xp) }) }}</span>
            </span>
          </li>
        </ol>
      </section>
    </template>
  </div>
</template>
