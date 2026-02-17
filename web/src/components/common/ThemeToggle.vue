<template>
  <el-dropdown trigger="click" @command="handleCommand" popper-class="settings-dropdown">
    <el-button circle :icon="Setting" />
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item command="theme">
          <div class="dropdown-item">
            <el-icon><component :is="themeStore.theme === 'light' ? Moon : Sunny" /></el-icon>
            <span>{{ themeText }}</span>
          </div>
        </el-dropdown-item>
        <el-dropdown-item divided>
          <div class="lang-item">
            <div class="lang-label">
              <el-icon><Promotion /></el-icon>
              <span>{{ $t('nav.language') }}</span>
            </div>
            <el-radio-group v-model="currentLocale" size="small">
              <el-radio-button value="zh-CN">中文</el-radio-button>
              <el-radio-button value="en-US">EN</el-radio-button>
            </el-radio-group>
          </div>
        </el-dropdown-item>
      </el-dropdown-menu>
    </template>
  </el-dropdown>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Setting, Moon, Sunny, Promotion } from '@element-plus/icons-vue'
import { useThemeStore } from '@/stores/theme'
import { useLocaleStore, type LocaleCode } from '@/stores/locale'
import { useLocale } from '@/composables/useLocale'

const { t } = useI18n({ useScope: 'global' })
const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { setLocale } = useLocale()

const themeText = computed(() => {
  return themeStore.theme === 'light' ? t('theme.darkMode') : t('theme.lightMode')
})

const currentLocale = computed({
  get: () => localeStore.locale,
  set: (locale: LocaleCode) => {
    setLocale(locale)
  }
})

const handleCommand = (command: string) => {
  if (command === 'theme') {
    themeStore.toggleTheme()
  }
}

</script>

<style scoped lang="scss">
.el-button {
  background-color: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
  color: var(--navbar-text);

  &:hover {
    background-color: rgba(255, 255, 255, 0.2);
  }
}
</style>
