<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { refDebounced } from '@vueuse/core'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { listAttributes, listTeams, listUsers } from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableEmpty, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useErrorMessage } from '@/composables/useErrorMessage'
import AssignmentBadge from '@/features/users/AssignmentBadge.vue'
import { flatten } from '@/features/teams/tree'
import { useAuthStore } from '@/stores/auth'
import { ROLES } from '@/types/api'

const { t } = useI18n()
const auth = useAuthStore()
const errorMessage = useErrorMessage()

const search = ref('')
const debounced = refDebounced(search, 300)
const teamId = ref('')
const role = ref('')
const status = ref('')
const attrId = ref('')
const attrValue = ref('')
const attrFrom = ref('')
const attrTo = ref('')
const page = ref(1)

const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams })
const { data: attributes } = useQuery({ queryKey: ['attributes'], queryFn: listAttributes })
const selectedAttr = computed(() => attributes.value?.find((a) => a.id === attrId.value))
const isRange = computed(() => selectedAttr.value?.type === 'number' || selectedAttr.value?.type === 'date')

const query = computed(() => {
  const q: Record<string, string | number | undefined> = {
    q: debounced.value,
    team_id: teamId.value,
    role: role.value,
    status: status.value,
    page: page.value,
  }
  if (selectedAttr.value) {
    const key = `attr.${selectedAttr.value.id}`
    if (isRange.value) {
      q[`${key}.from`] = attrFrom.value
      q[`${key}.to`] = attrTo.value
    } else {
      q[key] = attrValue.value
    }
  }
  return q
})
watch([debounced, teamId, role, status, attrId, attrValue, attrFrom, attrTo], () => (page.value = 1))

const { data, error, isFetching } = useQuery({
  queryKey: ['users', query],
  queryFn: () => listUsers(query.value),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))

function clear() {
  search.value = teamId.value = role.value = status.value = attrId.value = ''
  attrValue.value = attrFrom.value = attrTo.value = ''
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <div class="flex items-center justify-between gap-2">
      <h1 class="font-heading text-xl font-semibold">{{ t('users.title') }}</h1>
      <Button v-if="auth.can('users:create:all', 'users:create:team')" as-child>
        <RouterLink to="/users/new">{{ t('users.new') }}</RouterLink>
      </Button>
    </div>

    <div class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-2 lg:grid-cols-4">
      <div class="flex flex-col gap-1.5 sm:col-span-2 lg:col-span-4">
        <Label for="search">{{ t('common.search') }}</Label>
        <Input id="search" v-model="search" :placeholder="t('users.searchPlaceholder')" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="team">{{ t('users.filters.team') }}</Label>
        <NativeSelect id="team" v-model="teamId">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="{ team, depth } in flatten(teams ?? [])" :key="team.id" :value="team.id">
            {{ '  '.repeat(depth) + team.name }}
          </option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="role">{{ t('users.filters.role') }}</Label>
        <NativeSelect id="role" v-model="role">
          <option value="">{{ t('common.all') }}</option>
          <option v-for="r in ROLES" :key="r" :value="r">{{ t(`roles.${r}`) }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="status">{{ t('users.filters.status') }}</Label>
        <NativeSelect id="status" v-model="status">
          <option value="">{{ t('users.filters.statusDefault') }}</option>
          <option value="active">{{ t('status.active') }}</option>
          <option value="inactive">{{ t('status.inactive') }}</option>
          <option value="offboarded">{{ t('status.offboarded') }}</option>
        </NativeSelect>
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="attr">{{ t('users.filters.attribute') }}</Label>
        <NativeSelect id="attr" v-model="attrId">
          <option value="">{{ t('common.none') }}</option>
          <option v-for="a in attributes ?? []" :key="a.id" :value="a.id">{{ a.name }}</option>
        </NativeSelect>
      </div>
      <template v-if="selectedAttr">
        <template v-if="isRange">
          <div class="flex flex-col gap-1.5">
            <Label for="attr-from">{{ t('users.filters.from') }}</Label>
            <Input id="attr-from" v-model="attrFrom" :type="selectedAttr.type === 'date' ? 'date' : 'number'" />
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="attr-to">{{ t('users.filters.to') }}</Label>
            <Input id="attr-to" v-model="attrTo" :type="selectedAttr.type === 'date' ? 'date' : 'number'" />
          </div>
        </template>
        <div v-else class="flex flex-col gap-1.5">
          <Label for="attr-value">{{ t('users.filters.value') }}</Label>
          <NativeSelect v-if="selectedAttr.type === 'boolean'" id="attr-value" v-model="attrValue">
            <option value="">{{ t('common.none') }}</option>
            <option value="true">{{ t('common.yes') }}</option>
            <option value="false">{{ t('common.no') }}</option>
          </NativeSelect>
          <NativeSelect v-else-if="selectedAttr.type === 'select'" id="attr-value" v-model="attrValue">
            <option value="">{{ t('common.none') }}</option>
            <option v-for="o in selectedAttr.options" :key="o.value" :value="o.value">{{ o.value }}</option>
          </NativeSelect>
          <Input v-else id="attr-value" v-model="attrValue" />
        </div>
      </template>
      <div class="flex items-end">
        <Button variant="ghost" size="sm" @click="clear">{{ t('users.filters.clear') }}</Button>
      </div>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="rounded-lg border bg-background" :aria-busy="isFetching">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('users.columns.name') }}</TableHead>
            <TableHead>{{ t('users.columns.email') }}</TableHead>
            <TableHead>{{ t('users.columns.teams') }}</TableHead>
            <TableHead>{{ t('users.columns.roles') }}</TableHead>
            <TableHead>{{ t('users.columns.status') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="u in data?.items ?? []" :key="u.id" data-testid="user-row">
            <TableCell>
              <RouterLink :to="`/users/${u.id}`" class="font-medium hover:underline">{{ u.name }}</RouterLink>
            </TableCell>
            <TableCell class="text-muted-foreground">{{ u.email ?? t('common.none') }}</TableCell>
            <TableCell>
              <div class="flex flex-wrap gap-1">
                <AssignmentBadge v-for="tm in u.teams" :key="tm.id" :label="tm.name" />
              </div>
            </TableCell>
            <TableCell>
              <div class="flex flex-wrap gap-1">
                <AssignmentBadge v-for="r in u.roles ?? []" :key="r.role" :label="t(`roles.${r.role}`)" />
              </div>
            </TableCell>
            <TableCell>
              <Badge :variant="u.status === 'active' ? 'outline' : 'secondary'">{{ t(`status.${u.status}`) }}</Badge>
            </TableCell>
          </TableRow>
          <TableEmpty v-if="data && data.items.length === 0" :colspan="5">{{ t('common.empty') }}</TableEmpty>
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
