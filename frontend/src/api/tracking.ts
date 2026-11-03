import type { Ref } from '@/types/work'
import { api } from './client'
import { localDate } from './metrics'
import type { Figures } from './schedule'

// Follow-up of several projects (spec 011).

export type MilestoneState = 'done' | 'late' | 'at_risk' | 'future' | 'no_data' | 'running'
export const MILESTONE_STATES: MilestoneState[] = ['done', 'running', 'at_risk', 'late', 'future', 'no_data']

export interface TrackingMilestone {
  id: string
  name: string
  due_date: string | null
  status: 'abierto' | 'cumplido'
  /** From its work items; null without planned dates (Conceptos). */
  bar_start: string | null
  bar_end: string | null
  figures: Figures
  state: MilestoneState
}

export interface TrackingProject {
  id: string
  identifier: string
  name: string
  owner: Ref
  figures: Figures
  milestones: TrackingMilestone[]
}

export interface TrackingTask {
  id: string
  key: string
  title: string
  project_id: string
  parent_title: string | null
  ratio: number
  start_date: string | null
  due_date: string | null
  closed_on: string | null
  late_days?: number
}

export interface TrackingColumn {
  total: number
  items: TrackingTask[]
}

export interface Tracking {
  today: string
  /** Before the limit of 50 (RF-9). */
  matched: number
  /** Without filters, the caller's projects (RF-6). */
  mine: boolean
  projects: TrackingProject[]
  summary: { projects: number; milestones: number; met: number; figures: Figures }
  columns: { late: TrackingColumn; in_progress: TrackingColumn; done: TrackingColumn; upcoming: TrackingColumn }
}

export interface TrackingFilters {
  program_id?: string
  project_id?: string[]
  owner_id?: string[]
}

export const getTracking = (f: TrackingFilters) =>
  api<Tracking>('/api/tracking', { query: { today: localDate(), program_id: f.program_id, project_id: f.project_id, owner_id: f.owner_id } })
