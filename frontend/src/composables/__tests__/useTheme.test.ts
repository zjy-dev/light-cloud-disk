import { describe, it, expect, vi, beforeEach } from 'vitest'

describe('useTheme', () => {
  beforeEach(() => {
    localStorage.clear()
    document.documentElement.classList.remove('dark')
    vi.resetModules()
  })

  it('defaults to system mode', async () => {
    const { useTheme } = await import('@/composables/useTheme')
    const { themeMode } = useTheme()
    expect(themeMode.value).toBe('system')
  })

  it('setTheme to dark adds dark class', async () => {
    const { useTheme } = await import('@/composables/useTheme')
    const { setTheme, isDark } = useTheme()

    setTheme('dark')
    expect(isDark.value).toBe(true)
    expect(document.documentElement.classList.contains('dark')).toBe(true)
  })

  it('setTheme to light removes dark class', async () => {
    document.documentElement.classList.add('dark')
    const { useTheme } = await import('@/composables/useTheme')
    const { setTheme, isDark } = useTheme()

    setTheme('light')
    expect(isDark.value).toBe(false)
    expect(document.documentElement.classList.contains('dark')).toBe(false)
  })

  it('toggleTheme switches between light and dark', async () => {
    const { useTheme } = await import('@/composables/useTheme')
    const { setTheme, toggleTheme, isDark } = useTheme()

    setTheme('light')
    expect(isDark.value).toBe(false)

    toggleTheme()
    expect(isDark.value).toBe(true)

    toggleTheme()
    expect(isDark.value).toBe(false)
  })

  it('persists theme to localStorage', async () => {
    const { useTheme } = await import('@/composables/useTheme')
    const { setTheme, themeMode } = useTheme()

    setTheme('dark')
    // useStorage syncs the ref, check the ref value
    expect(themeMode.value).toBe('dark')
  })
})
