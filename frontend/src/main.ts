import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from '@/App.vue'
import { setUnauthenticatedHandler } from '@/api/client'
import { createAppI18n } from '@/lib/i18n'
import { router } from '@/router'
import { useAuthStore } from '@/stores/auth'
import './style.css'

const app = createApp(App).use(createPinia())

// A session that expires or is revoked while the app is open goes back to the login (RF-42).
const auth = useAuthStore()
setUnauthenticatedHandler(() => {
  if (!auth.me) return
  auth.set(null)
  void router.push({ name: 'login', query: { redirect: router.currentRoute.value.fullPath } })
})

app.use(router).use(VueQueryPlugin).use(createAppI18n()).mount('#app')
