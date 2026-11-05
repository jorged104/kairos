<script setup lang="ts">
import { useMutation, useQuery, useQueryClient } from '@tanstack/vue-query'
import { onClickOutside } from '@vueuse/core'
import { CalendarClock, Expand, MapPin, Trash2, X } from '@lucide/vue'
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink, useRouter } from 'vue-router'
import { createComment } from '@/api/comments'
import { deleteMeeting, getMeeting, meetingCommentTarget, updateMeeting, type Meeting } from '@/api/meetings'
import type { SavedPage } from '@/api/pages'
import { createWorkItem, getProject } from '@/api/work'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import CommentsSection from '@/features/comments/CommentsSection.vue'
import CommentThread from '@/features/comments/CommentThread.vue'
import { useComments } from '@/features/comments/useComments'
import PageSection, { type PageCommenting, type PageTasks } from '@/features/pages/PageSection.vue'
import { useCatalog } from '@/features/work/useCatalog'
import { fromLocalInput, toLocalInput } from './dates'

// A meeting: its title, date and place, its notes with the editor of the
// work items, the work items that came from it and its comments (spec 018,
// RF-1 to RF-28). In the side panel it opens as a page and closes.
const props = defineProps<{ id: string; panel?: boolean }>()
const emit = defineEmits<{ close: [] }>()
const { t, d } = useI18n()
const qc = useQueryClient()
const router = useRouter()
const errorMessage = useErrorMessage()

const key = computed(() => ['meeting', props.id])
const { data: meeting, error: loadError } = useQuery({ queryKey: key, queryFn: () => getMeeting(props.id) })
const { data: project } = useQuery({
  queryKey: computed(() => ['project', meeting.value?.project.id]),
  queryFn: () => getProject(meeting.value!.project.id),
  enabled: computed(() => !!meeting.value),
})
const { catalog } = useCatalog()

// The fields, saved together with the version (RF-15).
const form = ref({ title: '', startsAt: '', location: '' })
watch(
  meeting,
  (m) => m && (form.value = { title: m.title, startsAt: toLocalInput(m.starts_at), location: m.location }),
  { immediate: true },
)
const dirty = computed(
  () =>
    !!meeting.value &&
    (form.value.title !== meeting.value.title ||
      form.value.location !== meeting.value.location ||
      form.value.startsAt !== toLocalInput(meeting.value.starts_at)),
)
const save = useMutation({
  mutationFn: () =>
    updateMeeting(props.id, {
      version: meeting.value!.version,
      title: form.value.title.trim(),
      starts_at: fromLocalInput(form.value.startsAt),
      location: form.value.location.trim(),
    }),
  onSuccess: (m) => {
    qc.setQueryData(key.value, m)
    void qc.invalidateQueries({ queryKey: ['meetings'] })
  },
})
const link = computed(() => (/^https?:\/\//i.test(meeting.value?.location ?? '') ? meeting.value!.location : null)) // RF-6

// Deleting asks once more (RF-11).
const armed = ref(false)
const remove = useMutation({
  mutationFn: () => deleteMeeting(props.id),
  onSuccess: async () => {
    void qc.invalidateQueries({ queryKey: ['meetings'] })
    if (props.panel) emit('close')
    else await router.push(`/projects/${meeting.value!.project.id}?tab=meetings`)
  },
})

// The notes: an autosave updates the cached meeting.
const pageSection = ref<InstanceType<typeof PageSection> | null>(null)
function onPageSaved(saved: SavedPage) {
  qc.setQueryData<Meeting>(key.value, (old) =>
    old && { ...old, description: saved.page, page_version: saved.page_version, version: saved.version, refs: saved.refs },
  )
}
async function reloadPage() {
  const fresh = await getMeeting(props.id)
  qc.setQueryData(key.value, fresh)
  return fresh
}

// Comments, as in the work items (RF-20 to RF-22).
const target = computed(() => meetingCommentTarget(props.id))
const { list: comments, apply: applyComments, openAnchors } = useComments(() => target.value)
const popover = ref<{ ids: string[]; rect: DOMRect } | null>(null)
const popoverEl = ref<HTMLElement | null>(null)
const popoverThreads = computed(() => (comments.value?.threads ?? []).filter((th) => popover.value?.ids.includes(th.id)))
onClickOutside(popoverEl, () => (popover.value = null))
const commenting = computed<PageCommenting>(() => ({
  workItemId: target.value,
  canComment: !!comments.value?.can_comment,
  openThreads: openAnchors.value,
  anchor: async (input, body) => applyComments(await createComment(target.value, body, { anchor: input })).page!,
  show: (ids, rect) => {
    if (ids.some((x) => openAnchors.value.includes(x))) popover.value = { ids, rect }
  },
}))
const popoverAt = (r: DOMRect) => ({ top: `${r.bottom + 8}px`, left: `${Math.max(8, Math.min(r.left, window.innerWidth - 424))}px` })

// Work items from the notes, of the types the project uses (RF-23 to RF-27).
const tasks = computed<PageTasks | undefined>(() => {
  if (!meeting.value?.can.write) return undefined
  const allowed = project.value?.work_item_type_ids ?? []
  const types = catalog.value.work_item_types
    .filter((x) => !x.retired && (!allowed.length || allowed.includes(x.id)))
    .sort((a, b) => a.position - b.position)
  return {
    types,
    create: async (title, typeId) => {
      const wi = await createWorkItem(meeting.value!.project.id, { type_id: typeId, title, meeting_id: props.id })
      void qc.invalidateQueries({ queryKey: key.value })
      return { id: wi.id, label: `${wi.key} ${wi.title}` }
    },
  }
})
const statusName = (id: string) => catalog.value.statuses.find((s) => s.id === id)?.name ?? ''

async function close() {
  await pageSection.value?.flush()
  emit('close')
}
</script>

<template>
  <div class="flex flex-col gap-5" data-testid="meeting-sheet">
    <Alert v-if="loadError" variant="destructive" role="alert">
      <AlertDescription>{{ errorMessage(loadError) }}</AlertDescription>
    </Alert>
    <template v-if="meeting">
      <div class="flex flex-wrap items-center gap-2">
        <nav class="flex min-w-0 flex-wrap items-center gap-1 text-sm text-muted-foreground">
          <RouterLink :to="`/projects/${meeting.project.id}?tab=meetings`" class="hover:underline">{{ meeting.project.name }}</RouterLink>
          <span>/</span>
          <span>{{ t('meetings.title') }}</span>
        </nav>
        <div class="ml-auto flex items-center gap-1">
          <Button
            v-if="meeting.can.write"
            size="sm"
            :variant="armed ? 'destructive' : 'ghost'"
            :disabled="remove.isPending.value"
            data-testid="meeting-delete"
            @click="armed ? remove.mutate() : (armed = true)"
          >
            <Trash2 aria-hidden="true" />{{ armed ? t('meetings.confirmDelete') : t('meetings.delete') }}
          </Button>
          <template v-if="panel">
            <Button as-child size="sm" variant="outline">
              <RouterLink :to="`/meetings/${meeting.id}`" data-testid="meeting-open-page"><Expand aria-hidden="true" />{{ t('page.openAsPage') }}</RouterLink>
            </Button>
            <Button size="sm" variant="ghost" :aria-label="t('page.closePanel')" data-testid="meeting-close" @click="close">
              <X aria-hidden="true" />
            </Button>
          </template>
        </div>
      </div>
      <Alert v-if="save.error.value || remove.error.value" variant="destructive" role="alert">
        <AlertDescription>{{ errorMessage(save.error.value ?? remove.error.value) }}</AlertDescription>
      </Alert>

      <form v-if="meeting.can.write" class="flex flex-col gap-3" data-testid="meeting-form" @submit.prevent="save.mutate()">
        <Input
          v-model="form.title"
          maxlength="255"
          required
          class="h-auto border-transparent px-0 text-2xl font-semibold shadow-none focus-visible:border-input focus-visible:px-2"
          :aria-label="t('meetings.fields.title')"
          data-testid="meeting-title"
        />
        <div class="grid gap-3 sm:grid-cols-2">
          <label class="flex flex-col gap-1 text-sm">
            {{ t('meetings.fields.startsAt') }}
            <Input v-model="form.startsAt" type="datetime-local" required data-testid="meeting-starts" />
          </label>
          <label class="flex flex-col gap-1 text-sm">
            {{ t('meetings.fields.location') }}
            <Input v-model="form.location" maxlength="500" :placeholder="t('meetings.fields.locationHint')" data-testid="meeting-location" />
          </label>
        </div>
        <div v-if="dirty" class="flex gap-2">
          <Button type="submit" size="sm" :disabled="save.isPending.value" data-testid="meeting-save">{{ t('common.save') }}</Button>
          <Button type="button" size="sm" variant="ghost" @click="meeting && (form = { title: meeting.title, startsAt: toLocalInput(meeting.starts_at), location: meeting.location })">
            {{ t('common.cancel') }}
          </Button>
        </div>
      </form>
      <div v-else class="flex flex-col gap-1">
        <h1 class="font-heading text-2xl font-semibold" data-testid="meeting-title-read">{{ meeting.title }}</h1>
      </div>
      <p class="flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">
        <span class="inline-flex items-center gap-1.5"><CalendarClock class="size-4" aria-hidden="true" />{{ d(meeting.starts_at, 'long') }}</span>
        <span v-if="meeting.location" class="inline-flex items-center gap-1.5">
          <MapPin class="size-4" aria-hidden="true" />
          <a v-if="link" :href="link" target="_blank" rel="noopener noreferrer" class="text-primary hover:underline" data-testid="meeting-link">{{ meeting.location }}</a>
          <span v-else>{{ meeting.location }}</span>
        </span>
      </p>

      <section class="flex flex-col gap-2" :aria-label="t('meetings.tasks')" data-testid="meeting-tasks">
        <h2 class="text-sm font-medium">{{ t('meetings.tasks') }} <span class="text-muted-foreground">{{ meeting.tasks.length }}</span></h2>
        <p v-if="!meeting.tasks.length" class="text-sm text-muted-foreground">{{ meeting.can.write ? t('meetings.noTasksHint') : t('meetings.noTasks') }}</p>
        <ul v-else class="flex flex-col divide-y rounded-lg border text-sm">
          <li v-for="task in meeting.tasks" :key="task.id" class="flex items-center gap-2 px-3 py-1.5">
            <RouterLink :to="`/work-items/${task.key}`" class="font-mono text-xs text-muted-foreground hover:underline">{{ task.key }}</RouterLink>
            <span class="min-w-0 flex-1 truncate">{{ task.title }}</span>
            <span class="text-xs text-muted-foreground">{{ statusName(task.status_id) }}</span>
          </li>
        </ul>
      </section>

      <PageSection
        :key="meeting.id"
        ref="pageSection"
        :owner="{ kind: 'meetings', id: meeting.id }"
        :doc="meeting.description"
        :page-version="meeting.page_version"
        :refs="meeting.refs"
        :editable="meeting.can.write"
        :reload="reloadPage"
        :commenting="commenting"
        :tasks="tasks"
        @saved="onPageSaved"
      />

      <CommentsSection :work-item-id="target" />

      <Teleport to="body">
        <div
          v-if="popover && popoverThreads.length && comments"
          ref="popoverEl"
          class="fixed z-50 flex max-h-[60vh] w-[26rem] max-w-[calc(100vw-1rem)] flex-col gap-2 overflow-y-auto rounded-lg border bg-popover p-2 shadow-xl"
          :style="popoverAt(popover.rect)"
          role="dialog"
          :aria-label="t('comments.title')"
        >
          <CommentThread
            v-for="th in popoverThreads"
            :key="th.id"
            :thread="th"
            :refs="comments.refs"
            :work-item-id="target"
            :can-comment="comments.can_comment"
            :apply="applyComments"
          />
        </div>
      </Teleport>
    </template>
  </div>
</template>
