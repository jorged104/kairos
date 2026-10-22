import { VueQueryPlugin } from '@tanstack/vue-query'
import { createPinia } from 'pinia'
import { createApp } from 'vue'
import App from '@/App.vue'
import { createAppI18n } from '@/lib/i18n'
import { router } from '@/router'
import './style.css'

createApp(App)
  .use(router)
  .use(createPinia())
  .use(VueQueryPlugin)
  .use(createAppI18n())
  .mount('#app')
