import { api } from './client'
import { localDate } from './metrics'
import type { Figures } from './schedule'

// Mi panel (spec 013): the caller's own data only.

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

export interface Dashboard {
  today: string
  profile: { open: number; late: number; closed_month: number }
  tasks: DashboardTask[]
  projects: DashboardProject[]
  weeks: DashboardWeek[]
  load: DashboardLoad[]
}

export const getDashboard = () => api<Dashboard>('/api/me/dashboard', { query: { today: localDate() } })
