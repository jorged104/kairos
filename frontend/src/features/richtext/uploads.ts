import type { Editor } from '@tiptap/core'
import { reactive } from 'vue'
import { ApiError } from '@/api/client'
import { uploadPageFile, type PageFile, type PageOwner } from '@/api/pages'

/** The limit of one file (RF-16), checked before sending it. */
export const MAX_FILE_BYTES = 10 * 1024 * 1024

/** Types the picker offers (RF-14, RF-15); the API decides by the content. */
export const IMAGE_TYPES = 'image/png,image/jpeg,image/gif,image/webp'
export const FILE_TYPES = `${IMAGE_TYPES},application/pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.csv,.zip`

/** An upload in progress, shown by FileBlock where it will land (RF-19, RF-20). */
export interface Upload {
  id: string
  name: string
  progress: number
  /** error.code of the failure, or null. */
  error: string | null
  retry: () => void
}

/** Uploads by id: the placeholder node only keeps the id. */
export const uploads = reactive(new Map<string, Upload>())

function findUpload(editor: Editor, id: string): { pos: number; size: number } | null {
  let found: { pos: number; size: number } | null = null
  editor.state.doc.descendants((node, pos) => {
    if (found) return false
    if (node.type.name === 'fileUpload' && node.attrs.uploadId === id) found = { pos, size: node.nodeSize }
    return !found
  })
  return found
}

/** Swaps the placeholder of an upload for the image or attachment it became. */
function land(editor: Editor, id: string, f: PageFile) {
  const at = findUpload(editor, id)
  uploads.delete(id)
  if (!at) return // the user removed it meanwhile
  const { schema } = editor.state
  const node =
    f.kind === 'image'
      ? schema.nodes.image.create({ fileId: f.id, alt: f.name })
      : schema.nodes.fileAttachment.create({ fileId: f.id, name: f.name, size: f.size })
  editor.view.dispatch(editor.state.tr.replaceWith(at.pos, at.pos + at.size, node))
}

/** Removes the placeholder of an upload, e.g. after it failed. */
export function removeUpload(editor: Editor, id: string) {
  const at = findUpload(editor, id)
  uploads.delete(id)
  if (at) editor.view.dispatch(editor.state.tr.delete(at.pos, at.pos + at.size))
}

/**
 * Starts uploading files at pos of the page of owner. Each one gets a
 * placeholder with its progress, replaced by its block when it is saved.
 */
export function startUploads(editor: Editor, owner: PageOwner, files: File[], pos: number) {
  let at = Math.min(pos, editor.state.doc.content.size)
  for (const file of files) {
    const id = crypto.randomUUID()
    const entry: Upload = reactive({
      id,
      name: file.name,
      progress: 0,
      error: null as string | null,
      retry: () => send(),
    })
    const send = () => {
      entry.error = null
      entry.progress = 0
      if (file.size > MAX_FILE_BYTES) {
        entry.error = 'file_too_large'
        return
      }
      uploadPageFile(owner, file, (p) => (entry.progress = p))
        .then((f) => land(editor, id, f))
        .catch((e: unknown) => (entry.error = e instanceof ApiError ? e.code : 'unknown'))
    }
    uploads.set(id, entry)
    editor.chain().insertContentAt(at, { type: 'fileUpload', attrs: { uploadId: id } }).run()
    at = Math.min(at + 1, editor.state.doc.content.size)
    send()
  }
}
