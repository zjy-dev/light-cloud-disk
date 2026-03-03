import { ref, watch } from 'vue'
import { usePreferredDark, useStorage } from '@vueuse/core'

export type ThemeMode = 'light' | 'dark' | 'system'

const themeMode = useStorage<ThemeMode>('light-cloud-theme', 'system')
const isDark = ref(false)

export function useTheme() {
  const prefersDark = usePreferredDark()

  function applyTheme() {
    const shouldBeDark =
      themeMode.value === 'dark' ||
      (themeMode.value === 'system' && prefersDark.value)

    isDark.value = shouldBeDark
    document.documentElement.classList.toggle('dark', shouldBeDark)
  }

  function setTheme(mode: ThemeMode) {
    themeMode.value = mode
    applyTheme()
  }

  function toggleTheme() {
    if (themeMode.value === 'system') {
      setTheme(prefersDark.value ? 'light' : 'dark')
    } else {
      setTheme(themeMode.value === 'dark' ? 'light' : 'dark')
    }
  }

  watch(prefersDark, applyTheme)
  applyTheme()

  return {
    themeMode,
    isDark,
    setTheme,
    toggleTheme,
  }
}
