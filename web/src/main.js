import { createApp } from 'vue'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'
import 'element-plus/dist/index.css'
import App from '@/App.vue'
import JobLogBox from '@/components/JobLogBox.vue'
import { createAppRouter } from '@/router/index.js'
import '@/assets/styles/index.css'

const app = createApp(App)
app.use(createAppRouter())
app.use(ElementPlus, { locale: zhCn, size: 'default' })
for (const [name, icon] of Object.entries(ElementPlusIconsVue)) {
  app.component(name, icon)
}
// 向导各步、包中心、升级页都要放任务日志框，全局注册省去逐个 import。
app.component('JobLogBox', JobLogBox)
app.mount('#app')
