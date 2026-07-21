import {createApp, ref} from 'vue'
import { provide } from 'vue'
import App from './App.vue'

const app = createApp(App)

const lang = ref(localStorage.getItem('lang') || 'zh')
provide('lang', lang)

app.mount('#app')
