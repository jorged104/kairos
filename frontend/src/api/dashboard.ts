import { api } from './client'
import { localDate } from './metrics'
import type { Figures } from './schedule'

// Mi panel (spec 013): the caller's own, or, read only, another person's for
// an admin or a lead (change of 2026-11-06, RF-37 to RF-51).

export interface DashboardRef {
  id: string
  name: string
}

export interface DashboardTask {
  id: string
  key: string
  title: string
  project: DashboardRef
  status_id: string
  /** Its colour marks the card (RF-14a). */
  type_id: string
  closed: boolean
  due_date: string | null
  closed_on: string | null
  late_days: number
  weight: number
  can_move: boolean
  /** Without a project: its team and mode, if any (spec 022, RF-30). */
  loose?: { team: DashboardRef | null; mode: '' | 'claim' | 'each' }
  /** In another person's panel, of a project the viewer does not see: no key, title nor project (spec 023, RF-26). */
  hidden?: boolean
}

export type DashboardRole = 'owner' | 'pm' | 'tech_lead' | 'member' | 'observer' | 'tasks'

export interface DashboardProject {
  id: string
  identifier: string
  name: string
  role: DashboardRole
  open: number
  late: number
  figures: Figures
}

export interface DashboardWeek {
  /** Monday. */
  start: string
  closed: number
  weight: number
  with_due: number
  on_time: number
}

export interface DashboardLoad {
  /** null: «Otros» (RF-34). */
  project: DashboardRef | null
  open: number
  late: number
}

/** Whose panel it is, only when it is another person's. */
export interface DashboardPerson {
  id: string
  name: string
  roles: string[]
  teams: DashboardRef[]
}

export interface Dashboard {
  user?: DashboardPerson
  today: string
  profile: { open: number; late: number; closed_month: number }
  tasks: DashboardTask[]
  projects: DashboardProject[]
  weeks: DashboardWeek[]
  load: DashboardLoad[]
}

export const getDashboard = (userId?: string) =>
  api<Dashboard>(userId ? `/api/users/${userId}/dashboard` : '/api/me/dashboard', { query: { today: localDate() } })

/** The other people whose panel the caller may see (RF-37). */
export const getDashboardPeople = () => api<DashboardRef[]>('/api/me/dashboard/people')
