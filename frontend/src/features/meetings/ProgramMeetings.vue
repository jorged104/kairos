<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { CalendarClock } from '@lucide/vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { listMeetings } from '@/api/meetings'

// The next 3 meetings of a program in the portfolio, its own only (spec 018,
// RF-43, RF-44).
const props = defineProps<{ projectId: string }>()
const { t, d } = useI18n()
const { data } = useQuery({
  queryKey: computed(() => ['meetings', props.projectId, 'upcoming', 'portfolio']),
  queryFn: () => listMeetings(props.projectId, 'upcoming', 1, 3),
})
</script>

<template>
  <div v-if="data" class="flex flex-col gap-1 text-sm" data-testid="program-meetings">
    <h3 class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">{{ t('meetings.next') }}</h3>
    <p v-if="!data.items.length" class="text-muted-foreground">{{ t('meetings.emptyUpcoming') }}</p>
    <ul v-else class="flex flex-col gap-0.5">
      <li v-for="m in data.items" :key="m.id" class="flex items-center gap-2" data-testid="program-meeting">
        <CalendarClock class="size-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        <span class="shrink-0 text-muted-foreground tabular-nums">{{ d(m.starts_at, 'long') }}</span>
        <RouterLink :to="`/meetings/${m.id}`" class="truncate hover:underline">{{ m.title }}</RouterLink>
      </li>
    </ul>
    <RouterLink :to="`/projects/${projectId}?tab=meetings`" class="w-fit text-xs text-primary hover:underline" data-testid="program-meetings-all">
      {{ t('meetings.seeAll') }}
    </RouterLink>
  </div>
</template>
