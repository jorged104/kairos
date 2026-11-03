import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { listMilestones } from '@/api/work'
import { useCatalog } from '@/features/work/useCatalog'
import { usePeople } from '@/features/work/usePeople'
import type { Project } from '@/types/work'
import type { FilterField } from './useProjectFilters'

export interface FilterOption {
  value: string
  label: string
  color?: string
}

/**
 * What each filter can take and the name of each value. A value that no
 * longer exists or is retired is «no disponible» (spec 008, RF-31).
 */
export function useFilterOptions(project: () => Project | undefined) {
  const { t } = useI18n()
  const { catalog } = useCatalog()
  const { users } = usePeople()
  const { data: milestones } = useQuery({
    queryKey: computed(() => ['milestones', project()?.id ?? '']),
    queryFn: () => listMilestones(project()!.id),
    enabled: computed(() => !!project()),
  })

  const active = <T extends { id: string; name: string; retired?: boolean; color?: string | null }>(xs: T[]) =>
    xs.filter((x) => !x.retired).map((x) => ({ value: x.id, label: x.name, color: x.color ?? undefined }))

  const options = computed<Partial<Record<FilterField, FilterOption[]>>>(() => ({
    status_id: active(catalog.value.statuses),
    type_id: active(catalog.value.work_item_types.filter((ty) => project()?.work_item_type_ids.includes(ty.id))),
    priority_id: active(catalog.value.priorities),
    size_id: [{ value: 'none', label: t('filters.noSize') }, ...active(catalog.value.sizes)],
    label_id: active(catalog.value.labels),
    milestone_id: [
      { value: 'none', label: t('filters.noMilestone') },
      ...(milestones.value ?? []).filter((m) => m.status !== 'cancelado').map((m) => ({ value: m.id, label: m.name })),
    ],
    assignee_id: [
      { value: 'me', label: t('filters.me') },
      { value: 'none', label: t('workItems.unassigned') },
      ...users.value.map((u) => ({ value: u.id, label: u.name })),
    ],
    closed: [
      { value: 'false', label: t('filters.open') },
      { value: 'true', label: t('filters.closed') },
    ],
    blocked: [
      { value: 'true', label: t('filters.blocked') },
      { value: 'false', label: t('filters.notBlocked') },
    ],
    overdue: [{ value: 'true', label: t('filters.overdue') }],
  }))

  // Retired ones too, to name what a view still has.
  const names = computed(() => {
    const m = new Map<string, string>()
    for (const x of [...catalog.value.statuses, ...catalog.value.work_item_types, ...catalog.value.priorities,
      ...catalog.value.sizes, ...catalog.value.labels, ...(milestones.value ?? [])]) m.set(x.id, x.name)
    for (const u of users.value) m.set(u.id, u.name)
    return m
  })

  function label(field: string, value: string): string {
    if (field === 'q') return `«${value}»`
    if (field === 'due_before' || field === 'due_after') return value
    const opt = options.value[field as FilterField]?.find((o) => o.value === value)
    return opt?.label ?? names.value.get(value) ?? t('filters.unavailable')
  }
  function available(field: string, value: string): boolean {
    if (field === 'q' || field === 'due_before' || field === 'due_after') return true
    return !!options.value[field as FilterField]?.some((o) => o.value === value)
  }
  return { options, label, available }
}
