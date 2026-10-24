import type { Health, Lifecycle, MilestoneStatus, ObjectiveStatus } from '@/types/work'

type Variant = 'default' | 'secondary' | 'destructive' | 'outline'

/** Badge variants for the states of spec 004. */
export const healthVariant: Record<Health, Variant> = {
  en_curso: 'secondary',
  en_riesgo: 'outline',
  fuera_de_curso: 'destructive',
}

export const lifecycleVariant: Record<Lifecycle, Variant> = {
  activo: 'default',
  cerrado: 'secondary',
  archivado: 'outline',
}

export const milestoneVariant: Record<MilestoneStatus, Variant> = {
  abierto: 'outline',
  cumplido: 'default',
  cancelado: 'secondary',
}

export const objectiveVariant: Record<ObjectiveStatus, Variant> = {
  borrador: 'outline',
  activo: 'default',
  cerrado: 'secondary',
  cancelado: 'secondary',
}
