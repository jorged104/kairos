<script setup lang="ts">
import { keepPreviousData, useQuery } from '@tanstack/vue-query'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listAudit } from '@/api/admin'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableEmpty, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { AuditEntry } from '@/types/api'

const { t, d } = useI18n()
const errorMessage = useErrorMessage()

const userId = ref('')
const actorId = ref('')
const from = ref('')
const to = ref('')
const page = ref(1)
watch([userId, actorId, from, to], () => (page.value = 1))

const query = computed(() => ({
  user_id: userId.value.trim(),
  actor_id: actorId.value.trim(),
  from: from.value,
  to: to.value,
  page: page.value,
}))
const { data, error } = useQuery({
  queryKey: ['audit', query],
  queryFn: () => listAudit(query.value),
  placeholderData: keepPreviousData,
})
const pages = computed(() => (data.value ? Math.max(1, Math.ceil(data.value.total / data.value.page_size)) : 1))

function actor(e: AuditEntry): string {
  if (e.actor_kind === 'user') return e.actor_name ?? e.actor_id ?? ''
  return t(`audit.actors.${e.actor_kind}`)
}

function change(e: AuditEntry): string {
  const fmt = (v: unknown) => (v === null || v === undefined ? '' : JSON.stringify(v))
  return [fmt(e.before), fmt(e.after)].filter(Boolean).join(' → ')
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <h1 class="font-heading text-xl font-semibold">{{ t('audit.title') }}</h1>

    <div class="grid gap-3 rounded-lg border bg-background p-3 sm:grid-cols-4">
      <div class="flex flex-col gap-1.5">
        <Label for="audit-user">{{ t('audit.userFilter') }}</Label>
        <Input id="audit-user" v-model="userId" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="audit-actor">{{ t('audit.actorFilter') }}</Label>
        <Input id="audit-actor" v-model="actorId" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="audit-from">{{ t('users.filters.from') }}</Label>
        <Input id="audit-from" v-model="from" type="date" />
      </div>
      <div class="flex flex-col gap-1.5">
        <Label for="audit-to">{{ t('users.filters.to') }}</Label>
        <Input id="audit-to" v-model="to" type="date" />
      </div>
    </div>

    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <div class="rounded-lg border bg-background">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('audit.when') }}</TableHead>
            <TableHead>{{ t('audit.actor') }}</TableHead>
            <TableHead>{{ t('audit.action') }}</TableHead>
            <TableHead>{{ t('audit.target') }}</TableHead>
            <TableHead>{{ t('audit.change') }}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="e in data?.items ?? []" :key="e.id">
            <TableCell class="whitespace-nowrap">{{ d(new Date(e.occurred_at), 'long') }}</TableCell>
            <TableCell>{{ actor(e) }}</TableCell>
            <TableCell class="font-mono text-xs">{{ e.action }}</TableCell>
            <TableCell>{{ e.target_name ?? e.target_id }}</TableCell>
            <TableCell class="max-w-md truncate font-mono text-xs" :title="change(e)">{{ change(e) }}</TableCell>
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
