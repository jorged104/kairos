<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { X } from '@lucide/vue'
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { createIdpMapping, deleteIdpMapping, listIdpMappings, listTeams, type IdpMappingInput } from '@/api/admin'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Table, TableBody, TableCell, TableEmpty, TableHead, TableHeader, TableRow } from '@/components/ui/table'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { flatten } from '@/features/teams/tree'
import { ROLES, type IdpMapping } from '@/types/api'

const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const { data } = useQuery({ queryKey: ['idp-mappings'], queryFn: listIdpMappings })
const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams })

const blank = (): IdpMappingInput => ({ issuer: '', claim: 'groups', external_value: '', target_type: 'team', target: '' })
const form = ref<IdpMappingInput>(blank())
const error = ref<unknown>(null)

const create = useMutation({
  mutationFn: () => createIdpMapping(form.value),
  onMutate: () => (error.value = null),
  onSuccess: async () => {
    form.value = blank()
    await qc.invalidateQueries({ queryKey: ['idp-mappings'] })
  },
  onError: (e) => (error.value = e),
})
const remove = useMutation({
  mutationFn: (id: string) => deleteIdpMapping(id),
  onSuccess: () => qc.invalidateQueries({ queryKey: ['idp-mappings'] }),
  onError: (e) => (error.value = e),
})

function targetLabel(m: IdpMapping): string {
  if (m.target_type === 'role') return t(`roles.${m.target}`)
  return teams.value?.find((tm) => tm.id === m.target)?.name ?? m.target
}
</script>

<template>
  <div class="flex flex-col gap-4">
    <Card>
      <CardHeader>
        <CardTitle>{{ t('idpMappings.title') }}</CardTitle>
        <CardDescription>{{ t('idpMappings.description') }}</CardDescription>
      </CardHeader>
      <CardContent>
        <form class="grid gap-3 sm:grid-cols-5" @submit.prevent="create.mutate()">
          <Alert v-if="error" variant="destructive" role="alert" class="sm:col-span-5">
            <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
          </Alert>
          <div class="flex flex-col gap-1.5">
            <Label for="map-claim">{{ t('idpMappings.claim') }}</Label>
            <NativeSelect id="map-claim" v-model="form.claim">
              <option value="groups">{{ t('idpMappings.claims.groups') }}</option>
              <option value="roles">{{ t('idpMappings.claims.roles') }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5 sm:col-span-2">
            <Label for="map-value">{{ t('idpMappings.externalValue') }}</Label>
            <Input id="map-value" v-model="form.external_value" placeholder="/Areas/BI/*" required />
            <p class="text-xs text-muted-foreground">{{ t('idpMappings.externalHint') }}</p>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="map-type">{{ t('idpMappings.targetType') }}</Label>
            <NativeSelect id="map-type" v-model="form.target_type" @change="form.target = ''">
              <option value="team">{{ t('idpMappings.targetTypes.team') }}</option>
              <option value="role">{{ t('idpMappings.targetTypes.role') }}</option>
            </NativeSelect>
          </div>
          <div class="flex flex-col gap-1.5">
            <Label for="map-target">{{ t('idpMappings.target') }}</Label>
            <NativeSelect id="map-target" v-model="form.target" required>
              <option value="" />
              <template v-if="form.target_type === 'role'">
                <option v-for="r in ROLES" :key="r" :value="r">{{ t(`roles.${r}`) }}</option>
              </template>
              <template v-else>
                <option v-for="{ team, depth } in flatten((teams ?? []).filter((tm) => !tm.archived_at))" :key="team.id" :value="team.id">
                  {{ '  '.repeat(depth) + team.name }}
                </option>
              </template>
            </NativeSelect>
          </div>
          <div class="flex justify-end sm:col-span-5">
            <Button type="submit" :disabled="create.isPending.value">{{ t('common.add') }}</Button>
          </div>
        </form>
      </CardContent>
    </Card>

    <div class="rounded-lg border bg-background">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>{{ t('idpMappings.issuer') }}</TableHead>
            <TableHead>{{ t('idpMappings.claim') }}</TableHead>
            <TableHead>{{ t('idpMappings.externalValue') }}</TableHead>
            <TableHead>{{ t('idpMappings.target') }}</TableHead>
            <TableHead />
          </TableRow>
        </TableHeader>
        <TableBody>
          <TableRow v-for="m in data?.items ?? []" :key="m.id">
            <TableCell class="font-mono text-xs">{{ m.issuer }}</TableCell>
            <TableCell>{{ t(`idpMappings.claims.${m.claim}`) }}</TableCell>
            <TableCell class="font-mono text-xs">{{ m.external_value }}</TableCell>
            <TableCell>{{ t(`idpMappings.targetTypes.${m.target_type}`) }}: {{ targetLabel(m) }}</TableCell>
            <TableCell class="text-right">
              <Button size="icon-xs" variant="ghost" :aria-label="t('common.remove')" @click="remove.mutate(m.id)"><X /></Button>
            </TableCell>
          </TableRow>
          <TableEmpty v-if="data && data.items.length === 0" :colspan="5">{{ t('common.empty') }}</TableEmpty>
        </TableBody>
      </Table>
    </div>
  </div>
</template>
