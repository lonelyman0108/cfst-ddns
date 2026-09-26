import { createApp } from 'vue'
import { createPinia } from 'pinia'
import 'vue-sonner/style.css'
import './styles/main.css'
import App from './App.vue'
import router from './router'
import { i18n } from './i18n'
import { useThemeStore } from './stores/theme'

const app = createApp(App)
app.use(createPinia())
app.use(i18n)
useThemeStore()
app.use(router)
app.mount('#app')
