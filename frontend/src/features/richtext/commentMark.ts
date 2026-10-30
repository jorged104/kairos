import { Mark, mergeAttributes } from '@tiptap/core'
import { Fragment, Slice, type MarkType } from '@tiptap/pm/model'
import { Plugin, PluginKey, type EditorState } from '@tiptap/pm/state'
import type { JSONContent } from '@tiptap/vue-3'

/** The mark that anchors a comment thread to a piece of text (spec 006, RF-45). */
export const COMMENT_MARK = 'comment'

function strip(content: Fragment, type: MarkType): Fragment {
  const nodes: import('@tiptap/pm/model').Node[] = []
  content.forEach((n) => nodes.push(n.isText ? n.mark(type.removeFromSet(n.marks)) : n.copy(strip(n.content, type))))
  return Fragment.from(nodes)
}

/**
 * The anchor of a thread. It doesn't grow when typing at its edges, several
 * can cover the same text (RF-54) and pasted text never brings one: the
 * thread stays with the original text (RF-53).
 */
export const CommentMark = Mark.create({
  name: COMMENT_MARK,
  inclusive: false,
  excludes: '',
  addAttributes() {
    return {
      threadId: {
        default: null,
        parseHTML: (el) => el.getAttribute('data-comment'),
        renderHTML: (attrs) => ({ 'data-comment': attrs.threadId }),
      },
    }
  },
  parseHTML() {
    return [{ tag: 'span[data-comment]' }]
  },
  renderHTML({ HTMLAttributes }) {
    return ['span', mergeAttributes(HTMLAttributes, { class: 'page-comment' }), 0]
  },
  addProseMirrorPlugins() {
    const type = this.type
    return [
      new Plugin({
        key: new PluginKey('commentPaste'),
        props: { transformPasted: (slice) => new Slice(strip(slice.content, type), slice.openStart, slice.openEnd) },
      }),
    ]
  },
})

/** A node as JSON without anchors, to duplicate a block (RF-53). */
export function withoutComments(json: JSONContent): JSONContent {
  const marks = json.marks?.filter((m) => m.type !== COMMENT_MARK)
  return {
    ...json,
    ...(json.marks ? { marks: marks?.length ? marks : undefined } : {}),
    ...(json.content ? { content: json.content.map(withoutComments) } : {}),
  }
}

/** The threads anchored at pos, for a click on a highlight (RF-47, RF-54). */
export function threadsAt(state: EditorState, pos: number): string[] {
  const $pos = state.doc.resolve(pos)
  const marks = [...($pos.nodeAfter?.marks ?? []), ...($pos.nodeBefore?.marks ?? [])]
  return [...new Set(marks.filter((m) => m.type.name === COMMENT_MARK).map((m) => String(m.attrs.threadId)))]
}
