<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { listProjects } from '@/api/work'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableEmpty, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { healthVariant, lifecycleVariant } from '@/features/work/Badges'
import { usePeople } from '@/features/work/usePeople'
import { useAuthStore } from '@/stores/auth'
import { HEALTHS, LIFECYCLES } from '@/types/work'

// Project list with search and filters (RF-128).
const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()
const { teams } = usePeople()

const search = ref('')
const debounced = refDebounced(search, 300)
const teamId = ref('')
const lifecycle = ref('')
const health = ref('')
const page = ref(1)
watch([debounced, teamId, lifecycle, health], () => (page.value = 1))

const query = computed(() => ({
  q: debounced.value,
  team_id: teamId.value,
  lifecycle: lifecycle.value,
  health: health.value,
  page: page.value,
}))
const { data, error } = useQuery({
  queryKey: ['projects', query],
  queryFn: () => listProjects(query.value),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2">
      <h1 class="font-heading text-xl font-semibold">{{ t('projects.title') }}</h1>
      <Button v-if="auth.can('projects:create:all', 'projects:create:team')" as-child>
        <RouterLink to="/projects/new">{{ t('projects.new') }}</RouterLink>
      </Button>
    </div>

    <div class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="flex flex-col gap-1.5">
        <Label for="project-search">{{ t('common.search') }}</Label>
        <Input id="project-search" v-model="search" :placeholder="t('projects.searchPlaceholder')" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-team">{{ t('projects.team') }}</Label>
        <NativeSelect id="project-team" v-model="teamId">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="{ team, depth } in teams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-lifecycle">{{ t('projects.lifecycle') }}</Label>
        <NativeSelect id="project-lifecycle" v-model="lifecycle">
          <option value="">{{ t('projects.lifecycleDefault') }}</option>
          <option v-for="l in LIFECYCLES" :key="l" :value="l">{{ t(`projects.lifecycles.${l}`) }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="project-health">{{ t('projects.health') }}</Label>
        <NativeSelect id="project-health" v-model="health">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="h in HEALTHS" :key="h" :value="h">{{ t(`projects.healths.${h}`) }}</option>
        </NativeSelect>
      </div>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="rounded-lg border bg-background">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('projects.identifier') }}</TableHead>
            <TableHead>{{ t('projects.name') }}</TableHead>
            <TableHead>{{ t('projects.team') }}</TableHead>
            <TableHead>{{ t('projects.owner') }}</TableHead>
            <TableHead>{{ t('projects.health') }}</TableHead>
            <TableHead>{{ t('projects.lifecycle') }}</TableHead>
            <TableHead>{{ t('projects.endDate') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="p in data?.items ?? []" :key="p.id" data-testid="project-row">
            <TableCell class="font-mono text-xs">{{ p.identifier }}</TableCell>
            <TableCell>
              <RouterLink :to="`/projects/${p.id}`" class="font-medium hover:underline">{{ p.name }}</RouterLink>
              <span v-if="p.methodology" class="ml-2 text-xs text-muted-foreground">{{ p.methodology.name }}</span>
            </TableCell>
            <TableCell>{{ p.team.name }}</TableCell>
            <TableCell>{{ p.owner.name }}</TableCell>
            <TableCell><Badge :variant="healthVariant[p.health]">{{ t(`projects.healths.${p.health}`) }}</Badge></TableCell>
            <TableCell><Badge :variant="lifecycleVariant[p.lifecycle]">{{ t(`projects.lifecycles.${p.lifecycle}`) }}</Badge></TableCell>
            <TableCell class="whitespace-nowrap">{{ p.end_date ?? t('common.none') }}</TableCell>
          </TableRow>
          <TableEmpty v-if="data && data.items.length === 0" :colspan="7">{{ t('common.empty') }}</TableEmpty>
        </TableBody>
      </Table>
    </div>

    <div v-if="data" class="flex items-center justify-between text-sm text-muted-foreground">
      <span>{{ t('common.pageOf', { page: data.page, pages, total: data.total }) }}</span>
      <div class="flex gap-2">
        <Button variant="outline" size="sm" :disabled="page <= 1" @click="page--">{{ t('common.previous') }}</Button>
        <Button variant="outline" size="sm" :disabled="page >= pages" @click="page++">{{ t('common.next') }}</Button>
      </div>
    </div>
  </div>
</template>
