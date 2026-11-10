import { api } from './client'
import { localDate } from './metrics'

// Cronograma por persona (spec 021): of a project's team and of Mi equipo.

export interface WorkloadBar {
  id: string
  key: string
  title: string
  project: { id: string; identifier: string; name: string }
  start: string
  end: string
  done_ratio: number
  overdue: boolean
  /** Without a project (spec 022, RF-36). */
  loose: boolean
}

export interface WorkloadRow {
  user: { id: string; name: string }
  /** null: free already (RF-3). */
  free_from: string | null
  open: number
  weight: number
  /** Open work items without a due date, not drawn (RF-4). */
  undated: number
  bars: WorkloadBar[]
}

export interface TeamMember extends WorkloadRow {
  teams: { id: string; name: string }[]
  capacity: number | null
  overloaded: boolean
}

export interface MyTeam {
  today: string
  teams: { id: string; name: string }[]
  people: TeamMember[]
}

export const getProjectTeamSchedule = (projectId: string) =>
  api<{ items: WorkloadRow[] }>(`/api/projects/${projectId}/team-schedule`, { query: { today: localDate() } }).then((r) => r.items)

export const getMyTeam = () => api<MyTeam>('/api/me/team', { query: { today: localDate() } })
