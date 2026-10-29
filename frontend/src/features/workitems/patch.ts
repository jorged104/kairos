import { getWorkItem, updateWorkItem, type WorkItemUpdateInput } from '@/api/work'
import type { WorkItem } from '@/types/work'

/**
 * Every editable field of a work item, as the PATCH that replaces them
 * expects. The description is left out: the page saves it (spec 005), and
 * the API keeps it when the PATCH doesn't bring it.
 */
export function toUpdateInput(w: WorkItem): WorkItemUpdateInput {
  return {
    version: w.version, type_id: w.type_id, status_id: w.status_id, priority_id: w.priority_id, title: w.title,
    parent_id: w.parent?.id ?? null, assignee_id: w.assignee?.id ?? null,
    start_date: w.start_date, due_date: w.due_date, estimated_hours: w.estimated_hours,
    remaining_hours: w.remaining_hours, done_ratio: w.done_ratio, position: w.position, weight: w.weight,
    custom_values: { ...w.custom_values },
  }
}

/**
 * Changes some fields of a work item from a table cell: the rest are read
 * fresh, so the edit never overwrites fields the row doesn't show.
 */
export async function patchWorkItem(id: string, patch: Partial<WorkItemUpdateInput>): Promise<WorkItem> {
  const current = await getWorkItem(id)
  return updateWorkItem(id, { ...toUpdateInput(current), ...patch })
}
