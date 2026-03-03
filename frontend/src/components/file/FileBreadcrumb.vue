<script setup lang="ts">
import { useFileStore } from '@/stores/file'
import { ChevronRight } from 'lucide-vue-next'

const fileStore = useFileStore()
</script>

<template>
  <nav class="flex items-center gap-1 text-sm min-h-[32px]">
    <template v-for="(crumb, index) in fileStore.breadcrumb" :key="crumb.id">
      <ChevronRight
        v-if="index > 0"
        class="w-3.5 h-3.5 text-text-tertiary shrink-0"
      />
      <button
        class="px-2 py-1 rounded-md text-text-secondary hover:text-text-primary
               hover:bg-surface-hover transition-all duration-150 truncate max-w-[160px]"
        :class="{
          '!text-text-primary font-medium': index === fileStore.breadcrumb.length - 1,
        }"
        @click="fileStore.navigateToFolder(crumb.id, crumb.name)"
      >
        {{ crumb.name }}
      </button>
    </template>

    <span
      v-if="fileStore.searchKeyword"
      class="ml-2 px-2.5 py-0.5 rounded-full bg-accent-soft text-accent text-xs font-medium"
    >
      Search: {{ fileStore.searchKeyword }}
    </span>
  </nav>
</template>
