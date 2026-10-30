import type { MentionRefs, Ref, RichDoc } from '@/types/work'
import type { SavedPage } from './pages'
import { api } from './client'

// Comments of work items (spec 006).

export interface WorkComment {
  id: string
  author: Ref
  /** null once deleted (RF-33). */
  body: RichDoc
  created_at: string
  edited_at: string | null
  deleted: boolean
  can: { edit: boolean; delete: boolean }
}

export interface CommentThread extends WorkComment {
  /** The commented text of an anchored thread (RF-48). */
  quote: string | null
  anchored: boolean
  /** The commented text is no longer in the page (RF-51). */
  anchor_deleted: boolean
  resolved_at: string | null
  resolved_by: Ref | null
  replies: WorkComment[]
}

export interface CommentList {
  threads: CommentThread[]
  refs: MentionRefs
  can_comment: boolean
}

/** Every change answers with the comments as they are now (RF-43). */
export interface CommentResult {
  comments: CommentList
  /** The page with the new anchor, when a thread was anchored. */
  page?: SavedPage
}

export interface AnchorInput {
  thread_id: string
  quote: string
  page: RichDoc
  page_version: number
}

export const listComments = (workItemId: string) => api<CommentList>(`/api/work-items/${workItemId}/comments`)

export const createComment = (workItemId: string, body: RichDoc, opts: { threadId?: string; anchor?: AnchorInput } = {}) =>
  api<CommentResult>(`/api/work-items/${workItemId}/comments`, {
    method: 'POST',
    body: { body, ...(opts.threadId ? { thread_id: opts.threadId } : {}), ...(opts.anchor ? { anchor: opts.anchor } : {}) },
  })

export const updateComment = (id: string, body: RichDoc) => api<CommentResult>(`/api/comments/${id}`, { method: 'PATCH', body: { body } })

export const deleteComment = (id: string) => api<CommentResult>(`/api/comments/${id}`, { method: 'DELETE' })

export const setThreadResolved = (id: string, resolved: boolean) =>
  api<CommentResult>(`/api/comments/${id}/${resolved ? 'resolve' : 'reopen'}`, { method: 'POST' })

/** Where the files of the comments of a work item are uploaded (RF-26). */
export const commentFilesPath = (workItemId: string) => `/api/work-items/${workItemId}/comment-files`
