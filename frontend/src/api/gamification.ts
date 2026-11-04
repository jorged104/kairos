import { api } from './client'
import { localDate } from './metrics'

// Gamification (spec 014): points, levels and team rankings.

export interface Level {
  id: string
  number: number
  name: string
  min_xp: number
}

export interface PointsSummary {
  xp: number
  coins: number
  level: Level
  next: Level | null
  last_seq: number
}

export type MovementReason = 'close' | 'reopen' | 'adjust'

export interface Movement {
  id: string
  seq: number
  xp: number
  coins: number
  reason: MovementReason
  note: string | null
  created_at: string
  work_item: { id: string; key: string; title: string } | null
}

export interface MovementPage {
  items: Movement[]
  total: number
  page: number
  page_size: number
}

export interface PointValue {
  xp: number
  coins: number
}

export interface SizeValue extends PointValue {
  size_id: string
  name: string
  retired: boolean
}

export interface PointValues {
  no_size: PointValue
  sizes: SizeValue[]
}

export type RankingPeriod = 'week' | 'month' | 'all'

export interface Ranking {
  team: { id: string; name: string }
  period: RankingPeriod
  from: string | null
  to: string | null
  items: { position: number; user: { id: string; name: string }; level: Level; xp: number }[]
}

// Someone else's points go through the administration routes (RF-40).
const base = (userId?: string) => (userId ? `/api/gamification/users/${userId}` : '/api/me/points')

export const getPoints = (userId?: string) => api<PointsSummary>(userId ? `${base(userId)}/points` : base())

export const getMovements = (opts: { userId?: string; page?: number; afterSeq?: number } = {}) =>
  api<MovementPage>(`${base(opts.userId)}/movements`, { query: { page: opts.page, after_seq: opts.afterSeq } })

export const adjustPoints = (userId: string, body: { xp: number; coins: number; note: string }) =>
  api<PointsSummary>(`/api/gamification/users/${userId}/adjustments`, { method: 'POST', body })

export const getPointValues = () => api<PointValues>('/api/gamification/values')

export const setPointValues = (body: { no_size: PointValue; sizes: { size_id: string; xp: number; coins: number }[] }) =>
  api<PointValues>('/api/gamification/values', { method: 'PUT', body })

export const listLevels = () => api<{ items: Level[] }>('/api/gamification/levels').then((r) => r.items)

export const createLevel = (body: { name: string; min_xp: number }) => api<Level>('/api/gamification/levels', { method: 'POST', body })

export const updateLevel = (id: string, body: { name: string; min_xp: number }) =>
  api<Level>(`/api/gamification/levels/${id}`, { method: 'PATCH', body })

export const deleteLevel = (id: string) => api<void>(`/api/gamification/levels/${id}`, { method: 'DELETE' })

export const getRanking = (teamId: string, period: RankingPeriod, today = localDate()) =>
  api<Ranking>(`/api/gamification/teams/${teamId}/ranking`, { query: { period, today } })
