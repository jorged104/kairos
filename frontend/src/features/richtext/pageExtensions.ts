import { Extension, Node, mergeAttributes, type Editor, type Extensions, type Range } from '@tiptap/core'
import { Details, DetailsContent, DetailsSummary } from '@tiptap/extension-details'
import Image from '@tiptap/extension-image'
import { TaskItem, TaskList } from '@tiptap/extension-list'
import Mention from '@tiptap/extension-mention'
import { Placeholder } from '@tiptap/extensions'
import { Plugin, PluginKey } from '@tiptap/pm/state'
import Suggestion, { type SuggestionOptions } from '@tiptap/suggestion'
import { VueNodeViewRenderer, type JSONContent } from '@tiptap/vue-3'
import type { Component } from 'vue'
import { fileUrl } from '@/api/pages'
import type { MentionRefs } from '@/types/work'
import { richTextExtensions } from './extensions'
import { suggestionRenderer, type SuggestionItem } from './suggestionRender'

/**
 * Nodes a page stores, which must match the page allowlist of the API
 * (richtext.CleanPage). fileUpload is the placeholder of a file being
 * uploaded: it never leaves the browser (pageForSave drops it).
 */
export const PAGE_NODES = [
  'doc', 'paragraph', 'heading', 'bulletList', 'orderedList', 'listItem', 'codeBlock', 'blockquote',
  'hardBreak', 'horizontalRule', 'table', 'tableRow', 'tableHeader', 'tableCell', 'text',
  'taskList', 'taskItem', 'details', 'detailsSummary', 'detailsContent', 'callout', 'image', 'fileAttachment', 'mention',
]
export const CLIENT_ONLY_NODES = ['fileUpload']

/** What the page extensions need from the screen that hosts the editor. */
export interface PageContext {
  t: (key: string, values?: Record<string, unknown>) => string
  /** The mentions as the API resolved them (RF-27 to RF-29). */
  refs: () => MentionRefs
  /** The @ search (RF-25) and the # search (RF-26); items carry the mention's kind. */
  searchPeople: (q: string) => Promise<SuggestionItem[]>
  searchElements: (q: string) => Promise<SuggestionItem[]>
  /** Opens the file picker; kind limits it to images. */
  pickFiles: (kind: 'image' | 'file') => void
  /** Uploads files dropped or pasted at pos. */
  upload: (files: File[], pos: number) => void
  /** Renders the placeholder of an upload in progress (FileBlock). */
  uploadView: Component
}

const Callout = Node.create({
  name: 'callout',
  group: 'block',
  content: 'block+',
  defining: true,
  parseHTML: () => [{ tag: 'div[data-callout]' }],
  renderHTML: ({ HTMLAttributes }) => ['div', mergeAttributes(HTMLAttributes, { 'data-callout': '', class: 'callout' }), 0],
})

// Only images of the page's own files: an image pasted from another site is
// dropped, the API would reject it (RF-14, ING-4).
const PageImage = Image.extend({
  addAttributes() {
    return {
      fileId: { default: null, parseHTML: (el) => el.getAttribute('data-file-id'), renderHTML: (a) => ({ 'data-file-id': a.fileId }) },
      src: { default: null, parseHTML: () => null, renderHTML: (a) => ({ src: a.fileId ? fileUrl(a.fileId) : null }) },
      alt: { default: null },
    }
  },
  parseHTML: () => [{ tag: 'img[data-file-id]' }],
}).configure({ HTMLAttributes: { class: 'page-image' } })

export function formatSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`
  return `${(bytes / 1024 / 1024).toFixed(1).replace('.', ',')} MB`
}

const FileAttachment = Node.create({
  name: 'fileAttachment',
  group: 'block',
  atom: true,
  addAttributes: () => ({
    fileId: { default: null, parseHTML: (el) => el.getAttribute('data-file-attachment') },
    name: { default: '', parseHTML: (el) => el.getAttribute('data-name') ?? '' },
    size: { default: 0, parseHTML: (el) => Number(el.getAttribute('data-size') ?? 0) },
  }),
  parseHTML: () => [{ tag: 'a[data-file-attachment]' }],
  renderHTML: ({ node }) => [
    'a',
    {
      href: fileUrl(node.attrs.fileId),
      download: node.attrs.name,
      'data-file-attachment': node.attrs.fileId,
      'data-name': node.attrs.name,
      'data-size': node.attrs.size,
      class: 'page-file',
      contenteditable: 'false',
    },
    `${node.attrs.name} · ${formatSize(node.attrs.size)}`,
  ],
})

function fileUpload(view: Component) {
  return Node.create({
    name: 'fileUpload',
    group: 'block',
    atom: true,
    selectable: false,
    addAttributes: () => ({ uploadId: { default: null } }),
    renderHTML: () => ['div', { 'data-file-upload': '' }],
    addNodeView: () => VueNodeViewRenderer(view),
  })
}

/** The page without the placeholders of uploads in progress, as it is saved. */
export function pageForSave(doc: JSONContent): JSONContent {
  return {
    ...doc,
    content: doc.content?.filter((c) => !CLIENT_ONLY_NODES.includes(c.type ?? '')).map(pageForSave),
  }
}

const slashKey = new PluginKey('slashCommand')
const userKey = new PluginKey('mentionUser')
const elementKey = new PluginKey('mentionElement')

const normalize = (s: string) => s.normalize('NFD').replace(/[̀-ͯ]/g, '').toLowerCase()

interface SlashItem {
  id: string
  label: string
  hint: string
  run: (editor: Editor, range: Range) => void
}

/** The blocks of the / menu (RF-7). */
function slashItems(ctx: PageContext): SlashItem[] {
  const block = (id: string, run: SlashItem['run']): SlashItem => ({
    id, label: ctx.t(`page.blocks.${id}`), hint: ctx.t(`page.blocks.${id}Hint`), run,
  })
  const chain = (editor: Editor, range: Range) => editor.chain().focus().deleteRange(range)
  return [
    block('text', (e, r) => chain(e, r).setParagraph().run()),
    block('heading1', (e, r) => chain(e, r).setHeading({ level: 1 }).run()),
    block('heading2', (e, r) => chain(e, r).setHeading({ level: 2 }).run()),
    block('heading3', (e, r) => chain(e, r).setHeading({ level: 3 }).run()),
    block('bulletList', (e, r) => chain(e, r).toggleBulletList().run()),
    block('orderedList', (e, r) => chain(e, r).toggleOrderedList().run()),
    block('taskList', (e, r) => chain(e, r).toggleTaskList().run()),
    block('toggle', (e, r) => chain(e, r).setDetails().run()),
    block('callout', (e, r) => chain(e, r).wrapIn('callout').run()),
    block('quote', (e, r) => chain(e, r).toggleBlockquote().run()),
    block('code', (e, r) => chain(e, r).toggleCodeBlock().run()),
    block('divider', (e, r) => chain(e, r).setHorizontalRule().run()),
    block('table', (e, r) => chain(e, r).insertTable({ rows: 3, cols: 3, withHeaderRow: true }).run()),
    block('image', (e, r) => {
      chain(e, r).run()
      ctx.pickFiles('image')
    }),
    block('file', (e, r) => {
      chain(e, r).run()
      ctx.pickFiles('file')
    }),
  ]
}

function slashCommand(ctx: PageContext) {
  const items = slashItems(ctx)
  return Extension.create({
    name: 'slashCommand',
    addProseMirrorPlugins() {
      const options: Omit<SuggestionOptions<SuggestionItem>, 'editor'> = {
        char: '/',
        pluginKey: slashKey,
        // At the start of a block or after a space (RF-7), never inside code.
        allowedPrefixes: [' '],
        allow: ({ state, range }) => !state.doc.resolve(range.from).parent.type.spec.code,
        items: ({ query }) => {
          const q = normalize(query)
          return items
            .filter((i) => normalize(i.label).includes(q) || i.id.toLowerCase().includes(q))
            .map((i) => ({ id: i.id, label: i.label, hint: i.hint }))
        },
        command: ({ editor, range, props }) => items.find((i) => i.id === props.id)?.run(editor, range),
        render: suggestionRenderer(slashKey, ctx.t('page.slash.label'), () => ctx.t('page.slash.empty')),
      }
      return [Suggestion({ editor: this.editor, ...options })]
    },
  })
}

const routeOf = (kind: string, id: string, label: string | undefined) => {
  switch (kind) {
    case 'work_item':
      return label ? `/work-items/${label.split(' ')[0]}` : null
    case 'milestone':
      return `/milestones/${id}`
    case 'project':
      return `/projects/${id}`
  }
  return null
}

function pageMention(ctx: PageContext) {
  // The text of a mention always comes from the API's refs: the label the
  // editor wrote is only a fallback until the page is saved (RF-27).
  const text = (attrs: Record<string, string>) => {
    const ref = ctx.refs()[attrs.id]
    if (ref?.state === 'no_access') return { text: ctx.t('page.mention.noAccess'), ok: false }
    if (ref?.state === 'deleted') return { text: ctx.t('page.mention.deleted'), ok: false }
    const prefix = attrs.kind === 'user' ? '@' : '#'
    return { text: prefix + (ref?.label ?? attrs.label ?? ''), ok: true, label: ref?.label ?? attrs.label }
  }
  return Mention.extend({
    addAttributes() {
      return {
        ...this.parent?.(),
        kind: { default: 'user', parseHTML: (el) => el.getAttribute('data-kind'), renderHTML: (a) => ({ 'data-kind': a.kind }) },
      }
    },
  }).configure({
    renderText: ({ node }) => text(node.attrs).text,
    renderHTML: ({ node }) => {
      const r = text(node.attrs)
      const href = r.ok ? routeOf(node.attrs.kind, node.attrs.id, r.label) : null
      const attrs = { class: r.ok ? 'mention' : 'mention mention-missing', 'data-type': 'mention', 'data-id': node.attrs.id, 'data-kind': node.attrs.kind }
      return href ? ['a', { ...attrs, href }, r.text] : ['span', attrs, r.text]
    },
    suggestions: [
      {
        char: '@',
        pluginKey: userKey,
        items: ({ query }) => ctx.searchPeople(query),
        render: suggestionRenderer(userKey, ctx.t('page.mention.people'), () => ctx.t('page.mention.empty')),
      },
      {
        char: '#',
        pluginKey: elementKey,
        items: ({ query }) => ctx.searchElements(query),
        render: suggestionRenderer(elementKey, ctx.t('page.mention.elements'), () => ctx.t('page.mention.empty')),
      },
    ],
  })
}

// Files pasted or dropped on the page are uploaded where they fell (RF-14,
// RF-15); text pasted along with them is kept.
function fileDrop(ctx: PageContext) {
  return Extension.create({
    name: 'fileDrop',
    addProseMirrorPlugins: () => [
      new Plugin({
        key: new PluginKey('fileDrop'),
        props: {
          handlePaste: (view, event) => {
            const files = Array.from(event.clipboardData?.files ?? [])
            if (!files.length) return false
            ctx.upload(files, view.state.selection.from)
            return !event.clipboardData?.getData('text/html') && !event.clipboardData?.getData('text/plain')
          },
          handleDrop: (view, event, _slice, moved) => {
            const files = Array.from(event.dataTransfer?.files ?? [])
            if (moved || !files.length) return false
            const pos = view.posAtCoords({ left: event.clientX, top: event.clientY })?.pos ?? view.state.selection.from
            ctx.upload(files, pos)
            return true
          },
        },
      }),
    ],
  })
}

/** The editor and viewer of pages share these extensions (spec 005). */
export function pageExtensions(ctx: PageContext): Extensions {
  return [
    ...richTextExtensions(),
    TaskList,
    TaskItem.configure({ nested: true }),
    // Toggles always open folded and their state is never saved (RF-4).
    Details.configure({ persist: false, HTMLAttributes: { class: 'details' } }),
    DetailsSummary,
    DetailsContent,
    Callout,
    PageImage,
    FileAttachment,
    fileUpload(ctx.uploadView),
    pageMention(ctx),
    slashCommand(ctx),
    fileDrop(ctx),
    // The help of an empty page, only for who can edit it.
    Placeholder.configure({
      placeholder: ({ editor }) => (editor.isEmpty ? ctx.t('page.placeholder') : ''),
      showOnlyWhenEditable: true,
    }),
  ]
}
