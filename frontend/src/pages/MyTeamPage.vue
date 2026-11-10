<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { getMyTeam, type TeamMember } from '@/api/workload'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import PersonSchedule from '@/features/workload/PersonSchedule.vue'
import { useAuthStore } from '@/stores/auth'

// Mi equipo (spec 021, RF-19 to RF-27): the people whose panel an admin or a
// lead may see, their load and their schedule over every project.
const { t, locale } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const allowed = computed(() => auth.can('dashboard:others:all', 'dashboard:others:team'))
const { data, error } = useQuery({ queryKey: ['my-team'], queryFn: getMyTeam, enabled: allowed })

// By team when there is more than one (RF-26).
const team = ref('')
const people = computed<TeamMember[]>(() => (data.value?.people ?? []).filter((p) => !team.value || p.teams.some((tm) => tm.id === team.value)))
const byName = computed(() => [...people.value].sort((a, b) => a.user.name.localeCompare(b.user.name, locale.value)))

const dateFmt = computed(() => new Intl.DateTimeFormat(locale.value, { day: 'numeric', month: 'short', timeZone: 'UTC' }))
const free = (p: TeamMember) => (p.free_from ? t('workload.freeFrom', { date: dateFmt.value.format(new Date(p.free_from + 'T00:00:00Z')) }) : t('workload.free'))
// Everybody listed is somebody whose panel the viewer may open (RF-14).
const linkable = () => true
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="my-team">
    <div class="flex flex-col gap-1">
      <h1 class="font-heading text-xl font-semibold">{{ t('myTeam.title') }}</h1>
      <p class="text-sm text-muted-foreground">{{ t('myTeam.subtitle') }}</p>
    </div>

    <Alert v-if="!allowed" role="alert" data-testid="my-team-forbidden">
      <AlertDescription>{{ t('myTeam.forbidden') }}</AlertDescription>
    </Alert>
    <Alert v-else-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <template v-else-if="data">
      <p v-if="!data.people.length" class="rounded-lg bg-muted p-4 text-center text-sm" data-testid="my-team-empty">{{ t('myTeam.empty') }}</p>
      <template v-else>
        <label v-if="data.teams.length > 1" class="flex max-w-xs items-center gap-2 text-sm">
          <span class="shrink-0 text-muted-foreground">{{ t('myTeam.team') }}</span>
          <NativeSelect v-model="team" data-testid="my-team-filter">
            <option value="">{{ t('myTeam.allTeams') }}</option>
            <option v-for="tm in data.teams" :key="tm.id" :value="tm.id">{{ tm.name }}</option>
          </NativeSelect>
        </label>

        <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="my-team-people">
          <h2 id="my-team-people" class="text-base font-semibold">{{ t('myTeam.people') }} <span class="font-normal text-muted-foreground">· {{ people.length }}</span></h2>
          <div class="overflow-x-auto">
            <table class="w-full min-w-[720px] text-sm">
              <thead>
                <tr class="text-left text-xs text-muted-foreground">
                  <th class="py-2 pr-2 font-medium">{{ t('workload.person') }}</th>
                  <th class="p-2 font-medium">{{ t('myTeam.teams') }}</th>
                  <th class="w-28 p-2 font-medium">{{ t('myTeam.capacity') }}</th>
                  <th class="w-36 p-2 font-medium">{{ t('myTeam.load') }}</th>
                  <th class="w-24 p-2 font-medium">{{ t('myTeam.open') }}</th>
                  <th class="w-44 p-2 font-medium">{{ t('myTeam.free') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="p in byName" :key="p.user.id" class="border-t" data-testid="my-team-person">
                  <td class="py-2 pr-2">
                    <RouterLink :to="{ path: '/', query: { user: p.user.id } }" class="font-medium hover:underline" data-testid="my-team-link">{{ p.user.name }}</RouterLink>
                  </td>
                  <td class="p-2 text-muted-foreground">{{ p.teams.map((tm) => tm.name).join(', ') || t('common.none') }}</td>
                  <td class="p-2 text-muted-foreground">{{ p.capacity ?? t('myTeam.noCapacity') }}</td>
                  <td class="p-2">
                    <span :class="p.overloaded ? 'font-semibold text-orange-700 dark:text-orange-400' : 'text-muted-foreground'" data-testid="my-team-load">
                      {{ p.capacity === null ? p.weight : `${p.weight} / ${p.capacity}` }}<template v-if="p.overloaded"> · {{ t('myTeam.overloaded') }}</template>
                    </span>
                  </td>
                  <td class="p-2 text-muted-foreground">{{ p.open }}</td>
                  <td class="p-2" :class="!p.free_from && 'text-emerald-700 dark:text-emerald-400'">{{ free(p) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </section>

        <section class="flex flex-col gap-3 rounded-xl border bg-background p-4" aria-labelledby="my-team-schedule">
          <div class="flex flex-col gap-1">
            <h2 id="my-team-schedule" class="text-base font-semibold">{{ t('myTeam.schedule') }}</h2>
            <p class="text-sm text-muted-foreground">{{ t('myTeam.scheduleHint') }}</p>
          </div>
          <PersonSchedule :rows="people" :today="data.today" :linkable="linkable" by-project />
        </section>
      </template>
    </template>
  </div>
</template>
