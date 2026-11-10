import { api } from './client'
import type { Ref, WorkItem } from '@/types/work'

// Work items without a project (spec 022): for a person, or for a team where
// one person takes it or each member does their own.

export type LooseMode = 'claim' | 'each'

export interface LooseInput {
  title: string
  due_date?: string | null
  weight?: number | null
  priority_id?: string | null
  /** One of the two (RF-3). */
  user_id?: string
  team_id?: string
  mode?: LooseMode
}

export interface Claimable {
  id: string
  key: string
  title: string
  due_date: string | null
  weight: number
  team: Ref
  author: Ref
}

export interface TeamTask {
  id: string
  key: string
  title: string
  due_date: string | null
  mode: LooseMode
  team: Ref
  closed: boolean
  created_at: string
  /** Who has it, or each member and whether they did theirs. */
  people: { id: string; name: string; done: boolean }[]
  done: number
  total: number
}

export const createLoose = (input: LooseInput) => api<WorkItem>('/api/loose-tasks', { method: 'POST', body: input })
export const getClaimable = () => api<{ items: Claimable[] }>('/api/me/claimable-tasks').then((r) => r.items)
export const getTeamTasks = () => api<{ items: TeamTask[] }>('/api/me/team-tasks').then((r) => r.items)
export const claimTask = (id: string) => api<WorkItem>(`/api/work-items/${id}/claim`, { method: 'POST' })
export const releaseTask = (id: string) => api<WorkItem>(`/api/work-items/${id}/release`, { method: 'POST' })
export const moveToProject = (id: string, projectId: string, version: number) =>
  api<WorkItem>(`/api/work-items/${id}/move`, { method: 'POST', body: { project_id: projectId, version } })
