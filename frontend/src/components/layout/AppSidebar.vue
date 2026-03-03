<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  FolderOpen,
  Trash2,
  User,
  LogOut,
  Cloud,
  ChevronLeft,
} from 'lucide-vue-next'

const route = useRoute()
const auth = useAuthStore()

const collapsed = ref(false)

const navItems = [
  { name: 'files', label: 'My Files', icon: FolderOpen, path: '/' },
  { name: 'trash', label: 'Trash', icon: Trash2, path: '/trash' },
  { name: 'profile', label: 'Profile', icon: User, path: '/profile' },
]

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
</script>

<template>
  <aside
    class="flex flex-col h-screen bg-sidebar border-r border-border-subtle transition-all duration-300"
    :class="collapsed ? 'w-[68px]' : 'w-[240px]'"
  >
    <!-- Logo -->
    <div class="flex items-center gap-3 px-5 h-16 shrink-0">
      <div
        class="flex items-center justify-center w-9 h-9 rounded-xl
               bg-accent text-accent-text shrink-0"
      >
        <Cloud class="w-5 h-5" />
      </div>
      <transition name="fade">
        <span
          v-if="!collapsed"
          class="font-display font-semibold text-lg text-text-primary whitespace-nowrap"
        >
          Light Cloud
        </span>
      </transition>
    </div>

    <!-- Navigation -->
    <nav class="flex-1 px-3 py-4 space-y-1">
      <router-link
        v-for="item in navItems"
        :key="item.name"
        :to="item.path"
        class="flex items-center gap-3 px-3 h-10 rounded-lg
               text-text-secondary hover:text-text-primary
               hover:bg-surface-hover transition-all duration-150"
        :class="{
          'bg-sidebar-active !text-accent font-medium': route.name === item.name,
          'justify-center': collapsed,
        }"
      >
        <component :is="item.icon" class="w-[18px] h-[18px] shrink-0" />
        <transition name="fade">
          <span v-if="!collapsed" class="text-sm">{{ item.label }}</span>
        </transition>
      </router-link>
    </nav>

    <!-- Storage usage -->
    <div v-if="auth.user" class="px-3 pb-3">
      <div
        class="rounded-lg bg-surface-hover p-3"
        :class="collapsed ? 'px-2' : ''"
      >
        <transition name="fade">
          <div v-if="!collapsed">
            <div class="flex justify-between text-xs text-text-tertiary mb-2">
              <span>Storage</span>
              <span>{{ storagePercent }}%</span>
            </div>
            <div class="h-1.5 rounded-full bg-border-subtle overflow-hidden">
              <div
                class="h-full rounded-full bg-accent transition-all duration-500"
                :style="{ width: `${storagePercent}%` }"
              />
            </div>
            <div class="text-xs text-text-tertiary mt-1.5">
              {{ formatSize(auth.user.storageUsed) }} / {{ formatSize(auth.user.storageLimit) }}
            </div>
          </div>
        </transition>
        <div v-if="collapsed" class="flex justify-center">
          <div
            class="w-6 h-6 rounded-full border-2 border-accent flex items-center justify-center"
          >
            <span class="text-[8px] font-bold text-accent">{{ storagePercent }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Bottom actions -->
    <div class="px-3 pb-4 space-y-1">
      <button
        class="flex items-center gap-3 px-3 h-10 w-full rounded-lg
               text-text-secondary hover:text-danger hover:bg-danger-soft
               transition-all duration-150"
        :class="collapsed ? 'justify-center' : ''"
        @click="auth.logout()"
      >
        <LogOut class="w-[18px] h-[18px] shrink-0" />
        <transition name="fade">
          <span v-if="!collapsed" class="text-sm">Log out</span>
        </transition>
      </button>
      <button
        class="flex items-center justify-center w-full h-8 rounded-lg
               text-text-tertiary hover:text-text-secondary hover:bg-surface-hover
               transition-all duration-150"
        @click="collapsed = !collapsed"
      >
        <ChevronLeft
          class="w-4 h-4 transition-transform duration-300"
          :class="collapsed ? 'rotate-180' : ''"
        />
      </button>
    </div>
  </aside>
</template>
