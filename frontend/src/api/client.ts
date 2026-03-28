import axios from 'axios'
import router from '@/router'

/** Recursively convert snake_case keys to camelCase */
function snakeToCamel(data: unknown): unknown {
  if (Array.isArray(data)) return data.map(snakeToCamel)
  if (data !== null && typeof data === 'object' && !(data instanceof Blob)) {
    const result: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(data as Record<string, unknown>)) {
      const camelKey = key.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
      result[camelKey] = snakeToCamel(value)
    }
    return result
  }
  return data
}

const client = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json',
  },
})

client.interceptors.request.use((config) => {
  if (config.data instanceof FormData && config.headers) {
    delete (config.headers as Record<string, string>)['Content-Type']
  }

  const token = localStorage.getItem('light-cloud-token')
  if (token) {
    config.headers.Authorization = `Bearer ${token}`
  }
  return config
})

client.interceptors.response.use(
  (response) => {
    if (response.data && response.config.responseType !== 'blob') {
      response.data = snakeToCamel(response.data)
    }
    return response
  },
  (error) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('light-cloud-token')
      router.push('/login')
    }
    return Promise.reject(error)
  },
)

export default client
