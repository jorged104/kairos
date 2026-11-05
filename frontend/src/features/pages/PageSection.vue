<script setup lang="ts">
import { useStyleTag } from '@vueuse/core'
import { AlertTriangle, Check, CircleDot, ListPlus, LoaderCircle, MessageSquarePlus } from '@lucide/vue'
import { computed, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave } from 'vue-router'
import { ApiError } from '@/api/client'
import type { AnchorInput } from '@/api/comments'
import type { PageOwner, SavedPage } from '@/api/pages'
import NativeSelect from '@/components/NativeSelect.vue'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { useErrorMessage } from '@/composables/useErrorMessage'
import CommentEditor from '@/features/comments/CommentEditor.vue'
import PageEditor, { type TextSelection } from '@/features/richtext/PageEditor.vue'
import { pageForSave } from '@/features/richtext/pageExtensions'
import type { MentionRefs, RichDoc } from '@/types/work'
import { usePageAutosave } from './usePageAutosave'

/** What a work item's page needs to comment on its text (spec 006, RF-44 to RF-55). */
export interface PageCommenting {
  workItemId: string
  canComment: boolean
  /** Threads whose text is highlighted: the open ones (RF-49). */
  openThreads: string[]
  /** Publishes an anchored thread and returns the page with its anchor. */
  anchor: (input: AnchorInput, body: RichDoc) => Promise<SavedPage>
  /** Shows the threads of a clicked highlight (RF-47). */
  show: (ids: string[], rect: DOMRect) => void
}

/**
 * Work items out of the selected text, as the notes of a meeting do (spec
 * 018, RF-23 to RF-25): the new one is mentioned right after the selection.
 */
export interface PageTasks {
  types: { id: string; name: string }[]
  create: (title: string, typeId: string) => Promise<{ id: string; label: string }>
}

// The page of a work item, milestone or project, saved as it is written
// (RF-31 to RF-36). Who can't edit it reads it (RF-13).
const props = defineProps<{
  owner: PageOwner
  doc: RichDoc
  pageVersion: number
  refs?: MentionRefs
  editable: boolean
  /** Reads the owner again, to take someone else's version after a conflict. */
  reload: () => Promise<{ description: RichDoc; page_version: number; refs?: MentionRefs }>
  commenting?: PageCommenting
  tasks?: PageTasks
}>()
const emit = defineEmits<{ saved: [SavedPage] }>()
const { t } = useI18n()

// The editor's content is only replaced when the page is reloaded; the
// changes the user types stay in the editor.
const shown = shallowRef<RichDoc>(props.doc)
const autosave = usePageAutosave(() => props.owner, props.pageVersion, props.refs ?? {}, { onSaved: (s) => emit('saved', s) })
const copied = ref(false)

async function copyMine() {
  try {
    await navigator.clipboard.writeText(autosave.myText())
    copied.value = true
  } catch {
    copied.value = false
  }
}

async function takeTheirs() {
  const fresh = await props.reload()
  shown.value = fresh.description
  autosave.adopt(fresh.page_version, fresh.refs ?? {})
  copied.value = false
}

// Comments on the page's text (spec 006). The highlight is a style sheet of
// the open threads, so resolving one removes it without touching the page.
const pageEditor = ref<InstanceType<typeof PageEditor> | null>(null)
const selection = ref<TextSelection | null>(null)
const composing = ref<TextSelection | null>(null)
const anchoring = ref(false)
const ID = /^[0-9a-f-]{36}$/i
useStyleTag(
  computed(() => {
    const ids = (props.commenting?.openThreads ?? []).filter((id) => ID.test(id))
    return ids.length ? ids.map((id) => `.page-comment[data-comment="${id}"]`).join(',') + '{background-color:var(--comment-highlight);border-bottom:2px solid var(--comment-underline);cursor:pointer}' : ''
  }),
)

function onSelect(s: TextSelection | null) {
  if (!composing.value) selection.value = s
}
function startComment() {
  composing.value = selection.value
  selection.value = null
}

// The anchor goes on the saved page: pending changes are saved first and the
// editor waits for the answer, so no keystroke is lost (RF-45, RF-46).
async function publishAnchored(body: RichDoc) {
  const c = composing.value
  const editor = pageEditor.value
  if (!c || !editor || !props.commenting) return
  await autosave.flush()
  if (autosave.status.value === 'conflict') throw new ApiError('page_conflict', 409)
  const current = editor.editor?.state.doc.textBetween(c.from, c.to, ' ').trim()
  if (current !== c.quote) throw new ApiError('anchor_page_changed', 409)
  const threadId = crypto.randomUUID()
  const page = editor.markedDoc(threadId, c.from, c.to)
  anchoring.value = true
  try {
    const saved = await props.commenting.anchor(
      { thread_id: threadId, quote: c.quote, page: page && pageForSave(page), page_version: autosave.pageVersion.value },
      body,
    )
    editor.applyMark(threadId, c.from, c.to)
    autosave.adopt(saved.page_version, saved.refs)
    emit('saved', saved)
    composing.value = null
  } finally {
    anchoring.value = false
  }
}

// «Crear tarea» from the selection (spec 018).
const errorMessage = useErrorMessage()
const tasking = ref<{ sel: TextSelection; title: string; typeId: string; error: unknown; busy: boolean } | null>(null)
function startTask() {
  const sel = selection.value
  if (!sel || !props.tasks) return
  tasking.value = { sel, title: sel.quote.slice(0, 255), typeId: props.tasks.types[0]?.id ?? '', error: null, busy: false }
  selection.value = null
}
async function createTask() {
  const draft = tasking.value
  const editor = pageEditor.value?.editor
  if (!draft || !props.tasks || !editor || !draft.title.trim() || !draft.typeId) return
  draft.busy = true
  draft.error = null
  try {
    const task = await props.tasks.create(draft.title.trim(), draft.typeId)
    editor
      .chain()
      .insertContentAt(draft.sel.to, [
        { type: 'text', text: ' ' },
        { type: 'mention', attrs: { id: task.id, kind: 'work_item', label: task.label } },
      ])
      .run()
    tasking.value = null
  } catch (e) {
    draft.error = e
  } finally {
    draft.busy = false
  }
}

const below = (r: DOMRect) => ({ top: `${r.bottom + 8}px`, left: `${Math.max(8, Math.min(r.left, window.innerWidth - 400))}px` })
// The second button above a selection goes next to the first.
const above = (r: DOMRect, slot = 0) => ({
  top: `${Math.max(8, r.top - 40)}px`,
  left: `${Math.max(8, Math.min(r.right - 40 + slot * 130, window.innerWidth - 140))}px`,
})

onBeforeRouteLeave(() => autosave.flush())
defineExpose({ flush: autosave.flush, hasChanges: autosave.hasChanges })
</script>

<template>
  <section class="flex flex-col gap-2" :aria-label="t('page.label')">
    <div
      v-if="autosave.status.value === 'conflict'"
      role="alert"
      class="flex flex-wrap items-center gap-3 rounded-lg border border-amber-300 bg-amber-50 px-3 py-2 text-sm text-amber-950 dark:border-amber-700 dark:bg-amber-950 dark:text-amber-100"
      data-testid="page-conflict"
    >
      <AlertTriangle class="size-4 flex-none" aria-hidden="true" />
      <span class="min-w-0 flex-[1_1_16rem]">{{ t('page.conflict.message') }}</span>
      <Button size="sm" variant="outline" type="button" data-testid="copy-mine" @click="copyMine">
        {{ copied ? t('page.conflict.copied') : t('page.conflict.copy') }}
      </Button>
      <Button size="sm" type="button" data-testid="take-theirs" @click="takeTheirs">{{ t('page.conflict.reload') }}</Button>
    </div>
    <div v-if="editable" class="flex items-center gap-1.5 text-xs" aria-live="polite" data-testid="save-status">
      <template v-if="autosave.status.value === 'saved'">
        <Check class="size-3.5 text-emerald-700 dark:text-emerald-400" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.saved') }}</span>
      </template>
      <template v-else-if="autosave.status.value === 'dirty'">
        <CircleDot class="size-3.5 text-amber-700 dark:text-amber-400" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.dirty') }}</span>
      </template>
      <template v-else-if="autosave.status.value === 'saving'">
        <LoaderCircle class="size-3.5 animate-spin text-muted-foreground" aria-hidden="true" />
        <span class="text-muted-foreground">{{ t('page.save.saving') }}</span>
      </template>
      <span v-else-if="autosave.status.value === 'too_long'" class="text-destructive" role="alert">{{ t('page.save.tooLong') }}</span>
      <span v-else-if="autosave.status.value === 'error'" class="text-destructive" role="alert">
        {{ t('page.save.error', { reason: t(`errors.${autosave.errorCode.value ?? 'unknown'}`) }) }}
      </span>
    </div>
    <PageEditor
      ref="pageEditor"
      :doc="shown"
      :refs="autosave.refs.value"
      :editable="editable && autosave.status.value !== 'conflict' && !anchoring"
      :owner="owner"
      :commentable="!!commenting?.canComment"
      @change="autosave.change"
      @select="onSelect"
      @comment-click="(ids, rect) => commenting?.show(ids, rect)"
    />
    <Teleport to="body">
      <Button
        v-if="selection && commenting?.canComment"
        size="sm"
        class="fixed z-50 shadow-lg"
        :style="above(selection.rect)"
        data-testid="comment-selection"
        @mousedown.prevent
        @click="startComment"
      >
        <MessageSquarePlus aria-hidden="true" />{{ t('comments.comment') }}
      </Button>
      <Button
        v-if="selection && tasks && editable"
        size="sm"
        variant="secondary"
        class="fixed z-50 shadow-lg"
        :style="above(selection.rect, commenting?.canComment ? 1 : 0)"
        data-testid="task-selection"
        @mousedown.prevent
        @click="startTask"
      >
        <ListPlus aria-hidden="true" />{{ t('meetings.createTask') }}
      </Button>
      <form
        v-if="tasking && tasks"
        class="fixed z-50 flex w-96 max-w-[calc(100vw-1rem)] flex-col gap-2 rounded-lg border bg-popover p-3 text-sm shadow-xl"
        :style="below(tasking.sel.rect)"
        role="dialog"
        :aria-label="t('meetings.createTask')"
        data-testid="task-popover"
        @submit.prevent="createTask"
      >
        <label class="flex flex-col gap-1">
          {{ t('meetings.taskTitle') }}
          <Input v-model="tasking.title" maxlength="255" required data-testid="task-title" />
        </label>
        <label class="flex flex-col gap-1">
          {{ t('meetings.taskType') }}
          <NativeSelect v-model="tasking.typeId" required data-testid="task-type">
            <option v-for="x in tasks.types" :key="x.id" :value="x.id">{{ x.name }}</option>
          </NativeSelect>
        </label>
        <p v-if="tasking.error" class="text-destructive" role="alert">{{ errorMessage(tasking.error) }}</p>
        <div class="flex justify-end gap-2">
          <Button type="button" size="sm" variant="ghost" @click="tasking = null">{{ t('common.cancel') }}</Button>
          <Button type="submit" size="sm" :disabled="tasking.busy" data-testid="task-create">{{ t('meetings.createTask') }}</Button>
        </div>
      </form>
      <div
        v-if="composing && commenting"
        class="fixed z-50 flex w-96 max-w-[calc(100vw-1rem)] flex-col gap-2 rounded-lg border bg-popover p-2 shadow-xl"
        :style="below(composing.rect)"
        role="dialog"
        :aria-label="t('comments.comment')"
        data-testid="comment-anchor-popover"
      >
        <p class="line-clamp-2 border-l-2 border-amber-400 pl-2 text-sm text-muted-foreground">{{ composing.quote }}</p>
        <CommentEditor :work-item-id="commenting.workItemId" :submit="publishAnchored" autofocus @cancel="composing = null" />
      </div>
    </Teleport>
  </section>
</template>
