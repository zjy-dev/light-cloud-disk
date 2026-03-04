import { describe, it, expect, beforeEach, vi } from 'vitest'

describe('router auth guard', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetModules()
  })

  it('redirects to login when accessing auth route without token', async () => {
    localStorage.removeItem('light-cloud-token')

    const { default: router } = await import('@/router')
    await router.push('/')

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('allows access to auth route with token', async () => {
    localStorage.setItem('light-cloud-token', 'valid-token')

    const { default: router } = await import('@/router')
    await router.push('/')

    expect(router.currentRoute.value.name).toBe('files')
  })

  it('redirects authenticated users from login to files', async () => {
    localStorage.setItem('light-cloud-token', 'valid-token')

    const { default: router } = await import('@/router')
    // Navigate to a neutral page first, then verify redirect behavior
    await router.push('/')
    await router.push('/login')

    expect(router.currentRoute.value.name).toBe('files')
  })

  it('allows unauthenticated users to access login', async () => {
    localStorage.removeItem('light-cloud-token')

    const { default: router } = await import('@/router')
    await router.push('/login')

    expect(router.currentRoute.value.name).toBe('login')
  })

  it('allows access to share route with token', async () => {
    localStorage.setItem('light-cloud-token', 'valid-token')

    const { default: router } = await import('@/router')
    await router.push('/share/abc123')

    expect(router.currentRoute.value.name).toBe('share-view')
    expect(router.currentRoute.value.params.shareId).toBe('abc123')
  })

  it('allows unauthenticated access to share route', async () => {
    localStorage.removeItem('light-cloud-token')

    const { default: router } = await import('@/router')
    await router.push('/share/xyz789')

    expect(router.currentRoute.value.name).toBe('share-view')
  })

  it('allows access to register page without token', async () => {
    localStorage.removeItem('light-cloud-token')

    const { default: router } = await import('@/router')
    await router.push('/register')

    expect(router.currentRoute.value.name).toBe('register')
  })

  it('redirects authenticated users from register to files', async () => {
    localStorage.setItem('light-cloud-token', 'valid-token')

    const { default: router } = await import('@/router')
    // Navigate to authenticated home first, then try register route
    await router.push('/')
    await router.push('/register')

    expect(router.currentRoute.value.name).toBe('files')
  })
})
