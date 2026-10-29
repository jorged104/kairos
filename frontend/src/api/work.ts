import type { AttributeOption, AttributeType, Page } from '@/types/api'
import type {
  Catalog,
  CustomField,
  CustomFieldEntity,
  CustomValues,
  DependencyType,
  Dependency,
  EffectiveMilestone,
  Health,
  Milestone,
  MilestoneItem,
  Objective,
  ObjectiveScope,
  ObjectiveSummary,
  Project,
  ProjectPhase,
  ProjectSummary,
  RichDoc,
  Template,
  Transition,
  WorkItem,
  WorkNode,
  WorkRef,
} from '@/types/work'
import { api } from './client'

type Query = Record<string, string | number | undefined>
const items = <T>(r: { items: T[] }) => r.items

// Catalog (RF-1 to RF-17).

export const getCatalog = () => api<Catalog>('/api/catalog')

export type CatalogKind = 'methodologies' | 'work-item-types' | 'statuses' | 'priorities'

export interface CatalogInput {
  name: string
  position: number
  color?: string
  is_closed?: boolean
  is_default?: boolean
  default_weight?: number
}

export const createCatalogItem = (kind: CatalogKind, input: CatalogInput) =>
  api<unknown>(`/api/${kind}`, { method: 'POST', body: input })

export const updateCatalogItem = (kind: CatalogKind, id: string, input: CatalogInput) =>
  api<unknown>(`/api/${kind}/${id}`, { method: 'PATCH', body: input })

export const setCatalogRetired = (kind: CatalogKind | 'phase-definitions', id: string, retired: boolean) =>
  api<unknown>(`/api/${kind}/${id}/${retired ? 'retire' : 'restore'}`, { method: 'POST' })

export interface PhaseInput {
  name: string
  position: number
  has_start_gate: boolean
  has_finish_gate: boolean
}

export const createPhaseDefinition = (methodologyId: string, input: PhaseInput) =>
  api<unknown>(`/api/methodologies/${methodologyId}/phases`, { method: 'POST', body: input })

export const updatePhaseDefinition = (id: string, input: PhaseInput) =>
  api<unknown>(`/api/phase-definitions/${id}`, { method: 'PATCH', body: input })

export const getTransitions = (typeId: string) =>
  api<{ transitions: Transition[] }>(`/api/work-item-types/${typeId}/transitions`).then((r) => r.transitions)

export const setTransitions = (typeId: string, transitions: Transition[]) =>
  api<{ transitions: Transition[] }>(`/api/work-item-types/${typeId}/transitions`, { method: 'PUT', body: { transitions } })

export const listCustomFields = () => api<{ items: CustomField[] }>('/api/custom-fields').then(items)

export interface CustomFieldInput {
  entity?: CustomFieldEntity
  name: string
  type: AttributeType
  required: boolean
  options: AttributeOption[]
  work_item_type_ids: string[]
}

export const createCustomField = (input: CustomFieldInput) =>
  api<CustomField>('/api/custom-fields', { method: 'POST', body: input })

export const updateCustomField = (id: string, input: CustomFieldInput) =>
  api<CustomField>(`/api/custom-fields/${id}`, { method: 'PATCH', body: input })

export const setCustomFieldRetired = (id: string, retired: boolean) =>
  api<CustomField>(`/api/custom-fields/${id}/${retired ? 'retire' : 'restore'}`, { method: 'POST' })

// Projects (RF-18 to RF-43, RF-128).

export const listProjects = (query: Query) => api<Page<ProjectSummary>>('/api/projects', { query })
export const getProject = (id: string) => api<Project>(`/api/projects/${id}`)

export interface ProjectCreateInput {
  identifier: string
  name: string
  team_id: string
  owner_id?: string
  methodology_id?: string
  parent_id?: string
  start_date?: string | null
  end_date?: string | null
  description?: RichDoc
}

export const createProject = (input: ProjectCreateInput) =>
  api<Project>('/api/projects', { method: 'POST', body: input })

export interface ProjectUpdateInput {
  version: number
  name: string
  /** Left out, the API keeps it: the page saves it (spec 005). */
  description?: RichDoc
  team_id: string
  owner_id: string
  parent_id: string | null
  start_date: string | null
  end_date: string | null
  health: Health
  health_explanation: string
  custom_values?: CustomValues
}

export const updateProject = (id: string, input: ProjectUpdateInput) =>
  api<Project>(`/api/projects/${id}`, { method: 'PATCH', body: input })

export type LifecycleAction = 'close' | 'reopen' | 'archive' | 'unarchive'

export const changeLifecycle = (id: string, action: LifecycleAction, version: number) =>
  api<Project>(`/api/projects/${id}/${action}`, { method: 'POST', body: { version } })

export const setProjectTypes = (id: string, ids: string[]) =>
  api<Project>(`/api/projects/${id}/types`, { method: 'PUT', body: { ids } })

export const setProjectFields = (id: string, ids: string[]) =>
  api<Project>(`/api/projects/${id}/custom-fields`, { method: 'PUT', body: { ids } })

export const listPhases = (projectId: string) =>
  api<{ items: ProjectPhase[] }>(`/api/projects/${projectId}/phases`).then(items)

export const updatePhase = (id: string, start: string | null, finish: string | null) =>
  api<ProjectPhase>(`/api/phases/${id}`, { method: 'PATCH', body: { start_date: start, finish_date: finish } })

export const setGate = (id: string, gate: 'start' | 'finish', passed: boolean) =>
  api<ProjectPhase>(`/api/phases/${id}/gates/${gate}`, { method: 'POST', body: { passed } })

export const listTemplates = () => api<{ items: Template[] }>('/api/templates').then(items)

export interface FromTemplateInput {
  template_id: string
  identifier: string
  name: string
  team_id: string
  owner_id?: string
  start_date?: string | null
  end_date?: string | null
  include_content: boolean
}

export const createFromTemplate = (input: FromTemplateInput) =>
  api<Project>('/api/projects/from-template', { method: 'POST', body: input })

export const setTemplate = (id: string, template: boolean) =>
  api<Project>(`/api/projects/${id}/template`, { method: template ? 'POST' : 'DELETE' })

// Milestones (RF-44 to RF-60).

export const listMilestones = (projectId: string) =>
  api<{ items: Milestone[] }>(`/api/projects/${projectId}/milestones`).then(items)

export const getMilestone = (id: string) => api<Milestone>(`/api/milestones/${id}`)

export interface MilestoneInput {
  version?: number
  name: string
  description?: RichDoc
  due_date: string | null
  phase_id: string | null
  position?: number
}

export const createMilestone = (projectId: string, input: MilestoneInput) =>
  api<Milestone>(`/api/projects/${projectId}/milestones`, { method: 'POST', body: input })

export const updateMilestone = (id: string, input: MilestoneInput) =>
  api<Milestone>(`/api/milestones/${id}`, { method: 'PATCH', body: input })

export const deleteMilestone = (id: string) => api<void>(`/api/milestones/${id}`, { method: 'DELETE' })

export type MilestoneAction = 'complete' | 'reopen' | 'cancel'

export const changeMilestoneStatus = (id: string, action: MilestoneAction, version: number, completedOn?: string) =>
  api<Milestone>(`/api/milestones/${id}/${action}`, { method: 'POST', body: { version, completed_on: completedOn || undefined } })

export const milestoneWorkItems = (id: string) =>
  api<{ items: MilestoneItem[] }>(`/api/milestones/${id}/work-items`).then(items)

export const linkMilestoneWorkItem = (id: string, workItemId: string) =>
  api<void>(`/api/milestones/${id}/work-items`, { method: 'POST', body: { work_item_id: workItemId } })

export const unlinkMilestoneWorkItem = (id: string, workItemId: string) =>
  api<void>(`/api/milestones/${id}/work-items/${workItemId}`, { method: 'DELETE' })

// Work items (RF-61 to RF-96, RF-129, RF-130, RF-133).

export const projectTree = (projectId: string) =>
  api<{ items: WorkNode[] }>(`/api/projects/${projectId}/tree`).then(items)

export const listWorkItems = (projectId: string, query: Query) =>
  api<Page<WorkNode>>(`/api/projects/${projectId}/work-items`, { query })

export const getWorkItem = (id: string) => api<WorkItem>(`/api/work-items/${id}`)
export const getWorkItemByKey = (key: string) => api<WorkItem>(`/api/work-items/by-key/${encodeURIComponent(key)}`)

export interface WorkItemCreateInput {
  type_id: string
  title: string
  parent_id?: string | null
  assignee_id?: string | null
  description?: RichDoc
}

export const createWorkItem = (projectId: string, input: WorkItemCreateInput) =>
  api<WorkItem>(`/api/projects/${projectId}/work-items`, { method: 'POST', body: input })

export interface WorkItemUpdateInput {
  version: number
  type_id: string
  status_id: string
  priority_id: string
  title: string
  /** Left out, the API keeps it: the page saves it (spec 005). */
  description?: RichDoc
  parent_id: string | null
  assignee_id: string | null
  start_date: string | null
  due_date: string | null
  estimated_hours: number | null
  remaining_hours: number | null
  done_ratio: number
  position: number
  weight: number
  custom_values?: CustomValues
}

export const updateWorkItem = (id: string, input: WorkItemUpdateInput) =>
  api<WorkItem>(`/api/work-items/${id}`, { method: 'PATCH', body: input })

export const deleteWorkItem = (id: string) => api<void>(`/api/work-items/${id}`, { method: 'DELETE' })

export const workItemAncestors = (id: string) =>
  api<{ items: WorkRef[] }>(`/api/work-items/${id}/ancestors`).then(items)

export const workItemDescendants = (id: string) =>
  api<{ items: WorkNode[] }>(`/api/work-items/${id}/descendants`).then(items)

export const workItemMilestones = (id: string) =>
  api<{ items: EffectiveMilestone[] }>(`/api/work-items/${id}/milestones`).then(items)

export const addDependency = (id: string, toId: string, type: DependencyType, lagDays: number) =>
  api<{ items: Dependency[] }>(`/api/work-items/${id}/dependencies`, {
    method: 'POST',
    body: { to_id: toId, type, lag_days: lagDays },
  })

export const removeDependency = (id: string) => api<void>(`/api/dependencies/${id}`, { method: 'DELETE' })

// Objectives (RF-97 to RF-115).

export const listObjectives = (query: Query) => api<Page<ObjectiveSummary>>('/api/objectives', { query })
export const getObjective = (id: string) => api<Objective>(`/api/objectives/${id}`)

export interface ObjectiveCreateInput {
  title: string
  scope: ObjectiveScope
  scope_team_id?: string
  scope_project_id?: string
  owner_id?: string
  period_start: string
  period_end: string
  description?: RichDoc
}

export const createObjective = (input: ObjectiveCreateInput) =>
  api<Objective>('/api/objectives', { method: 'POST', body: input })

export interface ObjectiveUpdateInput {
  version: number
  title: string
  description: RichDoc
  owner_id: string
  period_start: string
  period_end: string
}

export const updateObjective = (id: string, input: ObjectiveUpdateInput) =>
  api<Objective>(`/api/objectives/${id}`, { method: 'PATCH', body: input })

export type ObjectiveAction = 'activate' | 'close' | 'cancel' | 'reopen'

export const changeObjectiveStatus = (id: string, action: ObjectiveAction, version: number) =>
  api<Objective>(`/api/objectives/${id}/${action}`, { method: 'POST', body: { version } })

export interface KeyResultInput {
  version?: number
  title: string
  unit: string
  start_value: number
  target_value: number
  current_value?: number
  calc_mode: 'manual' | 'derivado'
  position?: number
}

export const createKeyResult = (objectiveId: string, input: KeyResultInput) =>
  api<Objective>(`/api/objectives/${objectiveId}/key-results`, { method: 'POST', body: input })

export const updateKeyResult = (objectiveId: string, id: string, input: KeyResultInput) =>
  api<Objective>(`/api/objectives/${objectiveId}/key-results/${id}`, { method: 'PATCH', body: input })

export const deleteKeyResult = (objectiveId: string, id: string) =>
  api<Objective>(`/api/objectives/${objectiveId}/key-results/${id}`, { method: 'DELETE' })

export type ObjectiveLink = 'projects' | 'milestones'

export const linkObjective = (objectiveId: string, kind: ObjectiveLink, id: string) =>
  api<Objective>(`/api/objectives/${objectiveId}/${kind}`, { method: 'POST', body: { id } })

export const unlinkObjective = (objectiveId: string, kind: ObjectiveLink, id: string) =>
  api<Objective>(`/api/objectives/${objectiveId}/${kind}/${id}`, { method: 'DELETE' })

export type KeyResultLink = 'work-items' | 'milestones'

export const linkKeyResult = (objectiveId: string, krId: string, kind: KeyResultLink, id: string) =>
  api<Objective>(`/api/objectives/${objectiveId}/key-results/${krId}/${kind}`, { method: 'POST', body: { id } })

export const unlinkKeyResult = (objectiveId: string, krId: string, kind: KeyResultLink, id: string) =>
  api<Objective>(`/api/objectives/${objectiveId}/key-results/${krId}/${kind}/${id}`, { method: 'DELETE' })
