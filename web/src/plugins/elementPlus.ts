import type { App } from 'vue'
import { ElButton, ElConfigProvider, ElIcon, ElLoading } from 'element-plus'

export function installElementPlus(app: App): void {
  app.use(ElButton)
  app.use(ElConfigProvider)
  app.use(ElIcon)
  app.use(ElLoading)
}
