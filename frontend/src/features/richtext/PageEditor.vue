<script setup lang="ts">
import type { Editor as CoreEditor } from '@tiptap/core'
import type { Node as PmNode } from '@tiptap/pm/model'
import { DragHandle } from '@tiptap/extension-drag-handle-vue-3'
import { EditorContent, useEditor, type Editor, type JSONContent } from '@tiptap/vue-3'
import { Copy, GripVertical, Repeat2, Trash2 } from '@lucide/vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { listUsers } from '@/api/admin'
import { searchMentions, type PageOwner } from '@/api/pages'
import type { MentionRefs, RichDoc } from '@/types/work'
import { COMMENT_MARK, threadsAt, withoutComments } from './commentMark'
import FileBlock from './FileBlock.vue'
import { pageExtensions, type PageContext } from './pageExtensions'
import type { SuggestionItem } from './suggestionRender'
import { FILE_TYPES, IMAGE_TYPES, startUploads } from './uploads'

// A page of spec 005: blocks inserted with /, dragged by their handle and
// converted, duplicated or deleted from its menu (RF-7 to RF-12). Without
// editable it is the read view (RF-13). Compact, without handles, it writes
// comments (spec 006).
const props = defineProps<{
  doc: RichDoc
  refs?: MentionRefs
  editable: boolean
  /** Whose page it is: files are uploaded to it. Needed when editable. */
  owner?: PageOwner
  compact?: boolean
  placeholder?: string
  /** Reports the selected text, to comment on it (spec 006, RF-44). */
  commentable?: boolean
}>()
/** A piece of selected text and where it ends on screen. */
export interface TextSelection {
  from: number
  to: number
  quote: string
  rect: DOMRect
}
const emit = defineEmits<{
  change: [JSONContent]
  select: [TextSelection | null]
  /** A click on commented text: its threads and where (RF-47). */
  commentClick: [ids: string[], rect: DOMRect]
}>()
const { t } = useI18n()

const refs = computed(() => props.refs ?? {})
const picker = ref<HTMLInputElement | null>(null)
const pickKind = ref<'image' | 'file'>('file')

async function searchPeople(q: string): Promise<SuggestionItem[]> {
  const page = await listUsers({ q, status: 'active', page_size: 8 })
  return page.items.map((u) => ({ id: u.id, label: u.name, hint: u.email, kind: 'user' }))
}

async function searchElements(q: string): Promise<SuggestionItem[]> {
  if (!q.trim()) return []
  const r = await searchMentions(q)
  const group = (key: string) => t(`page.mention.groups.${key}`)
  return [
    ...r.work_items.map((x) => ({ ...x, group: group('workItems') })),
    ...r.milestones.map((x) => ({ ...x, group: group('milestones') })),
    ...r.projects.map((x) => ({ ...x, group: group('projects') })),
  ]
}

const ctx: PageContext = {
  t: (key, values) => (values ? t(key, values) : t(key)),
  refs: () => refs.value,
  searchPeople,
  searchElements,
  pickFiles: (kind) => {
    pickKind.value = kind
    picker.value?.click()
  },
  upload: (files, pos) => {
    if (editor.value && props.owner) startUploads(editor.value, props.owner, files, pos)
  },
  uploadView: FileBlock,
  placeholder: props.placeholder ? () => props.placeholder! : undefined,
}

// Read only, the checkboxes of task lists can't be ticked (RF-13).
function syncCheckboxes(e: CoreEditor) {
  e.view.dom.querySelectorAll<HTMLInputElement>('li[data-checked] input[type="checkbox"]').forEach((i) => {
    i.disabled = !e.isEditable
  })
}

const editor = useEditor({
  extensions: pageExtensions(ctx),
  content: props.doc ?? '',
  editable: props.editable,
  onCreate: ({ editor }) => syncCheckboxes(editor),
  onTransaction: ({ editor }) => syncCheckboxes(editor),
  // An anchor added after its comment was saved is no change to autosave.
  onUpdate: ({ editor, transaction }) => {
    if (!transaction.getMeta(ANCHOR_META)) emit('change', editor.getJSON())
  },
  editorProps: {
    handleClick: (view, pos, event) => {
      const ids = threadsAt(view.state, pos)
      if (ids.length) emit('commentClick', ids, (event.target as HTMLElement).getBoundingClientRect())
      return false
    },
  },
})

const ANCHOR_META = 'kairosAnchor'

// The selection is read from the browser, so it works read-only too (RF-46).
function reportSelection() {
  const e = editor.value
  if (!props.commentable || !e) return
  const sel = window.getSelection()
  if (!sel || sel.isCollapsed || sel.rangeCount === 0 || !e.view.dom.contains(sel.anchorNode) || !e.view.dom.contains(sel.focusNode)) {
    emit('select', null)
    return
  }
  try {
    const a = e.view.posAtDOM(sel.anchorNode!, sel.anchorOffset)
    const b = e.view.posAtDOM(sel.focusNode!, sel.focusOffset)
    const from = Math.min(a, b)
    const to = Math.max(a, b)
    const quote = e.state.doc.textBetween(from, to, ' ').trim()
    if (!quote) {
      emit('select', null)
      return
    }
    emit('select', { from, to, quote: quote.slice(0, 1000), rect: sel.getRangeAt(0).getBoundingClientRect() })
  } catch {
    emit('select', null)
  }
}

/** The document with thread's anchor over from-to, without applying it. */
function markedDoc(threadId: string, from: number, to: number): JSONContent | null {
  const e = editor.value
  if (!e) return null
  const type = e.schema.marks[COMMENT_MARK]!
  return e.state.tr.addMark(from, to, type.create({ threadId })).doc.toJSON()
}

/** Applies the anchor once saved, out of the history and the autosave. */
function applyMark(threadId: string, from: number, to: number) {
  const e = editor.value
  if (!e) return
  const type = e.schema.marks[COMMENT_MARK]!
  e.view.dispatch(e.state.tr.addMark(from, to, type.create({ threadId })).setMeta('addToHistory', false).setMeta(ANCHOR_META, true))
}

watch(
  () => props.doc,
  (doc) => {
    const e = editor.value
    if (!e || JSON.stringify(e.getJSON()) === JSON.stringify(doc ?? e.schema.topNodeType.createAndFill()?.toJSON())) return
    e.commands.setContent(doc ?? '', { emitUpdate: false })
  },
)
watch(
  () => props.editable,
  (v) => {
    editor.value?.setEditable(v, false)
    if (editor.value) syncCheckboxes(editor.value)
  },
)
onBeforeUnmount(() => editor.value?.destroy())

function onPicked(e: Event) {
  const input = e.target as HTMLInputElement
  const files = Array.from(input.files ?? [])
  input.value = ''
  if (files.length) ctx.upload(files, editor.value?.state.selection.from ?? 0)
}

// The block under the handle and its menu (RF-10 to RF-12). Touch screens
// get no handle: dragging blocks is out of scope there.
const touch = typeof window !== 'undefined' && window.matchMedia?.('(pointer: coarse)').matches
const current = ref<{ node: PmNode; pos: number } | null>(null)
const menuOpen = ref(false)
const convertOpen = ref(false)

function onNodeChange({ node, pos }: { node: PmNode | null; pos: number }) {
  if (menuOpen.value) return
  current.value = node ? { node, pos } : null
}

function closeMenu() {
  menuOpen.value = false
  convertOpen.value = false
}

type Conversion = { id: string; run: (e: Editor) => boolean }
const conversions: Conversion[] = [
  { id: 'text', run: (e) => e.chain().focus().setParagraph().run() },
  { id: 'heading1', run: (e) => e.chain().focus().setHeading({ level: 1 }).run() },
  { id: 'heading2', run: (e) => e.chain().focus().setHeading({ level: 2 }).run() },
  { id: 'heading3', run: (e) => e.chain().focus().setHeading({ level: 3 }).run() },
  { id: 'bulletList', run: (e) => e.chain().focus().toggleBulletList().run() },
  { id: 'orderedList', run: (e) => e.chain().focus().toggleOrderedList().run() },
  { id: 'taskList', run: (e) => e.chain().focus().toggleTaskList().run() },
  { id: 'quote', run: (e) => e.chain().focus().toggleBlockquote().run() },
  { id: 'code', run: (e) => e.chain().focus().toggleCodeBlock().run() },
]

function convert(c: Conversion) {
  const e = editor.value
  if (!e || !current.value) return
  e.commands.setTextSelection(current.value.pos + 1)
  c.run(e)
  closeMenu()
}

function duplicate() {
  const e = editor.value
  const at = current.value
  if (!e || !at) return
  // The copy doesn't take the block's comments with it (spec 006, RF-53).
  e.chain().focus().insertContentAt(at.pos + at.node.nodeSize, withoutComments(at.node.toJSON())).run()
  closeMenu()
}

function remove() {
  const e = editor.value
  const at = current.value
  if (!e || !at) return
  e.chain().focus().deleteRange({ from: at.pos, to: at.pos + at.node.nodeSize }).run()
  current.value = null
  closeMenu()
}

defineExpose({ editor, markedDoc, applyMark })
</script>

<template>
  <div
    class="page relative"
    :class="{ 'page-readonly': !editable, 'page-compact': compact }"
    data-testid="page"
    @mouseup="reportSelection"
    @keyup="reportSelection"
  >
    <DragHandle v-if="editor && editable && !touch && !compact" :editor="editor" :on-node-change="onNodeChange" class="page-handle">
      <div class="relative">
        <button
          type="button"
          class="flex h-6 w-5 cursor-grab items-center justify-center rounded text-muted-foreground hover:bg-muted"
          :aria-label="t('page.handle')"
          :aria-expanded="menuOpen"
          data-testid="block-handle"
          @click="menuOpen = !menuOpen"
        >
          <GripVertical class="size-4" aria-hidden="true" />
        </button>
        <div
          v-if="menuOpen"
          class="absolute left-6 top-0 z-40 w-56 rounded-lg border bg-popover p-1 text-sm shadow-lg"
          role="menu"
          data-testid="block-menu"
          @keydown.esc.prevent="closeMenu"
        >
          <button type="button" role="menuitem" class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted" @click="convertOpen = !convertOpen">
            <Repeat2 class="size-4" aria-hidden="true" />{{ t('page.menu.convert') }}
          </button>
          <div v-if="convertOpen" class="ml-6 flex flex-col border-l pl-1">
            <button
              v-for="c in conversions"
              :key="c.id"
              type="button"
              role="menuitem"
              class="rounded-md px-2 py-1 text-left hover:bg-muted"
              :data-testid="`convert-${c.id}`"
              @click="convert(c)"
            >
              {{ t(`page.blocks.${c.id}`) }}
            </button>
          </div>
          <button type="button" role="menuitem" class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 hover:bg-muted" data-testid="block-duplicate" @click="duplicate">
            <Copy class="size-4" aria-hidden="true" />{{ t('page.menu.duplicate') }}
          </button>
          <button type="button" role="menuitem" class="flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-destructive hover:bg-muted" data-testid="block-delete" @click="remove">
            <Trash2 class="size-4" aria-hidden="true" />{{ t('page.menu.delete') }}
          </button>
        </div>
      </div>
    </DragHandle>
    <EditorContent :editor="editor" class="rich-text page-content" />
    <input
      v-if="editable"
      ref="picker"
      type="file"
      class="hidden"
      multiple
      :accept="pickKind === 'image' ? IMAGE_TYPES : FILE_TYPES"
      data-testid="file-picker"
      @change="onPicked"
    />
  </div>
</template>
