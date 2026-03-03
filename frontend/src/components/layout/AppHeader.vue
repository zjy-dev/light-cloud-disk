<script setup lang="ts">
import { ref } from 'vue'
import { Search, X } from 'lucide-vue-next'
import { useFileStore } from '@/stores/file'
import { useAuthStore } from '@/stores/auth'
import ThemeToggle from '@/components/ui/ThemeToggle.vue'

const fileStore = useFileStore()
const auth = useAuthStore()
const searchInput = ref('')
const searchFocused = ref(false)

function onSearch() {
  fileStore.search(searchInput.value)
}

function clearSearch() {
  searchInput.value = ''
  fileStore.search('')
}
</script>

<template>
  <header
    class="flex items-center gap-4 h-16 px-6 bg-surface-elevated border-b border-border-subtle shrink-0"
  >
    <!-- Search bar -->
    <div
      class="relative flex items-center flex-1 max-w-xl"
    >
      <div
        class="flex items-center w-full h-10 rounded-xl border px-3 gap-2
               transition-all duration-200"
        :class="
          searchFocused
            ? 'border-accent bg-surface-elevated shadow-card'
            : 'border-border-subtle bg-surface-hover'
        "
      >
        <Search class="w-4 h-4 text-text-tertiary shrink-0" />
        <input
          v-model="searchInput"
          type="text"
          placeholder="Search files..."
          class="flex-1 bg-transparent text-sm text-text-primary placeholder:text-text-tertiary
                 outline-none"
          @focus="searchFocused = true"
          @blur="searchFocused = false"
          @keydown.enter="onSearch"
        />
        <button
          v-if="searchInput"
          class="p-0.5 rounded hover:bg-surface-active text-text-tertiary hover:text-text-secondary
                 transition-colors"
          @click="clearSearch"
        >
          <X class="w-3.5 h-3.5" />
        </button>
      </div>
    </div>

    <!-- Right side -->
    <div class="flex items-center gap-3">
      <ThemeToggle />
      <div
        v-if="auth.user"
        class="flex items-center gap-2 pl-3 border-l border-border-subtle"
      >
        <div
          class="w-8 h-8 rounded-full bg-accent-soft text-accent
                 flex items-center justify-center text-sm font-semibold"
        >
          {{ auth.user.nickname?.charAt(0)?.toUpperCase() || auth.user.username?.charAt(0)?.toUpperCase() }}
        </div>
        <span class="text-sm font-medium text-text-primary hidden sm:block">
          {{ auth.user.nickname || auth.user.username }}
        </span>
      </div>
    </div>
  </header>
</template>
