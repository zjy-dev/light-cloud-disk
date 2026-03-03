<script setup lang="ts">
import { onMounted, onUnmounted, watch } from 'vue'
import { X } from 'lucide-vue-next'

const props = defineProps<{
  open: boolean
  title?: string
  size?: 'sm' | 'md' | 'lg'
}>()

const emit = defineEmits<{
  close: []
}>()

const sizeClass = {
  sm: 'max-w-sm',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
}

function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape') emit('close')
}

onMounted(() => document.addEventListener('keydown', onKeydown))
onUnmounted(() => document.removeEventListener('keydown', onKeydown))

watch(() => props.open, (val) => {
  document.body.style.overflow = val ? 'hidden' : ''
})
</script>

<template>
  <Teleport to="body">
    <transition name="fade">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
      >
        <!-- Backdrop -->
        <div
          class="absolute inset-0 bg-overlay"
          @click="emit('close')"
        />

        <!-- Panel -->
        <div
          class="relative w-full rounded-2xl bg-surface-elevated shadow-modal
                 border border-border-subtle p-6 z-10"
          :class="sizeClass[size ?? 'md']"
        >
          <!-- Header -->
          <div v-if="title" class="flex items-center justify-between mb-5">
            <h2 class="font-display font-semibold text-lg text-text-primary">
              {{ title }}
            </h2>
            <button
              class="p-1.5 rounded-lg hover:bg-surface-hover text-text-tertiary
                     hover:text-text-secondary transition-colors"
              @click="emit('close')"
            >
              <X class="w-4 h-4" />
            </button>
          </div>

          <slot />
        </div>
      </div>
    </transition>
  </Teleport>
</template>
