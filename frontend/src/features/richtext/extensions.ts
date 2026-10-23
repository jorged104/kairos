import type { Extensions } from '@tiptap/vue-3'
import { TableKit } from '@tiptap/extension-table'
import StarterKit from '@tiptap/starter-kit'

/** Links the server accepts (RF-71); anything else is never turned into a link. */
export const SAFE_LINK = /^(https?:\/\/|mailto:)/i

/**
 * The editor and the viewer share these extensions, which must match the
 * allowlist of the API (internal/richtext). The API rebuilds what it stores,
 * so this is a convenience for the user, not the security boundary.
 */
export function richTextExtensions(): Extensions {
  return [
    StarterKit.configure({
      heading: { levels: [1, 2, 3] },
      link: {
        openOnClick: false,
        autolink: true,
        protocols: ['http', 'https', 'mailto'],
        defaultProtocol: 'https',
        isAllowedUri: (url) => SAFE_LINK.test(url),
        HTMLAttributes: { rel: 'noopener noreferrer nofollow', target: '_blank' },
      },
    }),
    TableKit.configure({ table: { resizable: false } }),
  ]
}

/** Nodes and marks the API accepts; the spec of the editor checks against them. */
export const ALLOWED_NODES = [
  'doc', 'paragraph', 'heading', 'bulletList', 'orderedList', 'listItem', 'codeBlock', 'blockquote',
  'hardBreak', 'horizontalRule', 'table', 'tableRow', 'tableHeader', 'tableCell', 'text',
]
export const ALLOWED_MARKS = ['bold', 'italic', 'strike', 'underline', 'code', 'link']
