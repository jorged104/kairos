import type { Ref } from '@/types/work'
import { api } from './client'
import { localDate } from './metrics'

// Plan, baseline and traffic light (spec 009).

export type Level = 'ahead' | 'on_track' | 'at_risk' | 'late'
export const LEVELS: Level[] = ['ahead', 'on_track', 'at_risk', 'late']

/** The closed palette of the levels (RF-40). */
export const PALETTE = ['blue', 'green', 'amber', 'red', 'orange', 'violet', 'teal', 'slate'] as const
export type PaletteColor = (typeof PALETTE)[number]

/** Plan, real and gap in points; without a plan only real counts (RF-35). */
export interface Figures {
  has_plan: boolean
  plan: number
  real: number
  gap: number
  level?: Level
}

export interface ScheduleSettings {
  upper: number
  risk: number
  late: number
  /** A null name is the default one of the translations (RF-39). */
  levels: Record<Level, { name: string | null; color: PaletteColor }>
  version: number
}

export const getScheduleSettings = () => api<ScheduleSettings>('/api/schedule-settings')
export const updateScheduleSettings = (s: ScheduleSettings) => api<ScheduleSettings>('/api/schedule-settings', { method: 'PUT', body: s })

export interface Baseline {
  version: number
  reason: string | null
  created_at: string
  author: Ref
}

export interface ItemSchedule {
  id: string
  figures: Figures
  in_baseline: boolean
  base_start: string | null
  base_due: string | null
  changed: boolean
  shift_days: number | null
}

export interface ProjectSchedule {
  today: string
  baseline: Baseline | null
  versions: Baseline[]
  items: ItemSchedule[]
}

export const getProjectSchedule = (projectId: string) =>
  api<ProjectSchedule>(`/api/projects/${projectId}/schedule`, { query: { today: localDate() } })

export const setBaseline = (projectId: string, reason: string | null) =>
  api<Baseline>(`/api/projects/${projectId}/baseline`, { method: 'POST', body: { reason } })

// Spec 024: two versions of the baseline, or one against today.
export type Change = 'added' | 'removed' | 'moved' | 'unchanged'
export const CHANGES: Change[] = ['added', 'removed', 'moved', 'unchanged']

export interface CompareItem {
  id: string | null // null: deleted or in a project one does not see
  key: string
  title: string
  change: Change
  before_start: string | null
  before_due: string | null
  after_start: string | null
  after_due: string | null
  start_shift: number | null
  due_shift: number | null
  deleted: boolean
  elsewhere: boolean
  hidden: boolean
}

export interface BaselineComparison {
  before: Baseline
  after: Baseline | null // null: today
  versions: Baseline[]
  summary: {
    added: number
    removed: number
    moved: number
    unchanged: number
    before_end: string | null
    after_end: string | null
    end_shift: number | null
  }
  items: CompareItem[]
}

// from and to: a version number or "today"; both empty pick the default ones.
export const compareBaselines = (projectId: string, from?: string, to?: string) =>
  api<BaselineComparison>(`/api/projects/${projectId}/baseline/compare`, { query: from && to ? { from, to } : {} })
