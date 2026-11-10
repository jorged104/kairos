import type { Lifecycle, Project } from '@/types/work'
import { api } from './client'
import { localDate } from './metrics'
import { createProject, getProject, updateProject, type ProjectCreateInput } from './work'
import type { Figures } from './schedule'

// Programs and portfolio (spec 010).

/** A direct child of a program, or the program itself at the head of its card. */
export interface PortfolioRow {
  id: string
  identifier: string
  name: string
  lifecycle: Lifecycle
  is_program: boolean
  weight: number
  /** A program's are its aggregate (RF-27); a project's, its own (RF-26). */
  figures: Figures
}

export interface PortfolioCard {
  /** null: «Sin programa» (RF-28). */
  program: PortfolioRow | null
  projects: number
  below_plan: number
  rows: PortfolioRow[]
  /** Projects of the program the user does not see (spec 023, RF-19). */
  hidden?: number
}

export interface Portfolio {
  today: string
  programs: PortfolioCard[]
  loose: PortfolioCard
}

export const getPortfolio = () => api<Portfolio>('/api/portfolio', { query: { today: localDate() } })

/** For the administrator; a null weight is the project's effort (RF-1 to RF-6). */
export const setPortfolio = (id: string, body: { version: number; is_program: boolean; weight: number | null }) =>
  api<Project>(`/api/projects/${id}/portfolio`, { method: 'PUT', body })

/** A blank project marked as program at once (RF-42). */
export async function createProgram(input: ProjectCreateInput): Promise<Project> {
  const p = await createProject(input)
  return setPortfolio(p.id, { version: p.version, is_program: true, weight: null })
}

/** Hangs a project under a program, or takes it out with null (RF-45, RF-47).
 * The PATCH replaces every editable field, so the rest is read fresh. */
export async function setParent(id: string, parentId: string | null): Promise<Project> {
  const cur = await getProject(id)
  return updateProject(id, {
    version: cur.version, name: cur.name, team_id: cur.team.id, owner_id: cur.owner.id, parent_id: parentId,
    start_date: cur.start_date, end_date: cur.end_date, health: cur.health,
    health_explanation: cur.health_explanation, custom_values: cur.custom_values,
  })
}
