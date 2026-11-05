<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { Pencil, Plus } from '@lucide/vue'
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  chooseTemplates,
  createTemplate,
  deleteTemplate,
  globalTemplates,
  setTemplateRetired,
  updateTemplate,
  type MeetingTemplate,
} from '@/api/meetings'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import PageEditor from '@/features/richtext/PageEditor.vue'
import { pageForSave } from '@/features/richtext/pageExtensions'
import type { RichDoc } from '@/types/work'
import type { JSONContent } from '@tiptap/vue-3'

// The meeting templates of the catalog (global) or of a project (spec 018,
// RF-29 to RF-33). Their page has the blocks of the notes but images and
// files: the editor has no owner to upload them to (RF-32).
const props = defineProps<{ projectId?: string }>()
const { t } = useI18n()
const qc = useQueryClient()
const errorMessage = useErrorMessage()

const key = computed(() => ['meeting-templates', props.projectId ?? 'global'])
const { data } = useQuery({
  queryKey: key,
  queryFn: async () =>
    props.projectId ? (await chooseTemplates(props.projectId)).items.filter((x) => x.scope === 'project') : (await globalTemplates()).items,
})

const empty: JSONContent = { type: 'doc', content: [{ type: 'paragraph' }] }
const editing = ref<{ id: string | null; name: string; title: string; location: string; page: JSONContent; version: number } | null>(null)
// What the editor holds; it is only read when saving.
let edited: JSONContent | null = null
function onEdit(doc: JSONContent) {
  edited = doc
}
function open(x?: MeetingTemplate) {
  edited = null
  editing.value = x
    ? { id: x.id, name: x.name, title: x.title, location: x.location, page: x.page ?? empty, version: Date.now() }
    : { id: null, name: '', title: '', location: '', page: empty, version: Date.now() }
}

const refresh = () => qc.invalidateQueries({ queryKey: ['meeting-templates'] })
const save = useMutation({
  mutationFn: () => {
    const e = editing.value!
    const page = pageForSave(edited ?? e.page) as RichDoc
    const body = { name: e.name.trim(), title: e.title.trim(), location: e.location.trim(), page, ...(props.projectId ? { project_id: props.projectId } : {}) }
    return e.id ? updateTemplate(e.id, body) : createTemplate(body)
  },
  onSuccess: () => {
    editing.value = null
    void refresh()
  },
})
const retire = useMutation({ mutationFn: (x: MeetingTemplate) => setTemplateRetired(x.id, !x.retired), onSuccess: refresh })
const armed = ref<string | null>(null)
const remove = useMutation({
  mutationFn: (x: MeetingTemplate) => deleteTemplate(x.id),
  onSuccess: () => {
    armed.value = null
    void refresh()
  },
})
const error = computed(() => save.error.value ?? retire.error.value ?? remove.error.value)
</script>

<template>
  <section class="flex flex-col gap-3 rounded-xl border p-4" :aria-label="t('meetings.templates.title')" data-testid="meeting-templates">
    <header class="flex items-center gap-2">
      <h2 class="text-sm font-semibold">{{ projectId ? t('meetings.templates.project') : t('meetings.templates.title') }}</h2>
      <Button v-if="!editing" size="sm" variant="outline" class="ml-auto" data-testid="template-new" @click="open()">
        <Plus aria-hidden="true" />{{ t('meetings.templates.new') }}
      </Button>
    </header>
    <p class="text-xs text-muted-foreground">{{ projectId ? t('meetings.templates.projectHint') : t('meetings.templates.globalHint') }}</p>
    <Alert v-if="error" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(error) }}</AlertDescription>
    </Alert>

    <form v-if="editing" class="flex flex-col gap-3 rounded-lg border p-3" data-testid="template-form" @submit.prevent="save.mutate()">
      <div class="grid gap-3 sm:grid-cols-3">
        <label class="flex flex-col gap-1 text-sm">
          {{ t('meetings.templates.name') }}
          <Input v-model="editing.name" maxlength="100" required data-testid="template-name" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('meetings.templates.proposedTitle') }}
          <Input v-model="editing.title" maxlength="255" data-testid="template-title" />
        </label>
        <label class="flex flex-col gap-1 text-sm">
          {{ t('meetings.fields.location') }}
          <Input v-model="editing.location" maxlength="500" data-testid="template-location" />
        </label>
      </div>
      <div class="rounded-lg border px-2">
        <PageEditor :key="editing.version" :doc="editing.page" :editable="true" :placeholder="t('meetings.templates.pageHint')" @change="onEdit" />
      </div>
      <div class="flex gap-2">
        <Button type="submit" size="sm" :disabled="save.isPending.value" data-testid="template-save">{{ t('common.save') }}</Button>
        <Button type="button" size="sm" variant="ghost" @click="editing = null">{{ t('common.cancel') }}</Button>
      </div>
    </form>

    <p v-if="data && !data.length && !editing" class="text-sm text-muted-foreground">{{ t('meetings.templates.empty') }}</p>
    <ul v-else-if="data" class="flex flex-col divide-y rounded-lg border text-sm">
      <li v-for="x in data" :key="x.id" class="flex flex-wrap items-center gap-2 px-3 py-2" data-testid="template-row">
        <span class="font-medium" :class="x.retired && 'text-muted-foreground line-through'">{{ x.name }}</span>
        <span v-if="x.title" class="truncate text-muted-foreground">· {{ x.title }}</span>
        <span class="ml-auto flex gap-1">
          <Button size="sm" variant="ghost" :aria-label="t('meetings.templates.edit', { name: x.name })" @click="open(x)"><Pencil aria-hidden="true" /></Button>
          <Button v-if="x.scope === 'global'" size="sm" variant="ghost" :data-testid="`template-retire-${x.id}`" @click="retire.mutate(x)">
            {{ x.retired ? t('catalog.restore') : t('catalog.retire') }}
          </Button>
          <Button
            v-else
            size="sm"
            :variant="armed === x.id ? 'destructive' : 'ghost'"
            :data-testid="`template-delete-${x.id}`"
            @click="armed === x.id ? remove.mutate(x) : (armed = x.id)"
          >
            {{ armed === x.id ? t('meetings.confirmDelete') : t('meetings.delete') }}
          </Button>
        </span>
      </li>
    </ul>
  </section>
</template>
