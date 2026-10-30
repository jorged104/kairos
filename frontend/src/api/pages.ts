import type { MentionRefs, RichDoc } from '@/types/work'
import { ApiError, api, errorFrom } from './client'

// Pages of work items, milestones and projects (spec 005).

export type PageOwnerKind = 'work-items' | 'milestones' | 'projects'

export interface PageOwner {
  kind: PageOwnerKind
  id: string
  /** Where its files go when not to its page, as for comments (spec 006). */
  filesPath?: string
}

export interface SavedPage {
  page: RichDoc
  page_version: number
  version: number
  refs: MentionRefs
}

export interface PageFile {
  id: string
  kind: 'image' | 'file'
  name: string
  size: number
  content_type: string
}

export interface MentionResult {
  kind: 'work_item' | 'milestone' | 'project'
  id: string
  label: string
}

export interface MentionResults {
  work_items: MentionResult[]
  milestones: MentionResult[]
  projects: MentionResult[]
}

const ownerPath = (o: PageOwner) => `/api/${o.kind}/${o.id}`

/** The autosave: compares only the page version (RF-34, RF-59). */
export const savePage = (o: PageOwner, page: RichDoc, pageVersion: number, init: { keepalive?: boolean } = {}) =>
  init.keepalive
    ? saveKeepalive(o, page, pageVersion)
    : api<SavedPage>(`${ownerPath(o)}/page`, { method: 'PUT', body: { page, page_version: pageVersion } })

// When the tab closes the page is sent with keepalive, which only fetch has (RF-32).
async function saveKeepalive(o: PageOwner, page: RichDoc, pageVersion: number): Promise<SavedPage> {
  const res = await fetch(`${ownerPath(o)}/page`, {
    method: 'PUT',
    credentials: 'same-origin',
    keepalive: true,
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ page, page_version: pageVersion }),
  }).catch(() => {
    throw new ApiError('network_error', 0)
  })
  const body: unknown = await res.json().catch(() => null)
  if (!res.ok) throw errorFrom(res.status, body)
  return body as SavedPage
}

/** Where the viewer reads a file: the API checks the session and the access (RF-23). */
export const fileUrl = (id: string) => `/api/files/${id}`

/**
 * Uploads one file of a page, reporting progress from 0 to 1 (RF-19). It
 * uses XMLHttpRequest because fetch has no upload progress.
 */
export function uploadPageFile(o: PageOwner, file: File, onProgress: (ratio: number) => void): Promise<PageFile> {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', o.filesPath ?? `${ownerPath(o)}/files`)
    xhr.withCredentials = true
    xhr.responseType = 'json'
    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(e.loaded / e.total)
    }
    xhr.onload = () => {
      if (xhr.status === 201) resolve(xhr.response as PageFile)
      else reject(errorFrom(xhr.status, xhr.response))
    }
    xhr.onerror = () => reject(new ApiError('network_error', 0))
    const form = new FormData()
    form.append('file', file)
    xhr.send(form)
  })
}

/** The # search of work items, milestones and projects (RF-26). */
export const searchMentions = (q: string) => api<MentionResults>('/api/mentions', { query: { q } })
