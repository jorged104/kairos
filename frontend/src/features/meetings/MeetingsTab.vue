<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { CalendarClock, ListChecks, MapPin, Plus } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRoute } from 'vue-router'
import { chooseTemplates, createMeeting, listMeetings } from '@/api/meetings'
import NativeSelect from '@/components/NativeSelect.vue'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import type { Project } from '@/types/work'
import { fromLocalInput } from './dates'
import MeetingTemplatesAdmin from './MeetingTemplatesAdmin.vue'

// The «Reuniones» tab of a project or program: the upcoming and the past
// ones, 50 a page, a new one blank or from a template, and the project's
// own templates for whoever manages it (spec 018, RF-34, RF-38 to RF-41).
const props = defineProps<{ project: Project }>()
const { t, d } = useI18n()
const qc = useQueryClient()
const route = useRoute()
const errorMessage = useErrorMessage()

const when = ref<'upcoming' | 'past'>('upcoming')
const page = ref(1)
watch(when, () => (page.value = 1))
const { data, error } = useQuery({
  queryKey: computed(() => ['meetings', props.project.id, when.value, page.value]),
  queryFn: () => listMeetings(props.project.id, when.value, page.value),
})
const pages = computed(() => Math.max(1, Math.ceil((data.value?.total ?? 0) / 50)))
const canWrite = computed(() => !!props.project.can.write_work_items && props.project.lifecycle === 'activo')
const canManage = computed(() => !!props.project.can.manage && props.project.lifecycle === 'activo')
const openIn = (id: string) => ({ query: { ...route.query, meeting: id } })

// A new meeting; a template proposes its title and place (RF-34, RF-35).
const creating = ref(false)
const form = ref({ templateId: '', title: '', startsAt: '', location: '' })
const { data: templates } = useQuery({
  queryKey: computed(() => ['meeting-templates', 'choose', props.project.id]),
  queryFn: () => chooseTemplates(props.project.id),
  enabled: creating,
})
watch(
  () => form.value.templateId,
  (id) => {
    const tpl = templates.value?.items.find((x) => x.id === id)
    if (tpl) form.value = { ...form.value, title: tpl.title || form.value.title, location: tpl.location || form.value.location }
  },
)
const create = useMutation({
  mutationFn: () =>
    createMeeting(props.project.id, {
      title: form.value.title.trim(),
      starts_at: fromLocalInput(form.value.startsAt),
      location: form.value.location.trim(),
      ...(form.value.templateId ? { template_id: form.value.templateId } : {}),
    }),
  onSuccess: () => {
    creating.value = false
    form.value = { templateId: '', title: '', startsAt: '', location: '' }
    void qc.invalidateQueries({ queryKey: ['meetings', props.project.id] })
  },
})
const showTemplates = ref(false)
</script>

<template>
  <div class="flex flex-col gap-4" data-testid="meetings-tab">
    <div class="flex flex-wrap items-center gap-3">
      <div class="flex rounded-lg border p-0.5 text-sm" role="group" :aria-label="t('meetings.title')">
        <button
          v-for="w in ['upcoming', 'past'] as const"
          :key="w"
          type="button"
          class="rounded-md px-3 py-1"
          :class="when === w ? 'bg-primary text-primary-foreground' : 'text-muted-foreground hover:text-foreground'"
          :aria-pressed="when === w"
          :data-testid="`meetings-${w}`"
          @click="when = w"
        >
          {{ t(`meetings.${w}`) }}
        </button>
      </div>
      <div class="ml-auto flex gap-2">
        <Button v-if="canManage" size="sm" variant="ghost" data-testid="meetings-templates" @click="showTemplates = !showTemplates">
          {{ t('meetings.templates.project') }}
        </Button>
        <Button v-if="canWrite && !creating" size="sm" data-testid="meeting-new" @click="creating = true">
          <Plus aria-hidden="true" />{{ t('meetings.new') }}
        </Button>
      </div>
    </div>
    <Alert v-if="error || create.error.value" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error ?? create.error.value) }}</AlertDescription>
    </Alert>

    <MeetingTemplatesAdmin v-if="showTemplates && canManage" :project-id="project.id" />

    <form v-if="creating" class="grid gap-3 rounded-xl border p-4 sm:grid-cols-2" data-testid="meeting-create" @submit.prevent="create.mutate()">
      <label class="flex flex-col gap-1 text-sm sm:col-span-2">
        {{ t('meetings.template') }}
        <NativeSelect v-model="form.templateId" data-testid="meeting-template">
          <option value="">{{ t('meetings.blank') }}</option>
          <optgroup v-if="templates?.items.some((x) => x.scope === 'global')" :label="t('meetings.templates.globalGroup')">
            <option v-for="x in templates.items.filter((x) => x.scope === 'global')" :key="x.id" :value="x.id">{{ x.name }}</option>
          </optgroup>
          <optgroup v-if="templates?.items.some((x) => x.scope === 'project')" :label="t('meetings.templates.projectGroup')">
            <option v-for="x in templates.items.filter((x) => x.scope === 'project')" :key="x.id" :value="x.id">{{ x.name }}</option>
          </optgroup>
        </NativeSelect>
      </label>
      <label class="flex flex-col gap-1 text-sm sm:col-span-2">
        {{ t('meetings.fields.title') }}
        <Input v-model="form.title" maxlength="255" required data-testid="meeting-new-title" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('meetings.fields.startsAt') }}
        <Input v-model="form.startsAt" type="datetime-local" required data-testid="meeting-new-starts" />
      </label>
      <label class="flex flex-col gap-1 text-sm">
        {{ t('meetings.fields.location') }}
        <Input v-model="form.location" maxlength="500" :placeholder="t('meetings.fields.locationHint')" data-testid="meeting-new-location" />
      </label>
      <div class="flex gap-2 sm:col-span-2">
        <Button type="submit" size="sm" :disabled="create.isPending.value" data-testid="meeting-new-save">{{ t('meetings.create') }}</Button>
        <Button type="button" size="sm" variant="ghost" @click="creating = false">{{ t('common.cancel') }}</Button>
      </div>
    </form>

    <template v-if="data">
      <p v-if="!data.items.length" class="rounded-lg border border-dashed p-6 text-center text-sm text-muted-foreground" data-testid="meetings-empty">
        {{ when === 'upcoming' ? t('meetings.emptyUpcoming') : t('meetings.emptyPast') }}
      </p>
      <ul v-else class="flex flex-col divide-y rounded-xl border">
        <li v-for="m in data.items" :key="m.id" data-testid="meeting-row">
          <RouterLink :to="openIn(m.id)" class="flex flex-wrap items-center gap-x-4 gap-y-1 px-4 py-3 hover:bg-muted">
            <span class="min-w-0 flex-1 font-medium">{{ m.title }}</span>
            <span class="inline-flex items-center gap-1 text-sm text-muted-foreground"><CalendarClock class="size-4" aria-hidden="true" />{{ d(m.starts_at, 'long') }}</span>
            <span v-if="m.location" class="inline-flex max-w-64 items-center gap-1 truncate text-sm text-muted-foreground"><MapPin class="size-4 shrink-0" aria-hidden="true" />{{ m.location }}</span>
            <span v-if="m.tasks" class="inline-flex items-center gap-1 text-xs text-muted-foreground" data-testid="meeting-row-tasks">
              <ListChecks class="size-4" aria-hidden="true" />{{ t('meetings.taskCount', { n: m.tasks }, m.tasks) }}
            </span>
          </RouterLink>
        </li>
      </ul>
      <div v-if="pages > 1" class="flex items-center gap-2 text-sm">
        <Button size="sm" variant="outline" :disabled="page === 1" @click="page--">{{ t('common.previous') }}</Button>
        <span class="text-muted-foreground">{{ t('notifications.page', { page, pages }) }}</span>
        <Button size="sm" variant="outline" :disabled="page === pages" @click="page++">{{ t('common.next') }}</Button>
      </div>
    </template>
  </div>
</template>
