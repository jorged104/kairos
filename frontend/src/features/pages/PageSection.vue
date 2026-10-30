<script setup lang="ts">
import { useStyleTag } from '@vueuse/core'
import { AlertTriangle, Check, CircleDot, LoaderCircle, MessageSquarePlus } from '@lucide/vue'
import { computed, ref, shallowRef } from 'vue'
import { useI18n } from 'vue-i18n'
import { onBeforeRouteLeave } from 'vue-router'
import { ApiError } from '@/api/client'
import type { AnchorInput } from '@/api/comments'
import type { PageOwner, SavedPage } from '@/api/pages'
import { Button } from '@/components/ui/button'
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

const below = (r: DOMRect) => ({ top: `${r.bottom + 8}px`, left: `${Math.max(8, Math.min(r.left, window.innerWidth - 400))}px` })
const above = (r: DOMRect) => ({ top: `${Math.max(8, r.top - 40)}px`, left: `${Math.max(8, Math.min(r.right - 40, window.innerWidth - 140))}px` })

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
