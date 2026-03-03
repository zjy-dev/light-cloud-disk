<script setup lang="ts">
import { computed } from 'vue'
import { useTheme } from '@/composables/useTheme'
import { Sun, Moon, Monitor } from 'lucide-vue-next'

const { themeMode, isDark, toggleTheme } = useTheme()

const icon = computed(() => {
  if (themeMode.value === 'system') return Monitor
  return isDark.value ? Moon : Sun
})
</script>

<template>
  <button
    class="relative flex items-center justify-center w-9 h-9 rounded-lg
           bg-surface-hover hover:bg-surface-active
           text-text-secondary hover:text-text-primary
           transition-all duration-200 focus-ring"
    :title="`Theme: ${themeMode}`"
    @click="toggleTheme"
  >
    <transition name="fade" mode="out-in">
      <component :is="icon" :key="themeMode" class="w-[18px] h-[18px]" />
    </transition>
  </button>
</template>
