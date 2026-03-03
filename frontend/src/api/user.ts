import client from './client'
import type { LoginReply, RegisterReply, UserInfo } from '@/types'

export const userApi = {
  register(data: { username: string; password: string; nickname: string; email: string }) {
    return client.post<RegisterReply>('/user/register', data)
  },

  login(data: { username: string; password: string }) {
    return client.post<LoginReply>('/user/login', data)
  },

  getUserInfo() {
    return client.get<{ user: UserInfo }>('/user/info')
  },

  updateUserInfo(data: { nickname?: string; email?: string; avatar?: string }) {
    return client.put<{ success: boolean }>('/user/info', data)
  },
}
