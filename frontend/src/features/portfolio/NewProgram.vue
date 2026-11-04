<script setup lang="ts">
import { useMutation } from '@tanstack/vue-query'
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { createProgram } from '@/api/portfolio'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { useErrorMessage } from '@/composables/useErrorMessage'
import { usePeople } from '@/features/work/usePeople'
import type { Project } from '@/types/work'

// A new blank program from the portfolio, for the administrator (spec 010,
// RF-41 to RF-43).
const emit = defineEmits<{ created: [Project]; cancel: [] }>()
const { t } = useI18n()
const errorMessage = useErrorMessage()
const { users, ownTeams } = usePeople()

const form = ref({ identifier: '', name: '', teamId: '', ownerId: '' })
watch(ownTeams, (list) => {
  if (!form.value.teamId && list.length) form.value.teamId = list[0]!.team.id
}, { immediate: true })

const save = useMutation({
  mutationFn: () => {
    const f = form.value
    return createProgram({ identifier: f.identifier.toUpperCase(), name: f.name, team_id: f.teamId, owner_id: f.ownerId || undefined })
  },
  onSuccess: (p) => emit('created', p),
})
</script>

<template>
  <form class="grid gap-3 rounded-xl border bg-background p-4 sm:grid-cols-2 lg:grid-cols-4" data-testid="portfolio-new-form" @submit.prevent="save.mutate()">
    <h2 class="text-base font-semibold sm:col-span-2 lg:col-span-4">{{ t('portfolio.new.title') }}</h2>
    <Alert v-if="save.error.value" variant="destructive" role="alert" class="sm:col-span-2 lg:col-span-4">
      <AlertDescription>{{ errorMessage(save.error.value) }}</AlertDescription>
    </Alert>
    <div class="flex flex-col gap-1.5">
      <Label for="program-identifier">{{ t('projects.identifier') }}</Label>
      <Input id="program-identifier" v-model="form.identifier" maxlength="10" class="uppercase" required data-testid="portfolio-new-identifier" />
    </div>
    <div class="flex flex-col gap-1.5">
      <Label for="program-name">{{ t('projects.name') }}</Label>
      <Input id="program-name" v-model="form.name" required data-testid="portfolio-new-name" />
    </div>
    <div class="flex flex-col gap-1.5">
      <Label for="program-team">{{ t('projects.team') }}</Label>
      <NativeSelect id="program-team" v-model="form.teamId" required data-testid="portfolio-new-team">
        <option value="" disabled>{{ t('projects.chooseTeam') }}</option>
        <option v-for="{ team, depth } in ownTeams" :key="team.id" :value="team.id">{{ '  '.repeat(depth) + team.name }}</option>
      </NativeSelect>
    </div>
    <div class="flex flex-col gap-1.5">
      <Label for="program-owner">{{ t('projects.owner') }}</Label>
      <NativeSelect id="program-owner" v-model="form.ownerId">
        <option value="">{{ t('projects.ownerMe') }}</option>
        <option v-for="u in users" :key="u.id" :value="u.id">{{ u.name }}</option>
      </NativeSelect>
    </div>
    <div class="flex gap-2 sm:col-span-2 lg:col-span-4">
      <Button type="submit" size="sm" :disabled="save.isPending.value" data-testid="portfolio-new-save">{{ t('common.create') }}</Button>
      <Button type="button" size="sm" variant="ghost" @click="emit('cancel')">{{ t('common.cancel') }}</Button>
    </div>
  </form>
</template>
