<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getDashboard } from '@/api/dashboard'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { useErrorMessage } from '@/composables/useErrorMessage'
import MyKanban from '@/features/dashboard/MyKanban.vue'
import MyProjects from '@/features/dashboard/MyProjects.vue'
import ProfileCard from '@/features/dashboard/ProfileCard.vue'
import ProgressCharts from '@/features/dashboard/ProgressCharts.vue'
import { useCatalog } from '@/features/work/useCatalog'

// Mi panel: the caller's profile, work items, projects and progress, at their
// date today (spec 013).
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()
const { data, error } = useQuery({ queryKey: ['dashboard'], queryFn: getDashboard })
const today = computed(() => (data.value ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'long' }).format(new Date(data.value.today + 'T00:00:00')) : ''))
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="home">
    <div class="flex flex-wrap items-baseline gap-x-4 gap-y-1">
      <h1 class="font-heading text-xl font-semibold">{{ t('dashboard.title') }}</h1>
      <p v-if="data" class="text-sm text-muted-foreground">{{ t('dashboard.today', { date: today }) }}</p>
    </div>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>
    <template v-if="data">
      <div class="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <div class="flex flex-col gap-4">
          <ProfileCard :profile="data.profile" />
          <MyProjects :projects="data.projects" />
        </div>
        <div class="flex min-w-0 flex-col gap-4">
          <MyKanban :tasks="data.tasks" :statuses="catalog.statuses" />
        </div>
      </div>
      <ProgressCharts :weeks="data.weeks" :load="data.load" />
    </template>
  </div>
</template>
