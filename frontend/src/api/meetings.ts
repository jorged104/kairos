import type { MentionRefs, RichDoc } from '@/types/work'
import { api } from './client'

// Meetings of projects and programs, and their templates (spec 018).

export interface MeetingTask {
  id: string
  key: string
  title: string
  status_id: string
}

export interface Meeting {
  id: string
  owner: { type: 'project'; id: string }
  project: { id: string; identifier: string; name: string }
  title: string
  starts_at: string
  location: string
  description: RichDoc
  page_version: number
  refs?: MentionRefs
  version: number
  template_id: string | null
  created_at: string
  updated_at: string
  tasks: MeetingTask[]
  can: { write: boolean; comment: boolean }
}

export interface MeetingSummary {
  id: string
  title: string
  starts_at: string
  location: string
  tasks: number
}

export interface MeetingPage {
  items: MeetingSummary[]
  total: number
  page: number
  page_size: number
}

export interface MeetingTemplate {
  id: string
  project_id: string | null
  scope: 'global' | 'project'
  name: string
  title: string
  location: string
  page: RichDoc
  position: number
  retired: boolean
}

export interface MeetingInput {
  title: string
  starts_at: string
  location?: string
  template_id?: string
  description?: RichDoc
}

export interface TemplateInput {
  project_id?: string
  name: string
  title: string
  location: string
  page: RichDoc
  position?: number
}

export const listMeetings = (projectId: string, when: 'upcoming' | 'past', page = 1, pageSize = 50) =>
  api<MeetingPage>(`/api/projects/${projectId}/meetings`, { query: { when, page, page_size: pageSize } })

export const createMeeting = (projectId: string, body: MeetingInput) =>
  api<Meeting>(`/api/projects/${projectId}/meetings`, { method: 'POST', body })

export const getMeeting = (id: string) => api<Meeting>(`/api/meetings/${id}`)

export const updateMeeting = (id: string, body: { version: number; title: string; starts_at: string; location: string }) =>
  api<Meeting>(`/api/meetings/${id}`, { method: 'PATCH', body })

export const deleteMeeting = (id: string) => api<void>(`/api/meetings/${id}`, { method: 'DELETE' })

export const chooseTemplates = (projectId: string) =>
  api<{ items: MeetingTemplate[] }>('/api/meeting-templates', { query: { project_id: projectId } })

export const globalTemplates = () => api<{ items: MeetingTemplate[] }>('/api/meeting-templates/global')

export const createTemplate = (body: TemplateInput) => api<MeetingTemplate>('/api/meeting-templates', { method: 'POST', body })

export const updateTemplate = (id: string, body: TemplateInput) =>
  api<MeetingTemplate>(`/api/meeting-templates/${id}`, { method: 'PATCH', body })

export const setTemplateRetired = (id: string, retired: boolean) =>
  api<MeetingTemplate>(`/api/meeting-templates/${id}/${retired ? 'retire' : 'restore'}`, { method: 'POST' })

export const deleteTemplate = (id: string) => api<void>(`/api/meeting-templates/${id}`, { method: 'DELETE' })

/** The comments of a meeting go through the same components as a work item's (RF-20). */
export const meetingCommentTarget = (id: string) => `meeting:${id}`
