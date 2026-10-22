import { defineStore } from 'pinia'
import { ref } from 'vue'
import { fetchMe } from '@/api/auth'
import { ApiError } from '@/api/client'
import type { Me, Permission } from '@/types/api'

/** The signed-in user, loaded from /api/me; null when there is no session. */
export const useAuthStore = defineStore('auth', () => {
  const me = ref<Me | null>(null)
  const loaded = ref(false)

  async function load(): Promise<Me | null> {
    try {
      me.value = await fetchMe()
    } catch (e) {
      if (!(e instanceof ApiError) || (e.status !== 401 && e.status !== 403)) throw e
      me.value = null
    }
    loaded.value = true
    return me.value
  }

  function set(value: Me | null) {
    me.value = value
    loaded.value = true
  }

  function can(...perms: Permission[]): boolean {
    return perms.some((p) => me.value?.permissions.includes(p) ?? false)
  }

  return { me, loaded, load, set, can }
})
