<script setup lang="ts">
import { ref } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { Cloud, Eye, EyeOff } from 'lucide-vue-next'
import { useTheme } from '@/composables/useTheme'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'

useTheme()
const auth = useAuthStore()

const username = ref('')
const password = ref('')
const showPassword = ref(false)
const error = ref('')

async function onSubmit() {
  error.value = ''
  try {
    await auth.login(username.value, password.value)
  } catch (err: unknown) {
    if (err && typeof err === 'object' && 'response' in err) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      error.value = axiosErr.response?.data?.error ?? 'Login failed'
    } else {
      error.value = 'Login failed'
    }
  }
}
</script>

<template>
  <div class="min-h-screen flex items-center justify-center bg-surface p-4 relative">
    <!-- Theme toggle in corner -->
    <div class="absolute top-5 right-5">
      <ThemeToggle />
    </div>

    <div class="w-full max-w-[400px]">
      <!-- Logo -->
      <div class="flex flex-col items-center mb-10">
        <div
          class="flex items-center justify-center w-14 h-14 rounded-2xl
                 bg-accent text-accent-text mb-4"
        >
          <Cloud class="w-7 h-7" />
        </div>
        <h1 class="font-display font-semibold text-2xl text-text-primary">
          Welcome back
        </h1>
        <p class="text-sm text-text-secondary mt-1">
          Sign in to Light Cloud
        </p>
      </div>

      <!-- Form card -->
      <div
        class="bg-surface-elevated rounded-2xl border border-border-subtle
               shadow-card p-7"
      >
        <form @submit.prevent="onSubmit" class="space-y-5">
          <!-- Error -->
          <div
            v-if="error"
            class="text-sm text-danger bg-danger-soft rounded-lg px-4 py-2.5"
          >
            {{ error }}
          </div>

          <!-- Username -->
          <div>
            <label class="block text-sm font-medium text-text-secondary mb-1.5">
              Username
            </label>
            <input
              v-model="username"
              type="text"
              required
              autocomplete="username"
              class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                     text-text-primary text-sm placeholder:text-text-tertiary
                     focus:border-accent focus:ring-2 focus:ring-accent-soft
                     outline-none transition-all"
              placeholder="Enter your username"
            />
          </div>

          <!-- Password -->
          <div>
            <label class="block text-sm font-medium text-text-secondary mb-1.5">
              Password
            </label>
            <div class="relative">
              <input
                v-model="password"
                :type="showPassword ? 'text' : 'password'"
                required
                autocomplete="current-password"
                class="w-full h-10 px-3 pr-10 rounded-xl border border-border bg-surface
                       text-text-primary text-sm placeholder:text-text-tertiary
                       focus:border-accent focus:ring-2 focus:ring-accent-soft
                       outline-none transition-all"
                placeholder="Enter your password"
              />
              <button
                type="button"
                class="absolute right-2.5 top-1/2 -translate-y-1/2 text-text-tertiary
                       hover:text-text-secondary transition-colors"
                @click="showPassword = !showPassword"
              >
                <component :is="showPassword ? EyeOff : Eye" class="w-4 h-4" />
              </button>
            </div>
          </div>

          <!-- Submit -->
          <button
            type="submit"
            :disabled="auth.loading"
            class="w-full h-10 rounded-xl bg-accent text-accent-text text-sm font-medium
                   hover:bg-accent-hover disabled:opacity-50 disabled:cursor-not-allowed
                   transition-colors duration-150"
          >
            <span v-if="auth.loading" class="flex items-center justify-center gap-2">
              <span class="w-4 h-4 border-2 border-accent-text/30 border-t-accent-text rounded-full animate-spin" />
              Signing in...
            </span>
            <span v-else>Sign in</span>
          </button>
        </form>
      </div>

      <!-- Register link -->
      <p class="text-center text-sm text-text-secondary mt-6">
        Don't have an account?
        <router-link
          to="/register"
          class="text-accent hover:text-accent-hover font-medium transition-colors"
        >
          Create one
        </router-link>
      </p>
    </div>
  </div>
</template>
