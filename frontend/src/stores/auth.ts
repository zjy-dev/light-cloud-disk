import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { userApi } from '@/api/user'
import type { UserInfo } from '@/types'
import router from '@/router'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('light-cloud-token') ?? '')
  const user = ref<UserInfo | null>(null)
  const loading = ref(false)

  const isAuthenticated = computed(() => !!token.value)

  async function login(username: string, password: string) {
    loading.value = true
    try {
      const { data } = await userApi.login({ username, password })
      token.value = data.token
      user.value = data.user
      localStorage.setItem('light-cloud-token', data.token)
      await router.push('/')
    } finally {
      loading.value = false
    }
  }

  async function register(form: {
    username: string
    password: string
    nickname: string
    email: string
  }) {
    loading.value = true
    try {
      await userApi.register(form)
      await router.push('/login')
    } finally {
      loading.value = false
    }
  }

  async function fetchUserInfo() {
    try {
      const { data } = await userApi.getUserInfo()
      user.value = data.user
    } catch {
      logout()
    }
  }

  async function updateProfile(data: { nickname?: string; email?: string; avatar?: string }) {
    await userApi.updateUserInfo(data)
    await fetchUserInfo()
  }

  function logout() {
    token.value = ''
    user.value = null
    localStorage.removeItem('light-cloud-token')
    router.push('/login')
  }

  return {
    token,
    user,
    loading,
    isAuthenticated,
    login,
    register,
    fetchUserInfo,
    updateProfile,
    logout,
  }
})
