import { createApp } from 'vue'
import { ref, provide } from 'vue'
import App from './App.vue'

const app = createApp(App)

const lang = ref(localStorage.getItem('lang') || 'zh')
provide('lang', lang)

app.mount('#app')
