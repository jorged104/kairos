import type { Health, Lifecycle, Project } from '@/types/work'
import { api } from './client'
import { localDate } from './metrics'
import type { Figures } from './schedule'

// The project list in its three modes and its saved views (spec 012).

export interface BoardRef {
  id: string
  name: string
}

export interface BoardMilestone {
  id: string
  name: string
  due_date: string
  done: boolean
}

/** A project as the Kanban, the Timeline and the Table draw it. */
export interface BoardProject {
  id: string
  identifier: string
  name: string
  status_id: string
  lifecycle: Lifecycle
  health: Health
  team: BoardRef
  owner: BoardRef
  is_program: boolean
  /** The nearest program above it. */
  program: BoardRef | null
  /** Every program above it (RF-67). */
  program_ids: string[]
  start_date: string | null
  end_date: string | null
  /** The dates of its last baseline (RF-49). */
  base_start: string | null
  base_end: string | null
  version: number
  can_manage: boolean
  /** A program's are its aggregate (spec 010); a project's, its own. */
  figures: Figures
  milestones: BoardMilestone[]
}

export interface BoardProgram {
  id: string
  name: string
  figures: Figures
}

export interface Board {
  today: string
  projects: BoardProject[]
  programs: BoardProgram[]
}

export const getBoard = () => api<Board>('/api/project-board', { query: { today: localDate() } })

/** Moves a project to another status; a closed one closes it (RF-17 to RF-20). */
export const setProjectStatus = (id: string, version: number, statusId: string) =>
  api<Project>(`/api/projects/${id}/status`, { method: 'PUT', body: { version, status_id: statusId } })

export const MODES = ['kanban', 'timeline', 'table'] as const
export type Mode = (typeof MODES)[number]

/** What a view keeps (RF-70). */
export interface ListViewConfig {
  mode: Mode | ''
  filters: Record<string, string[]>
  sort: string
  columns: string
  group: string
}

export interface ListView {
  id: string
  name: string
  shared: boolean
  mine: boolean
  can_edit: boolean
  config: ListViewConfig
}

export const listListViews = () => api<{ items: ListView[] }>('/api/project-views')

export const createListView = (body: { name: string; shared: boolean; config: ListViewConfig }) =>
  api<ListView>('/api/project-views', { method: 'POST', body })

export const updateListView = (id: string, body: { name: string; shared: boolean; config: ListViewConfig }) =>
  api<ListView>(`/api/project-views/${id}`, { method: 'PATCH', body })

export const deleteListView = (id: string) => api<void>(`/api/project-views/${id}`, { method: 'DELETE' })
