import type { Ref } from '@/types/work'
import { api } from './client'

// Metrics and history of a work item (spec 007).

export interface Share {
  no_weight: boolean
  percent: number
  points: number
}

export interface WorkMetrics {
  today: string
  /** null while closed (RF-15). */
  schedule: { has_plan: boolean; expected: number; actual: number; deviation: number; state: 'ahead' | 'on_track' | 'behind' | '' } | null
  contribution: { tracking: boolean; parent: Share | null; project: Share }
  /** In days; null is «Sin datos» or, for the cycle time, «—». */
  times: {
    closed: boolean
    lead_days: number | null
    cycle_days: number | null
    age_days: number | null
    in_progress_days: number | null
    in_status_days: number | null
    missing: boolean
  }
  due: { state: 'none' | 'pending' | 'overdue' | 'on_time' | 'late'; days: number }
  reopens: number | null
  blocked: { now: boolean; blockers: { hidden: boolean; id?: string; key?: string; title?: string }[]; total_days: number | null }
}

/** The local date, as the metrics count days (RF-10). */
export function localDate(d = new Date()): string {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

export const getMetrics = (id: string, today = localDate()) => api<WorkMetrics>(`/api/work-items/${id}/metrics`, { query: { today } })

/** A named value of the history; an empty name is something deleted. */
export interface Named {
  id: string
  name: string
}
export type HistoryValue = string | number | boolean | null | Named | Named[]

export interface HistoryEntry {
  at: string
  actor: Ref
  kind: 'created' | 'updated' | 'description' | 'dependency_added' | 'dependency_removed'
  changes: { field: string; from: HistoryValue; to: HistoryValue }[]
}

export interface HistoryPage {
  entries: HistoryEntry[]
  next: number | null
}

export const getHistory = (id: string, before?: number) =>
  api<HistoryPage>(`/api/work-items/${id}/history`, { query: { before: before === undefined ? undefined : String(before) } })
