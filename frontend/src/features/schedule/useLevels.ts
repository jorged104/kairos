import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { getScheduleSettings, type Level, type PaletteColor, type ScheduleSettings } from '@/api/schedule'

/** Classes of each palette color, written whole so Tailwind generates them (RF-40). */
export const COLOR_CLASSES: Record<PaletteColor, { badge: string; solid: string; soft: string }> = {
  blue: { badge: 'bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300', solid: 'bg-blue-600 dark:bg-blue-500', soft: 'bg-blue-200 dark:bg-blue-950' },
  green: { badge: 'bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-300', solid: 'bg-green-600 dark:bg-green-500', soft: 'bg-green-200 dark:bg-green-950' },
  amber: { badge: 'bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300', solid: 'bg-amber-500 dark:bg-amber-400', soft: 'bg-amber-200 dark:bg-amber-950' },
  red: { badge: 'bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-300', solid: 'bg-red-600 dark:bg-red-500', soft: 'bg-red-200 dark:bg-red-950' },
  orange: { badge: 'bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-300', solid: 'bg-orange-600 dark:bg-orange-500', soft: 'bg-orange-200 dark:bg-orange-950' },
  violet: { badge: 'bg-violet-100 text-violet-800 dark:bg-violet-950 dark:text-violet-300', solid: 'bg-violet-600 dark:bg-violet-500', soft: 'bg-violet-200 dark:bg-violet-950' },
  teal: { badge: 'bg-teal-100 text-teal-800 dark:bg-teal-950 dark:text-teal-300', solid: 'bg-teal-600 dark:bg-teal-500', soft: 'bg-teal-200 dark:bg-teal-950' },
  slate: { badge: 'bg-slate-200 text-slate-800 dark:bg-slate-800 dark:text-slate-200', solid: 'bg-slate-600 dark:bg-slate-400', soft: 'bg-slate-200 dark:bg-slate-800' },
}

const defaults: ScheduleSettings = {
  upper: 5,
  risk: -5,
  late: -15,
  version: 0,
  levels: { ahead: { name: null, color: 'blue' }, on_track: { name: null, color: 'green' }, at_risk: { name: null, color: 'amber' }, late: { name: null, color: 'red' } },
}

/** The traffic light as configured: name and classes of each level (spec 009). */
export function useLevels() {
  const { t } = useI18n()
  const { data } = useQuery({ queryKey: ['schedule-settings'], queryFn: getScheduleSettings, staleTime: 30_000 })
  const settings = computed(() => data.value ?? defaults)
  const style = (l: Level) => settings.value.levels?.[l] ?? defaults.levels[l] ?? { name: null, color: 'slate' as const }
  return {
    settings,
    name: (l: Level) => style(l).name || t(`schedule.levels.${l}`),
    classes: (l: Level) => COLOR_CLASSES[style(l).color] ?? COLOR_CLASSES.slate,
  }
}

/** A gap in points with its sign and one decimal. */
export const signed = (n: number) => `${n > 0 ? '+' : ''}${n.toLocaleString('es', { maximumFractionDigits: 1 })}`
