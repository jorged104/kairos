import type { RichDoc } from '@/types/work'
import { ApiError, api, errorFrom } from './client'

// Project reports written with AI (spec 020) and their templates.

export interface ReportTemplate {
  id: string
  name: string
  description: string
  /** Only for the system administrator. */
  prompt?: string
  archived: boolean
}

export interface ReportSummary {
  id: string
  project_id: string
  template_id: string | null
  template_name: string
  title: string
  author: { id: string; name: string }
  updated_by: { id: string; name: string }
  created_at: string
  updated_at: string
  version: number
}

export interface ReportCan {
  manage: boolean
  generate: boolean
}

export interface Report extends ReportSummary {
  content: string
  doc: RichDoc
  can: ReportCan
}

/** Where the saved content came from, for the audit (RF-70). */
export type ReportOrigin = 'ai' | 'ai_edited' | 'manual'

export interface ReportSave {
  template_id?: string | null
  title: string
  content: string
  origin: ReportOrigin
  version?: number
}

export const listReports = (projectId: string) => api<{ items: ReportSummary[]; can: ReportCan }>(`/api/projects/${projectId}/reports`)

export const getReport = (id: string) => api<Report>(`/api/reports/${id}`)

export const createReport = (projectId: string, body: ReportSave) => api<Report>(`/api/projects/${projectId}/reports`, { method: 'POST', body })

export const updateReport = (id: string, body: ReportSave) => api<Report>(`/api/reports/${id}`, { method: 'PUT', body })

export const deleteReport = (id: string) => api<void>(`/api/reports/${id}`, { method: 'DELETE' })

export const previewReport = (content: string) => api<{ doc: RichDoc }>('/api/reports/preview', { method: 'POST', body: { content } })

export const activeReportTemplates = () => api<{ items: ReportTemplate[] }>('/api/report-templates')

export const allReportTemplates = () => api<{ items: ReportTemplate[] }>('/api/system/report-templates')

export type ReportTemplateBody = { name: string; description: string; prompt: string }

export const createReportTemplate = (body: ReportTemplateBody) => api<ReportTemplate>('/api/system/report-templates', { method: 'POST', body })

export const updateReportTemplate = (id: string, body: ReportTemplateBody) =>
  api<ReportTemplate>(`/api/system/report-templates/${id}`, { method: 'PUT', body })

export const archiveReportTemplate = (id: string) => api<ReportTemplate>(`/api/system/report-templates/${id}/archive`, { method: 'POST' })

/** One line of the generation stream (RF-37 to RF-41). */
export type ReportEvent =
  | { type: 'todos'; todos: { content: string; status: 'pending' | 'in_progress' | 'completed' }[] }
  | { type: 'tool'; tool: string; project?: string }
  | { type: 'text'; delta: string }
  | { type: 'reset' }
  | { type: 'ping' }
  | { type: 'done' }
  | { type: 'error'; code: string }

export type GenerateBody = { template_id: string; instructions: string } | { draft: string; change: string; template_id?: string | null }

/**
 * Generates a report or a change and calls onEvent for each line as it
 * arrives. Failures before the stream are thrown as ApiError; the abort
 * signal cancels the generation (RF-40).
 */
export async function generateReport(projectId: string, body: GenerateBody, onEvent: (e: ReportEvent) => void, signal?: AbortSignal) {
  let res: Response
  try {
    res = await fetch(`/api/projects/${projectId}/reports/generate`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
      signal,
    })
  } catch (e) {
    if (signal?.aborted) throw new ApiError('cancelled', 0)
    throw e instanceof ApiError ? e : new ApiError('network_error', 0)
  }
  if (!res.ok || !res.body) throw errorFrom(res.status, await res.json().catch(() => null))
  const reader = res.body.getReader()
  const decoder = new TextDecoder()
  let pending = ''
  const flush = (line: string) => {
    if (!line.trim()) return
    try {
      onEvent(JSON.parse(line) as ReportEvent)
    } catch {
      // A broken line is skipped; the stream goes on.
    }
  }
  try {
    for (;;) {
      const { value, done } = await reader.read()
      if (done) break
      pending += decoder.decode(value, { stream: true })
      const lines = pending.split('\n')
      pending = lines.pop() ?? ''
      lines.forEach(flush)
    }
    flush(pending + decoder.decode())
  } catch {
    throw new ApiError(signal?.aborted ? 'cancelled' : 'network_error', 0)
  }
}
