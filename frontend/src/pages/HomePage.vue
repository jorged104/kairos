<script setup lang="ts">
import { useQuery } from '@tanstack/vue-query'
import { useQueryClient } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import { Plus } from '@lucide/vue'
import { getDashboard, getDashboardPeople } from '@/api/dashboard'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useErrorMessage } from '@/composables/useErrorMessage'
import MyKanban from '@/features/dashboard/MyKanban.vue'
import MyProjects from '@/features/dashboard/MyProjects.vue'
import ProfileCard from '@/features/dashboard/ProfileCard.vue'
import ProgressCharts from '@/features/dashboard/ProgressCharts.vue'
import ClaimableTasks from '@/features/loose/ClaimableTasks.vue'
import NewTaskDialog from '@/features/loose/NewTaskDialog.vue'
import { useCatalog } from '@/features/work/useCatalog'
import ItemPanel from '@/features/workitems/ItemPanel.vue'
import { useAuthStore } from '@/stores/auth'

// Mi panel: the caller's profile, work items, projects and progress, at their
// date today (spec 013). ?user=<id> shows another person's, read only, to an
// admin or a lead (change of 2026-11-06, RF-37 to RF-51).
const { t, locale } = useI18n()
const errorMessage = useErrorMessage()
const { catalog } = useCatalog()
const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const userId = computed(() => {
  const u = route.query.user
  return typeof u === 'string' && u && u !== auth.me?.id ? u : undefined
})
// A panel that cannot be seen answers 404 at once, without retrying (RF-48).
const { data, error } = useQuery({
  queryKey: computed(() => ['dashboard', userId.value ?? 'me']),
  queryFn: () => getDashboard(userId.value),
  retry: (count) => !userId.value && count < 3,
})
const canSeeOthers = computed(() => auth.can('dashboard:others:all', 'dashboard:others:team'))
const { data: people } = useQuery({ queryKey: ['dashboard', 'people'], queryFn: getDashboardPeople, enabled: canSeeOthers })
const owner = computed(() => data.value?.user?.name)
function choose(id: string | undefined) {
  router.replace({ query: { ...route.query, user: id || undefined, item: undefined } })
}
// The work item opened from the kanban, in the side panel (RF-15): what
// changed there shows when it closes.
const client = useQueryClient()
watch(
  () => route.query.item,
  (item, before) => before && !item && client.invalidateQueries({ queryKey: ['dashboard'] }),
)
// Tasks without a project, on one's own panel (spec 022, RF-1, RF-18).
const canCreate = computed(() => auth.can('loose:create:own'))
const creating = ref(false)
const today = computed(() => (data.value ? new Intl.DateTimeFormat(locale.value, { dateStyle: 'long' }).format(new Date(data.value.today + 'T00:00:00')) : ''))
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="home">
    <div class="flex flex-wrap items-center gap-x-4 gap-y-2">
      <h1 class="font-heading text-xl font-semibold">{{ owner ? t('dashboard.others.title', { name: owner }) : t('dashboard.title') }}</h1>
      <p v-if="data" class="text-sm text-muted-foreground">{{ t('dashboard.today', { date: today }) }}</p>
      <Button v-if="canCreate && !userId" size="sm" class="ml-auto" data-testid="new-loose" @click="creating = true">
        <Plus aria-hidden="true" />{{ t('loose.newTask') }}
      </Button>
      <label v-if="people?.length" class="flex items-center gap-2 text-sm" :class="!(canCreate && !userId) && 'ml-auto'">
        <span class="text-muted-foreground">{{ t('dashboard.others.select') }}</span>
        <NativeSelect :model-value="userId ?? ''" class="h-8 w-56" data-testid="dashboard-person" @update:model-value="(v) => choose(v)">
          <option value="">{{ t('dashboard.others.me') }}</option>
          <option v-for="p in people" :key="p.id" :value="p.id">{{ p.name }}</option>
        </NativeSelect>
      </label>
    </div>
    <Alert v-if="owner" role="status" data-testid="dashboard-other">
      <AlertDescription class="flex flex-wrap items-center gap-x-3">
        {{ t('dashboard.others.viewing', { name: owner }) }}
        <RouterLink :to="{ query: {} }" class="font-medium underline">{{ t('dashboard.others.back') }}</RouterLink>
      </AlertDescription>
    </Alert>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription class="flex flex-wrap items-center gap-x-3">
        {{ userId ? t('dashboard.others.notAvailable') : errorMessage(error) }}
        <RouterLink v-if="userId" :to="{ query: {} }" class="font-medium underline" data-testid="dashboard-back">{{ t('dashboard.others.back') }}</RouterLink>
      </AlertDescription>
    </Alert>
    <ClaimableTasks v-if="canCreate && !userId" />
    <template v-if="data">
      <div class="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
        <div class="flex flex-col gap-4">
          <ProfileCard :profile="data.profile" :user="data.user" />
          <MyProjects :projects="data.projects" :owner="owner" />
        </div>
        <div class="flex min-w-0 flex-col gap-4">
          <MyKanban :tasks="data.tasks" :statuses="catalog.statuses" :types="catalog.work_item_types" :owner="owner" />
        </div>
      </div>
      <ProgressCharts :weeks="data.weeks" :load="data.load" :owner="owner" />
    </template>
    <ItemPanel />
    <NewTaskDialog v-model="creating" />
  </div>
</template>
