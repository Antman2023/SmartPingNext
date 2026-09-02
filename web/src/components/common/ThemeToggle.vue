<template>
  <div ref="rootRef" class="theme-toggle">
    <button
      ref="triggerRef"
      type="button"
      class="theme-toggle__trigger"
      :aria-label="$t('nav.interfaceSettings')"
      :title="$t('nav.interfaceSettings')"
      aria-haspopup="dialog"
      :aria-expanded="isOpen"
      aria-controls="interface-settings"
      @click="toggleSettings"
    >
      <Setting aria-hidden="true" />
    </button>

    <Transition name="settings-popover">
      <div
        v-if="isOpen"
        id="interface-settings"
        ref="panelRef"
        class="theme-toggle__panel"
        role="dialog"
        :aria-label="$t('nav.interfaceSettings')"
        @keydown.esc.stop.prevent="closeAndFocusTrigger"
      >
        <fieldset class="settings-item">
          <legend class="settings-item__label">
            <Brush aria-hidden="true" />
            <span>{{ $t('nav.theme') }}</span>
          </legend>
          <div class="settings-segments">
            <label
              v-for="option in themeOptions"
              :key="option.value"
              class="settings-segment"
              :class="{ 'is-active': currentTheme === option.value }"
            >
              <input
                v-model="currentTheme"
                type="radio"
                name="interface-theme"
                :value="option.value"
              />
              <span class="settings-segment__content">
                <component :is="option.icon" aria-hidden="true" />
                <span>{{ $t(option.label) }}</span>
              </span>
            </label>
          </div>
        </fieldset>

        <fieldset class="settings-item settings-item--divided">
          <legend class="settings-item__label">
            <Promotion aria-hidden="true" />
            <span>{{ $t('nav.language') }}</span>
          </legend>
          <div class="settings-segments">
            <label
              v-for="option in localeOptions"
              :key="option.value"
              class="settings-segment"
              :class="{ 'is-active': currentLocale === option.value }"
            >
              <input
                v-model="currentLocale"
                type="radio"
                name="interface-locale"
                :value="option.value"
              />
              <span class="settings-segment__content">{{ option.label }}</span>
            </label>
          </div>
        </fieldset>
      </div>
    </Transition>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'
import { Setting, Brush, Monitor, Moon, Sunny, Promotion } from '@element-plus/icons-vue'
import { useThemeStore, type ThemePreference } from '@/stores/theme'
import { useLocaleStore, type LocaleCode } from '@/stores/locale'
import { useLocale } from '@/composables/useLocale'

const themeOptions: Array<{ value: ThemePreference; label: string; icon: typeof Monitor }> = [
  { value: 'system', label: 'theme.systemMode', icon: Monitor },
  { value: 'light', label: 'theme.lightMode', icon: Sunny },
  { value: 'dark', label: 'theme.darkMode', icon: Moon }
]

const localeOptions: Array<{ value: LocaleCode; label: string }> = [
  { value: 'zh-CN', label: '中文' },
  { value: 'en-US', label: 'EN' }
]

const themeStore = useThemeStore()
const localeStore = useLocaleStore()
const { setLocale } = useLocale()
const rootRef = ref<HTMLElement | null>(null)
const triggerRef = ref<HTMLButtonElement | null>(null)
const panelRef = ref<HTMLElement | null>(null)
const isOpen = ref(false)

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

const focusSelectedOption = () => {
  panelRef.value?.querySelector<HTMLInputElement>('input:checked')?.focus()
}

const toggleSettings = async () => {
  isOpen.value = !isOpen.value
  if (isOpen.value) {
    await nextTick()
    focusSelectedOption()
  }
}

const closeAndFocusTrigger = async () => {
  isOpen.value = false
  await nextTick()
  triggerRef.value?.focus()
}

const handleDocumentPointerDown = (event: PointerEvent) => {
  if (isOpen.value && !rootRef.value?.contains(event.target as Node)) {
    isOpen.value = false
  }
}

onMounted(() => document.addEventListener('pointerdown', handleDocumentPointerDown))
onBeforeUnmount(() => document.removeEventListener('pointerdown', handleDocumentPointerDown))
</script>

<style scoped lang="scss">
.theme-toggle {
  position: relative;
  display: flex;
}

.theme-toggle__trigger {
  width: 42px;
  height: 42px;
  display: grid;
  place-items: center;
  padding: 0;
  border-radius: 50%;
  background: color-mix(in srgb, var(--color-bg-primary) 80%, transparent);
  border: 1px solid var(--color-border-light);
  color: var(--navbar-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.16);
  cursor: pointer;

  svg {
    width: 18px;
    height: 18px;
  }

  &:hover,
  &:focus-visible {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--color-primary) 22%, transparent);
    box-shadow: var(--shadow-sm);
    outline: none;
  }
}

.theme-toggle__panel {
  position: absolute;
  top: calc(100% + 12px);
  right: 0;
  z-index: 1200;
  width: min(300px, calc(100vw - 28px));
  padding: 8px;
  border: 1px solid var(--color-border-light);
  border-radius: var(--radius-md);
  background:
    linear-gradient(var(--color-bg-secondary), var(--color-bg-secondary)), var(--color-bg-canvas);
  color: var(--color-text-primary);
  box-shadow: var(--shadow-md);
  backdrop-filter: blur(20px);
}

.settings-item {
  min-width: 0;
  margin: 0;
  padding: 10px 8px;
  border: 0;
}

.settings-item--divided {
  border-top: 1px solid var(--color-border-light);
}

.settings-item__label {
  width: 100%;
  display: flex;
  align-items: center;
  gap: 8px;
  margin: 0 0 8px;
  padding: 0;
  color: var(--color-text-primary);
  font-size: 14px;
  font-weight: 500;

  svg {
    width: 16px;
    height: 16px;
  }
}

.settings-segments {
  display: flex;
  width: 100%;
}

.settings-segment {
  min-width: 0;
  flex: 1 1 0;
  cursor: pointer;

  input {
    position: absolute;
    width: 1px;
    height: 1px;
    padding: 0;
    margin: -1px;
    overflow: hidden;
    clip: rect(0, 0, 0, 0);
    white-space: nowrap;
    border: 0;
  }

  &:not(:first-child) .settings-segment__content {
    margin-left: -1px;
  }

  &:first-child .settings-segment__content {
    border-radius: var(--radius-sm) 0 0 var(--radius-sm);
  }

  &:last-child .settings-segment__content {
    border-radius: 0 var(--radius-sm) var(--radius-sm) 0;
  }

  input:focus-visible + .settings-segment__content {
    position: relative;
    z-index: 1;
    outline: 2px solid color-mix(in srgb, var(--color-primary) 55%, transparent);
    outline-offset: 2px;
  }

  &.is-active .settings-segment__content {
    position: relative;
    z-index: 1;
    background: var(--color-primary);
    border-color: var(--color-primary);
    color: #ffffff;
  }
}

.settings-segment__content {
  min-height: 32px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 6px 8px;
  border: 1px solid var(--color-border-light);
  background: var(--color-bg-secondary);
  color: var(--color-text-regular);
  font-size: 12px;
  line-height: 1.2;
  text-align: center;
  transition:
    background-color 0.2s ease,
    border-color 0.2s ease,
    color 0.2s ease;

  svg {
    width: 14px;
    height: 14px;
    flex: 0 0 auto;
  }
}

.settings-popover-enter-active,
.settings-popover-leave-active {
  transition:
    opacity 0.18s ease,
    transform 0.18s ease;
}

.settings-popover-enter-from,
.settings-popover-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>
