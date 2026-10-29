// Types of the spec 004 API: catalog, projects, milestones, work items and objectives.

import type { JSONContent } from '@tiptap/vue-3'
import type { AttributeOption, AttributeType, AttributeValue, Role } from './api'

/** A TipTap/ProseMirror JSON document, as stored by the API. */
export type RichDoc = JSONContent | null

/** How a mention of a page reads now (spec 005, RF-27 to RF-29). */
export interface MentionRef {
  label?: string
  state: 'ok' | 'no_access' | 'deleted'
}
export type MentionRefs = Record<string, MentionRef>

export interface Ref {
  id: string
  name: string
}

export interface PhaseDefinition {
  id: string
  methodology_id: string
  name: string
  position: number
  has_start_gate: boolean
  has_finish_gate: boolean
  retired: boolean
}

export interface Methodology {
  id: string
  name: string
  position: number
  retired: boolean
  phases: PhaseDefinition[]
}

export interface WorkItemType {
  id: string
  name: string
  color: string
  position: number
  /** Copied to new work items of the type (RF-150, RF-151). */
  default_weight: number
  retired: boolean
}

export interface Status {
  id: string
  name: string
  color: string
  position: number
  is_closed: boolean
  is_default: boolean
  retired: boolean
}

export interface Priority {
  id: string
  name: string
  color: string
  position: number
  is_default: boolean
  retired: boolean
}

export interface Catalog {
  methodologies: Methodology[]
  work_item_types: WorkItemType[]
  statuses: Status[]
  priorities: Priority[]
}

export interface Transition {
  from_status_id: string
  to_status_id: string
  role: Role | null
}

export type CustomFieldEntity = 'project' | 'work_item'

export interface CustomField {
  id: string
  entity: CustomFieldEntity
  name: string
  type: AttributeType
  required: boolean
  options: AttributeOption[]
  work_item_type_ids: string[]
  retired_at: string | null
}

export type CustomValues = Record<string, AttributeValue>

export type Lifecycle = 'activo' | 'cerrado' | 'archivado'
export type Health = 'en_curso' | 'en_riesgo' | 'fuera_de_curso'

export const LIFECYCLES: Lifecycle[] = ['activo', 'cerrado', 'archivado']
export const HEALTHS: Health[] = ['en_curso', 'en_riesgo', 'fuera_de_curso']

export interface Project {
  id: string
  identifier: string
  name: string
  description: RichDoc
  /** Compared by the autosave of the page (spec 005). */
  page_version: number
  /** The page's mentions, on single reads. */
  refs?: MentionRefs
  team: Ref
  owner: Ref
  methodology: Ref | null
  parent: { id: string; identifier: string; name: string } | null
  template_id: string | null
  is_template: boolean
  lifecycle: Lifecycle
  health: Health
  health_explanation: string
  start_date: string | null
  end_date: string | null
  depth: number
  version: number
  work_item_type_ids: string[]
  custom_field_ids: string[]
  custom_values: CustomValues
  can: { manage: boolean; write_work_items: boolean; manage_template: boolean }
}

export interface ProjectSummary {
  id: string
  identifier: string
  name: string
  team: Ref
  owner: Ref
  methodology: Ref | null
  parent_id: string | null
  lifecycle: Lifecycle
  health: Health
  start_date: string | null
  end_date: string | null
  depth: number
  can: Project['can']
}

export interface Template {
  id: string
  identifier: string
  name: string
  team: Ref
  methodology: Ref | null
}

export interface ProjectPhase {
  id: string
  definition_id: string
  name: string
  position: number
  start_date: string | null
  finish_date: string | null
  has_start_gate: boolean
  has_finish_gate: boolean
  start_gate_passed_at: string | null
  start_gate_passed_by: string | null
  finish_gate_passed_at: string | null
  finish_gate_passed_by: string | null
}

export type MilestoneStatus = 'abierto' | 'cumplido' | 'cancelado'

export interface Milestone {
  id: string
  project: { id: string; identifier: string; name: string }
  phase: Ref | null
  name: string
  description: RichDoc
  /** Compared by the autosave of the page (spec 005). */
  page_version: number
  /** The page's mentions, on single reads. */
  refs?: MentionRefs
  due_date: string | null
  completed_on: string | null
  status: MilestoneStatus
  overdue: boolean
  position: number
  version: number
  progress: {
    total: number
    closed: number
    open: number
    percent: number
    estimated_hours: number
    remaining_hours: number
    weight: number
    weight_closed: number
  }
  can: { manage: boolean }
}

/** A row of the work item tree (RF-130). */
export interface WorkNode {
  id: string
  parent_id: string | null
  key: string
  title: string
  type_id: string
  status_id: string
  priority_id: string
  assignee: Ref | null
  derived_done_ratio: number
  derived_estimated_hours: number
  derived_remaining_hours: number
  depth: number
  position: number
  due_date: string | null
  weight: number
}

export interface MilestoneItem extends WorkNode {
  inherited: boolean
}

export interface EffectiveMilestone {
  id: string
  name: string
  status: MilestoneStatus
  due_date: string | null
  inherited: boolean
}

export type DependencyType = 'blocks' | 'precedes' | 'relates' | 'duplicates'
export const DEPENDENCY_TYPES: DependencyType[] = ['blocks', 'precedes', 'relates', 'duplicates']

export interface WorkRef {
  id: string
  key: string
  title: string
}

export interface Dependency {
  id: string
  type: DependencyType
  direction: 'outgoing' | 'incoming'
  lag_days: number
  other: WorkRef
  dates_violated: boolean
}

export interface WorkItem {
  id: string
  key: string
  sequence_number: number
  project: { id: string; identifier: string; name: string }
  type_id: string
  status_id: string
  priority_id: string
  title: string
  description: RichDoc
  /** Compared by the autosave of the page (spec 005). */
  page_version: number
  /** The page's mentions, on single reads. */
  refs?: MentionRefs
  author: Ref
  assignee: Ref | null
  parent: WorkRef | null
  start_date: string | null
  due_date: string | null
  estimated_hours: number | null
  remaining_hours: number | null
  done_ratio: number
  /** 0 to 100; 0 is a tracking work item that moves no progress (RF-149). */
  weight: number
  derived: { estimated_hours: number; remaining_hours: number; done_ratio: number }
  position: number
  depth: number
  child_count: number
  version: number
  closed_at: string | null
  custom_values: CustomValues
  allowed_status_ids: string[]
  dependencies: Dependency[]
  can: { write: boolean; delete: boolean }
}

export type ObjectiveScope = 'organizacion' | 'equipo' | 'proyecto'
export type ObjectiveStatus = 'borrador' | 'activo' | 'cerrado' | 'cancelado'
export const OBJECTIVE_SCOPES: ObjectiveScope[] = ['organizacion', 'equipo', 'proyecto']
export const OBJECTIVE_STATUSES: ObjectiveStatus[] = ['borrador', 'activo', 'cerrado', 'cancelado']

export interface KeyResult {
  id: string
  title: string
  unit: string
  start_value: number
  target_value: number
  current_value: number
  calc_mode: 'manual' | 'derivado'
  position: number
  version: number
  progress: number
  linked_total: number
  linked_done: number
  links: { kind: 'work_item' | 'milestone'; id: string; label: string; name: string }[]
}

export interface Objective {
  id: string
  title: string
  description: RichDoc
  owner: Ref
  scope: ObjectiveScope
  scope_team: Ref | null
  scope_project: Ref | null
  period_start: string
  period_end: string
  status: ObjectiveStatus
  version: number
  progress: number
  key_results: KeyResult[]
  projects: { id: string; identifier: string; name: string; lifecycle: Lifecycle }[]
  milestones: {
    id: string
    name: string
    status: MilestoneStatus
    due_date: string | null
    completed_on: string | null
    project: { id: string; identifier: string; name: string }
  }[]
  can: { manage: boolean; status: boolean }
}

export interface ObjectiveSummary {
  id: string
  title: string
  owner: Ref
  scope: ObjectiveScope
  period_start: string
  period_end: string
  status: ObjectiveStatus
  progress: number
}
