import type { PortfolioCard } from './portfolio'
import type { Figures, Level } from './schedule'
import type { Project, Ref, WorkNode } from '@/types/work'
import { localDate } from './metrics'
import { api } from './client'

// The screens of a project (spec 008): overview, filters and views, team.

export type ProjectRole = 'pm' | 'tech_lead' | 'member' | 'observer'
export const PROJECT_ROLES: ProjectRole[] = ['pm', 'tech_lead', 'member', 'observer']

/** The filters as the API's query parameters; a list is an «or» (RF-20). */
export type Filters = Record<string, string[]>

export interface ViewConfig {
  filters: Filters
  sort: string
  group: '' | 'milestone' | 'status' | 'assignee' | 'type'
}

export interface SavedView {
  id: string
  name: string
  shared: boolean
  mine: boolean
  can_edit: boolean
  config: ViewConfig
  count: number
  /** Filter values that no longer exist or are retired (RF-31). */
  unavailable: { field: string; value: string }[]
}

export interface ViewList {
  items: SavedView[]
  all_count: number
}

export const listViews = (projectId: string) => api<ViewList>(`/api/projects/${projectId}/views`)
export const createView = (projectId: string, input: { name: string; shared: boolean; config: ViewConfig }) =>
  api<SavedView>(`/api/projects/${projectId}/views`, { method: 'POST', body: input })
export const updateView = (id: string, input: { name: string; shared: boolean; config: ViewConfig }) =>
  api<SavedView>(`/api/views/${id}`, { method: 'PATCH', body: input })
export const deleteView = (id: string) => api<void>(`/api/views/${id}`, { method: 'DELETE' })

/** The work items that match some filters, every one (RF-22). */
export const filteredTree = (projectId: string, filters: Filters, sort = '') =>
  api<{ items: WorkNode[] }>(`/api/projects/${projectId}/tree`, { query: { ...filters, sort: sort || undefined, today: localDate() } }).then(
    (r) => r.items,
  )

export interface Member {
  user: Ref
  status: string
  role: ProjectRole
  owner: boolean
  in_team: boolean
  explicit: boolean
  can_work: boolean
  capacity: number | null
  load: { project: number; total: number; items: number; overloaded: boolean }
}

const items = <T>(r: { items: T[] }) => r.items

export const listMembers = (projectId: string) => api<{ items: Member[] }>(`/api/projects/${projectId}/members`).then(items)
export const addMember = (projectId: string, userId: string, role: ProjectRole) =>
  api<{ items: Member[] }>(`/api/projects/${projectId}/members`, { method: 'POST', body: { user_id: userId, role } }).then(items)
export const setMemberRole = (projectId: string, userId: string, role: ProjectRole) =>
  api<{ items: Member[] }>(`/api/projects/${projectId}/members/${userId}`, { method: 'PATCH', body: { role } }).then(items)
export const removeMember = (projectId: string, userId: string) =>
  api<{ items: Member[] }>(`/api/projects/${projectId}/members/${userId}`, { method: 'DELETE' }).then(items)
export const setCapacity = (userId: string, capacity: number | null) =>
  api<unknown>(`/api/users/${userId}/capacity`, { method: 'PUT', body: { capacity } })

export interface Suggestion {
  id: string
  key: string
  title: string
  weight: number
  due_date: string | null
  label_ids: string[]
  people: Ref[]
}
export const listSuggestions = (projectId: string) =>
  api<{ items: Suggestion[] }>(`/api/projects/${projectId}/assignment-suggestions`).then(items)

export interface MilestoneLine {
  id: string
  name: string
  due_date: string | null
  status: 'abierto' | 'cumplido' | 'cancelado'
  percent: number
  closed: number
  total: number
  days_left?: number
  /** Spec 009, RF-45. */
  figures: Figures
}

export interface Overview {
  today: string
  progress: number
  /** Plan, real, gap and level (spec 009, RF-44); expected and deviation are the same rounded. */
  figures: Figures
  /** The plan of each day for the chart (RF-46). */
  plan_series: { day: string; plan: number }[]
  has_plan: boolean
  expected: number
  deviation: number
  statuses: { status_id: string; count: number }[]
  total: number
  closed: number
  overdue: number
  blocked: number
  next_milestone: MilestoneLine | null
  milestones: MilestoneLine[]
  risks: { id: string; key: string; title: string; reason: 'overdue' | 'blocked' | 'behind'; days: number; deviation: number; gap: number; level?: Level; due_date: string | null; assignees: Ref[] }[]
  load: Member[]
  history: { day: string; progress: number }[]
  /** The aggregate and the direct children of a program (spec 010, RF-39, RF-40). */
  program?: PortfolioCard
}

export const getOverview = (projectId: string) => api<Overview>(`/api/projects/${projectId}/overview`, { query: { today: localDate() } })

export const setColumnLimit = (projectId: string, statusId: string, limit: number | null) =>
  api<Project>(`/api/projects/${projectId}/column-limits/${statusId}`, { method: 'PUT', body: { limit } })
