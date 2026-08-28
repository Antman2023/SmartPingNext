<template>
  <el-dropdown trigger="click" popper-class="settings-dropdown">
    <el-button
      circle
      :icon="Setting"
      :aria-label="$t('nav.interfaceSettings')"
      :title="$t('nav.interfaceSettings')"
    />
    <template #dropdown>
      <el-dropdown-menu>
        <el-dropdown-item>
          <div class="settings-item">
            <div class="settings-item__label">
              <el-icon><Brush /></el-icon>
              <span>{{ $t('nav.theme') }}</span>
            </div>
            <el-radio-group v-model="currentTheme" size="small">
              <el-radio-button value="system">
                <el-icon><Monitor /></el-icon>
                <span>{{ $t('theme.systemMode') }}</span>
              </el-radio-button>
              <el-radio-button value="light">
                <el-icon><Sunny /></el-icon>
                <span>{{ $t('theme.lightMode') }}</span>
              </el-radio-button>
              <el-radio-button value="dark">
                <el-icon><Moon /></el-icon>
                <span>{{ $t('theme.darkMode') }}</span>
              </el-radio-button>
            </el-radio-group>
          </div>
        </el-dropdown-item>
        <el-dropdown-item divided>
          <div class="settings-item">
            <div class="settings-item__label">
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
import { Setting, Brush, Monitor, Moon, Sunny, Promotion } from '@element-plus/icons-vue'
import { useThemeStore, type ThemePreference } from '@/stores/theme'
import { useLocaleStore, type LocaleCode } from '@/stores/locale'
import { useLocale } from '@/composables/useLocale'

const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { setLocale } = useLocale()

const currentTheme = computed({
  get: () => themeStore.preference,
  set: (theme: ThemePreference) => {
    themeStore.setTheme(theme)
  }
})

const currentLocale = computed({
  get: () => localeStore.locale,
  set: (locale: LocaleCode) => {
    setLocale(locale)
  }
})

</script>

<style scoped lang="scss">
.el-button {
  background: color-mix(in srgb, var(--color-bg-primary) 80%, transparent);
  border-color: var(--color-border-light);
  color: var(--navbar-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);

  &:hover {
    border-color: color-mix(in srgb, var(--color-primary) 22%, transparent);
  }
}
</style>
