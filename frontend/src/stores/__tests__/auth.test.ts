import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useAuthStore } from '@/stores/auth'

vi.mock('@/api/user', () => ({
  userApi: {
    login: vi.fn(),
    register: vi.fn(),
    getUserInfo: vi.fn(),
    updateUserInfo: vi.fn(),
  },
}))

vi.mock('@/router', () => ({
  default: {
    push: vi.fn(),
  },
}))

import { userApi } from '@/api/user'
import router from '@/router'

describe('auth store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    localStorage.clear()
    vi.clearAllMocks()
  })

  it('initializes with no auth state', () => {
    const store = useAuthStore()
    expect(store.token).toBe('')
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(store.loading).toBe(false)
  })

  it('reads token from localStorage on init', () => {
    localStorage.setItem('light-cloud-token', 'saved-token')
    const store = useAuthStore()
    expect(store.token).toBe('saved-token')
    expect(store.isAuthenticated).toBe(true)
  })

  it('login stores token and user, navigates to /', async () => {
    const mockUser = {
      id: 1,
      username: 'test',
      nickname: 'Test',
      email: 'test@test.com',
      avatar: '',
      storageUsed: 0,
      storageLimit: 10737418240,
      createdAt: 1000,
    }
    vi.mocked(userApi.login).mockResolvedValue({
      data: { token: 'jwt-123', expireAt: 9999, user: mockUser },
    } as never)

    const store = useAuthStore()
    await store.login('test', 'pass')

    expect(store.token).toBe('jwt-123')
    expect(store.user).toEqual(mockUser)
    expect(store.isAuthenticated).toBe(true)
    expect(localStorage.getItem('light-cloud-token')).toBe('jwt-123')
    expect(router.push).toHaveBeenCalledWith('/')
  })

  it('login sets loading to false on error', async () => {
    vi.mocked(userApi.login).mockRejectedValue(new Error('bad creds'))

    const store = useAuthStore()
    await expect(store.login('test', 'wrong')).rejects.toThrow('bad creds')
    expect(store.loading).toBe(false)
  })

  it('register calls API and navigates to /login', async () => {
    vi.mocked(userApi.register).mockResolvedValue({
      data: { userId: 1, username: 'newuser' },
    } as never)

    const store = useAuthStore()
    await store.register({
      username: 'newuser',
      password: 'pass',
      nickname: 'New',
      email: 'new@test.com',
    })

    expect(userApi.register).toHaveBeenCalledWith({
      username: 'newuser',
      password: 'pass',
      nickname: 'New',
      email: 'new@test.com',
    })
    expect(router.push).toHaveBeenCalledWith('/login')
    expect(store.loading).toBe(false)
  })

  it('fetchUserInfo updates user state', async () => {
    const mockUser = {
      id: 1,
      username: 'test',
      nickname: 'Test',
      email: 'test@test.com',
      avatar: '',
      storageUsed: 100,
      storageLimit: 10737418240,
      createdAt: 1000,
    }
    vi.mocked(userApi.getUserInfo).mockResolvedValue({
      data: { user: mockUser },
    } as never)

    const store = useAuthStore()
    await store.fetchUserInfo()

    expect(store.user).toEqual(mockUser)
  })

  it('fetchUserInfo calls logout on error', async () => {
    vi.mocked(userApi.getUserInfo).mockRejectedValue(new Error('unauthorized'))

    const store = useAuthStore()
    store.token = 'old-token'
    localStorage.setItem('light-cloud-token', 'old-token')

    await store.fetchUserInfo()

    expect(store.token).toBe('')
    expect(store.user).toBeNull()
    expect(localStorage.getItem('light-cloud-token')).toBeNull()
  })

  it('updateProfile calls API and refreshes user info', async () => {
    const mockUser = {
      id: 1,
      username: 'test',
      nickname: 'Updated',
      email: 'test@test.com',
      avatar: '',
      storageUsed: 0,
      storageLimit: 10737418240,
      createdAt: 1000,
    }
    vi.mocked(userApi.updateUserInfo).mockResolvedValue({
      data: { success: true },
    } as never)
    vi.mocked(userApi.getUserInfo).mockResolvedValue({
      data: { user: mockUser },
    } as never)

    const store = useAuthStore()
    await store.updateProfile({ nickname: 'Updated' })

    expect(userApi.updateUserInfo).toHaveBeenCalledWith({ nickname: 'Updated' })
    expect(store.user).toEqual(mockUser)
  })

  it('logout clears state and navigates to /login', () => {
    const store = useAuthStore()
    store.token = 'some-token'
    store.user = {
      id: 1,
      username: 'test',
      nickname: 'Test',
      email: 'test@test.com',
      avatar: '',
      storageUsed: 0,
      storageLimit: 10737418240,
      createdAt: 1000,
    }
    localStorage.setItem('light-cloud-token', 'some-token')

    store.logout()

    expect(store.token).toBe('')
    expect(store.user).toBeNull()
    expect(store.isAuthenticated).toBe(false)
    expect(localStorage.getItem('light-cloud-token')).toBeNull()
    expect(router.push).toHaveBeenCalledWith('/login')
  })
})
