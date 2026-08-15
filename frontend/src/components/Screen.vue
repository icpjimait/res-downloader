<template>
  <div class="flex items-center gap-4" style="--wails-draggable:no-drag">
    
    <!-- Minimize Button -->
    <button
        class="ctrl-btn"
        @click="minimizeWindow"
        :title="t('components.screen_minimize')"
    >
      <svg class="w-5 h-5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
        <path d="M4 12H20" />
      </svg>
    </button>

    <!-- Maximize/Restore Button -->
    <button
        class="ctrl-btn"
        @click="maximizeWindow"
        :title="isMaximized ? t('components.screen_restore') : t('components.screen_maximize')"
    >
      <svg v-if="isMaximized" class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round">
        <path d="M8 4V3C8 2.44772 8.44772 2 9 2H21C21.5523 2 22 2.44772 22 3V15C22 15.5523 21.5523 16 21 16H20"/>
        <path d="M4 8H15V19H4V8Z" />
      </svg>
      <svg v-else class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linejoin="round">
        <path d="M15 3h6v6M9 21H3v-6M21 3l-7 7M3 21l7-7"/>
      </svg>
    </button>
    
    <!-- Close Button -->
    <button
        class="ctrl-btn close"
        @click="closeWindow"
        :title="t('components.screen_close')"
    >
      <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round">
        <path d="M6 6L18 18M6 18L18 6" />
      </svg>
    </button>

  </div>
</template>

<script lang="ts" setup>
import {ref} from "vue"
import {Quit, WindowFullscreen, WindowMinimise, WindowUnfullscreen} from "../../wailsjs/runtime"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const isMaximized = ref(false)

const closeWindow = () => {
  Quit()
}
const minimizeWindow = () => {
  WindowMinimise()
}
const maximizeWindow = () => {
  isMaximized.value = !isMaximized.value;
  if (isMaximized.value) {
    WindowFullscreen()
  } else {
    WindowUnfullscreen()
  }
}
</script>

<style scoped>
.ctrl-btn {
  background: transparent;
  border: none;
  cursor: pointer;
  outline: none;
  color: var(--text-faint);
  opacity: 0.8;
  transition: all 0.2s ease;
  padding: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}
.ctrl-btn:hover {
  color: var(--accent);
  opacity: 1;
}
.ctrl-btn.close:hover {
  color: var(--red);
}
</style>
