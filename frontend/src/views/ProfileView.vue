<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { User, Mail, HardDrive, Calendar, Save, Loader2 } from 'lucide-vue-next'

const auth = useAuthStore()

const nickname = ref('')
const email = ref('')
const saving = ref(false)
const saved = ref(false)

onMounted(() => {
  if (auth.user) {
    nickname.value = auth.user.nickname
    email.value = auth.user.email
  }
})

const storagePercent = computed(() => {
  if (!auth.user) return 0
  return Math.round((auth.user.storageUsed / auth.user.storageLimit) * 100)
})

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

function formatDate(timestamp: number): string {
  if (!timestamp) return '--'
  return new Date(timestamp * 1000).toLocaleDateString('en-US', {
    month: 'long',
    day: 'numeric',
    year: 'numeric',
  })
}

async function onSave() {
  saving.value = true
  try {
    await auth.updateProfile({
      nickname: nickname.value,
      email: email.value,
    })
    saved.value = true
    setTimeout(() => (saved.value = false), 2000)
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="max-w-2xl space-y-6">
    <div>
      <h1 class="font-display font-semibold text-xl text-text-primary">Profile</h1>
      <p class="text-sm text-text-tertiary mt-0.5">Manage your account settings</p>
    </div>

    <!-- Avatar + Info card -->
    <div class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-6">
      <div class="flex items-center gap-5 mb-6">
        <div
          class="w-16 h-16 rounded-2xl bg-accent-soft text-accent
                 flex items-center justify-center text-2xl font-display font-semibold"
        >
          {{ auth.user?.nickname?.charAt(0)?.toUpperCase() || auth.user?.username?.charAt(0)?.toUpperCase() }}
        </div>
        <div>
          <h2 class="font-display font-semibold text-lg text-text-primary">
            {{ auth.user?.nickname || auth.user?.username }}
          </h2>
          <p class="text-sm text-text-secondary">@{{ auth.user?.username }}</p>
        </div>
      </div>

      <form @submit.prevent="onSave" class="space-y-4">
        <div>
          <label class="flex items-center gap-1.5 text-sm font-medium text-text-secondary mb-1.5">
            <User class="w-3.5 h-3.5" />
            Nickname
          </label>
          <input
            v-model="nickname"
            type="text"
            class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                   text-text-primary text-sm placeholder:text-text-tertiary
                   focus:border-accent focus:ring-2 focus:ring-accent-soft
                   outline-none transition-all"
          />
        </div>

        <div>
          <label class="flex items-center gap-1.5 text-sm font-medium text-text-secondary mb-1.5">
            <Mail class="w-3.5 h-3.5" />
            Email
          </label>
          <input
            v-model="email"
            type="email"
            class="w-full h-10 px-3 rounded-xl border border-border bg-surface
                   text-text-primary text-sm placeholder:text-text-tertiary
                   focus:border-accent focus:ring-2 focus:ring-accent-soft
                   outline-none transition-all"
          />
        </div>

        <div class="flex items-center gap-3 pt-2">
          <button
            type="submit"
            :disabled="saving"
            class="flex items-center gap-2 h-9 px-4 rounded-xl bg-accent text-accent-text
                   text-sm font-medium hover:bg-accent-hover disabled:opacity-50
                   transition-colors"
          >
            <Loader2 v-if="saving" class="w-4 h-4 animate-spin" />
            <Save v-else class="w-4 h-4" />
            {{ saving ? 'Saving...' : 'Save changes' }}
          </button>
          <span v-if="saved" class="text-sm text-success">Saved!</span>
        </div>
      </form>
    </div>

    <!-- Storage card -->
    <div class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-6">
      <div class="flex items-center gap-2 mb-4">
        <HardDrive class="w-4 h-4 text-text-secondary" />
        <h3 class="font-display font-medium text-text-primary">Storage</h3>
      </div>

      <div class="space-y-3">
        <div class="flex justify-between text-sm">
          <span class="text-text-secondary">Used</span>
          <span class="text-text-primary font-medium">
            {{ formatSize(auth.user?.storageUsed ?? 0) }}
            /
            {{ formatSize(auth.user?.storageLimit ?? 0) }}
          </span>
        </div>

        <div class="h-2.5 rounded-full bg-surface-hover overflow-hidden">
          <div
            class="h-full rounded-full transition-all duration-500"
            :class="storagePercent > 90 ? 'bg-danger' : storagePercent > 70 ? 'bg-warning' : 'bg-accent'"
            :style="{ width: `${storagePercent}%` }"
          />
        </div>

        <p class="text-xs text-text-tertiary">
          {{ storagePercent }}% of storage used
        </p>
      </div>
    </div>

    <!-- Account info card -->
    <div class="bg-surface-elevated rounded-2xl border border-border-subtle shadow-card p-6">
      <div class="flex items-center gap-2 mb-4">
        <Calendar class="w-4 h-4 text-text-secondary" />
        <h3 class="font-display font-medium text-text-primary">Account</h3>
      </div>

      <div class="space-y-3 text-sm">
        <div class="flex justify-between">
          <span class="text-text-secondary">Username</span>
          <span class="text-text-primary">{{ auth.user?.username }}</span>
        </div>
        <div class="flex justify-between">
          <span class="text-text-secondary">Member since</span>
          <span class="text-text-primary">{{ formatDate(auth.user?.createdAt ?? 0) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>
