import { useQuery } from '@tanstack/vue-query'
import { computed } from 'vue'
import { listTeams, listUsers } from '@/api/admin'
import { flatten, ledScope } from '@/features/teams/tree'
import { useAuthStore } from '@/stores/auth'

/** Active people that can own or take work, and the teams for the forms. */
export function usePeople() {
  const auth = useAuthStore()
  const { data: users } = useQuery({
    queryKey: ['users', 'active-workers'],
    queryFn: () => listUsers({ status: 'active', page_size: 100 }),
    staleTime: 60_000,
  })
  const { data: teams } = useQuery({ queryKey: ['teams'], queryFn: listTeams, staleTime: 60_000 })

  const activeTeams = computed(() => flatten((teams.value ?? []).filter((t) => !t.archived_at)))
  // Teams where the user may create projects or team objectives: all for
  // admins, the led ones and their subteams for leads.
  const ownTeams = computed(() => {
    if (auth.can('projects:create:all')) return activeTeams.value
    const scope = ledScope(teams.value ?? [], auth.me?.id ?? '')
    return activeTeams.value.filter(({ team }) => scope.has(team.id))
  })
  return { users: computed(() => users.value?.items ?? []), teams: activeTeams, ownTeams }
}
