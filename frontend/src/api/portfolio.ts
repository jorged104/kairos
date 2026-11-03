import type { Lifecycle, Project } from '@/types/work'
import { api } from './client'
import { localDate } from './metrics'
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
