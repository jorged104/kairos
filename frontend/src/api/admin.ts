import type {
  Attribute,
  AttributeOption,
  AttributeType,
  AttributeValue,
  AuditEntry,
  IdpMapping,
  Page,
  Role,
  Team,
  User,
} from '@/types/api'
import { api } from './client'

// Users (RF-66 to RF-87) and directory (RF-109 to RF-116).

export const listUsers = (query: Record<string, string | number | undefined>) =>
  api<Page<User>>('/api/users', { query })

export const getUser = (id: string) => api<User>(`/api/users/${id}`)

export interface UserInput {
  name: string
  email: string
  password?: string
  role?: Role | ''
  team_id?: string
  attributes?: Record<string, AttributeValue>
}

export const createUser = (input: UserInput) => api<User>('/api/users', { method: 'POST', body: input })

export const updateUser = (id: string, input: Partial<Pick<UserInput, 'name' | 'email' | 'attributes'>>) =>
  api<User>(`/api/users/${id}`, { method: 'PATCH', body: input })

export type StatusAction = 'deactivate' | 'reactivate' | 'offboard' | 'restore'

export const changeStatus = (id: string, action: StatusAction) =>
  api<User>(`/api/users/${id}/${action}`, { method: 'POST' })

export const setPassword = (id: string, password: string) =>
  api<void>(`/api/users/${id}/password`, { method: 'PUT', body: { password } })

export const addRole = (id: string, role: Role) =>
  api<User>(`/api/users/${id}/roles`, { method: 'POST', body: { role } })

export const removeRole = (id: string, role: Role) =>
  api<User>(`/api/users/${id}/roles/${role}`, { method: 'DELETE' })

export const addTeam = (id: string, teamId: string) =>
  api<User>(`/api/users/${id}/teams`, { method: 'POST', body: { team_id: teamId } })

export const removeTeam = (id: string, teamId: string) =>
  api<User>(`/api/users/${id}/teams/${teamId}`, { method: 'DELETE' })

// Teams (RF-88 to RF-93).

export const listTeams = () => api<{ items: Team[] }>('/api/teams').then((r) => r.items)

export interface TeamInput {
  name: string
  parent_id: string | null
  external_ref: string | null
}

export const createTeam = (input: TeamInput) => api<Team>('/api/teams', { method: 'POST', body: input })

export const updateTeam = (id: string, input: TeamInput) =>
  api<Team>(`/api/teams/${id}`, { method: 'PATCH', body: input })

export const setTeamArchived = (id: string, archived: boolean) =>
  api<Team>(`/api/teams/${id}/${archived ? 'archive' : 'unarchive'}`, { method: 'POST' })

export const addLeader = (teamId: string, userId: string) =>
  api<void>(`/api/teams/${teamId}/leaders`, { method: 'POST', body: { user_id: userId } })

export const removeLeader = (teamId: string, userId: string) =>
  api<void>(`/api/teams/${teamId}/leaders/${userId}`, { method: 'DELETE' })

// Dynamic attributes (RF-94 to RF-107).

export const listAttributes = () => api<{ items: Attribute[] }>('/api/attributes').then((r) => r.items)

export interface AttributeInput {
  name: string
  type: AttributeType
  required: boolean
  options: AttributeOption[]
}

export const createAttribute = (input: AttributeInput) =>
  api<Attribute>('/api/attributes', { method: 'POST', body: input })

export const updateAttribute = (id: string, input: AttributeInput) =>
  api<Attribute>(`/api/attributes/${id}`, { method: 'PATCH', body: input })

export const setAttributeRetired = (id: string, retired: boolean) =>
  api<Attribute>(`/api/attributes/${id}/${retired ? 'retire' : 'restore'}`, { method: 'POST' })

// IdP mappings (RF-39) and audit (RF-120).

export const listIdpMappings = () =>
  api<{ items: IdpMapping[]; default_issuer: string }>('/api/idp-mappings')

export type IdpMappingInput = Omit<IdpMapping, 'id'>

export const createIdpMapping = (input: IdpMappingInput) =>
  api<IdpMapping>('/api/idp-mappings', { method: 'POST', body: input })

export const deleteIdpMapping = (id: string) => api<void>(`/api/idp-mappings/${id}`, { method: 'DELETE' })

export const listAudit = (query: Record<string, string | number | undefined>) =>
  api<Page<AuditEntry>>('/api/audit', { query })
