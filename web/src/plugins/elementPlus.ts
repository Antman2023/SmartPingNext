import type { App } from 'vue'
import {
  ElButton,
  ElConfigProvider,
  ElDropdown,
  ElIcon,
  ElLoading,
  ElMenu,
  ElRadio
} from 'element-plus'

export function installElementPlus(app: App): void {
  app.use(ElButton)
  app.use(ElConfigProvider)
  app.use(ElDropdown)
  app.use(ElIcon)
  app.use(ElLoading)
  app.use(ElMenu)
  app.use(ElRadio)
}
