import { useQuery } from '@tanstack/vue-query'
import { fetchReadiness } from '@/api/health'

export const READINESS_REFRESH_MS = 30_000

export function useReadiness() {
  return useQuery({
    queryKey: ['health', 'ready'],
    queryFn: () => fetchReadiness(),
    refetchInterval: READINESS_REFRESH_MS,
    refetchIntervalInBackground: true,
    refetchOnWindowFocus: false,
    retry: false,
  })
}
