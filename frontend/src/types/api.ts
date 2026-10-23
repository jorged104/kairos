// Types shared with the API (spec 003). JSON is snake_case, as the API sends it.

export type Role = 'admin' | 'lead' | 'member' | 'viewer'
export type Source = 'manual' | 'idp'
export type UserStatus = 'active' | 'inactive' | 'offboarded'
export type AttributeType = 'text' | 'number' | 'date' | 'boolean' | 'select'
export type AttributeValue = string | number | boolean | null

export const ROLES: Role[] = ['admin', 'lead', 'member', 'viewer']

export type Permission =
  | 'directory:read'
  | 'users:read_full:all'
  | 'users:read_full:team'
  | 'users:create:all'
  | 'users:create:team'
  | 'users:update:all'
  | 'users:update:team'
  | 'users:status:all'
  | 'users:status:team'
  | 'users:offboard'
  | 'roles:assign:all'
  | 'roles:assign:basic'
  | 'teams:members:all'
  | 'teams:members:team'
  | 'teams:manage'
  | 'attributes:manage'
  | 'idp_mappings:manage'
  | 'passwords:set'
  | 'audit:read'
  | 'self:password'
  // Spec 004.
  | 'work:read'
  | 'templates:use'
  | 'templates:manage'
  | 'catalog:manage'
  | 'projects:create:all'
  | 'projects:create:team'
  | 'projects:manage:all'
  | 'projects:manage:team'
  | 'projects:manage:owner'
  | 'workitems:write:all'
  | 'workitems:write:team'
  | 'workitems:write:participant'
  | 'workitems:delete:own'
  | 'objectives:create:all'
  | 'objectives:create:team'
  | 'objectives:manage:all'
  | 'objectives:manage:team'
  | 'objectives:manage:owner'

export interface RoleAssignment {
  role: Role
  source: Source
}

export interface TeamAssignment {
  id: string
  name: string
  source: Source
}

export interface AuthConfig {
  local_enabled: boolean
  oidc_enabled: boolean
  registration_enabled: boolean
}

export interface Me {
  id: string
  name: string
  email: string
  status: UserStatus
  auth_method: 'local' | 'oidc' | ''
  has_password: boolean
  roles: RoleAssignment[]
  teams: TeamAssignment[]
  permissions: Permission[]
}

export interface Abilities {
  edit: boolean
  set_status: boolean
  offboard: boolean
  assign_roles: boolean
  manage_teams: boolean
  set_password: boolean
}

export interface User {
  id: string
  name: string
  status: UserStatus
  teams: TeamAssignment[]
  full_profile: boolean
  email?: string
  roles?: RoleAssignment[]
  attributes?: Record<string, AttributeValue>
  retired_attributes?: Record<string, AttributeValue>
  has_password?: boolean
  can?: Abilities
}

export interface Page<T> {
  items: T[]
  total: number
  page: number
  page_size: number
}

export interface Team {
  id: string
  name: string
  parent_id: string | null
  external_ref: string | null
  archived_at: string | null
  leaders: { user_id: string; name: string }[]
  member_count: number
}

export interface AttributeOption {
  value: string
  retired: boolean
}

export interface Attribute {
  id: string
  name: string
  type: AttributeType
  required: boolean
  options: AttributeOption[]
  retired_at: string | null
}

export interface IdpMapping {
  id: string
  issuer: string
  claim: 'roles' | 'groups'
  external_value: string
  target_type: 'role' | 'team'
  target: string
}

export interface AuditEntry {
  id: number
  occurred_at: string
  actor_id: string | null
  actor_kind: 'user' | 'idp' | 'system'
  actor_name: string | null
  action: string
  target_type: string
  target_id: string
  target_name: string | null
  before: unknown
  after: unknown
}
