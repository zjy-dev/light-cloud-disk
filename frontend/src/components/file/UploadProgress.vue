<script setup lang="ts">
import { computed } from 'vue'
import { useUpload } from '@/composables/useUpload'
import {
  Upload,
  Check,
  AlertCircle,
  X,
  Loader2,
} from 'lucide-vue-next'

const { tasks, clearCompleted, removeTask } = useUpload()

const hasTasks = computed(() => tasks.value.length > 0)

function statusLabel(status: string) {
  const labels: Record<string, string> = {
    pending: 'Waiting',
    hashing: 'Computing hash...',
    uploading: 'Uploading',
    completing: 'Finalizing...',
    done: 'Complete',
    error: 'Failed',
  }
  return labels[status] ?? status
}
</script>

<template>
  <transition name="slide-fade">
    <div
      v-if="hasTasks"
      class="fixed bottom-5 right-5 w-80 z-40 rounded-2xl bg-surface-elevated
             border border-border-subtle shadow-dropdown overflow-hidden"
    >
      <!-- Header -->
      <div class="flex items-center justify-between px-4 py-3 border-b border-border-subtle">
        <div class="flex items-center gap-2">
          <Upload class="w-4 h-4 text-accent" />
          <span class="text-sm font-medium text-text-primary">Uploads</span>
          <span class="text-xs text-text-tertiary">({{ tasks.length }})</span>
        </div>
        <button
          class="text-xs text-text-tertiary hover:text-text-secondary transition-colors"
          @click="clearCompleted"
        >
          Clear done
        </button>
      </div>

      <!-- Tasks -->
      <div class="max-h-60 overflow-y-auto">
        <div
          v-for="task in tasks"
          :key="task.id"
          class="flex items-center gap-3 px-4 py-2.5 border-b border-border-subtle last:border-0"
        >
          <!-- Status icon -->
          <div class="shrink-0">
            <Check v-if="task.status === 'done'" class="w-4 h-4 text-success" />
            <AlertCircle v-else-if="task.status === 'error'" class="w-4 h-4 text-danger" />
            <Loader2 v-else class="w-4 h-4 text-accent animate-spin" />
          </div>

          <!-- Info -->
          <div class="flex-1 min-w-0">
            <div class="text-xs text-text-primary truncate">{{ task.fileName }}</div>
            <div class="text-[10px] text-text-tertiary">
              {{ statusLabel(task.status) }}
              <span v-if="task.status === 'uploading' || task.status === 'completing'">
                {{ task.progress }}%
              </span>
            </div>
            <!-- Progress bar -->
            <div
              v-if="task.status === 'uploading' || task.status === 'completing' || task.status === 'hashing'"
              class="h-1 rounded-full bg-border-subtle mt-1 overflow-hidden"
            >
              <div
                class="h-full rounded-full bg-accent transition-all duration-300"
                :style="{ width: `${task.progress}%` }"
              />
            </div>
            <div v-if="task.error" class="text-[10px] text-danger mt-0.5">
              {{ task.error }}
            </div>
          </div>

          <!-- Remove -->
          <button
            v-if="task.status === 'done' || task.status === 'error'"
            class="shrink-0 p-1 rounded-md text-text-tertiary hover:text-text-secondary
                   hover:bg-surface-hover transition-colors"
            @click="removeTask(task.id)"
          >
            <X class="w-3 h-3" />
          </button>
        </div>
      </div>
    </div>
  </transition>
</template>
