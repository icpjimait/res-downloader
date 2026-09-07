<template>
  <div
      class="min-h-6 flex items-center cursor-pointer group"
      @click="handleOnClick"
  >
    <n-input
        v-if="isEdit"
        ref="inputRef"
        size="small"
        placeholder="输入自定义描述..."
        :value="inputValue"
        @update:value="v => inputValue = v"
        @change="handleChange"
        @blur="handleChange"
        @keyup.enter="handleChange"
    />

    <n-tooltip
        v-else
        trigger="hover"
        placement="top"
    >
      <template #trigger>
        <div class="flex items-center gap-1 max-w-full">
          <span
            v-if="inputValue"
            class="text-[12px] truncate block"
            style="color: var(--text);"
          >
            {{ inputValue }}
          </span>
          <span
            v-else
            class="text-[12px] transition-colors"
            style="color: var(--text-faint);"
          >
            —
          </span>
          <svg class="w-3 h-3 opacity-0 group-hover:opacity-60 transition-opacity flex-shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M11 4H4a2 2 0 00-2 2v14a2 2 0 002 2h14a2 2 0 002-2v-7" />
            <path d="M18.5 2.5a2.121 2.121 0 013 3L12 15l-4 1 1-4 9.5-9.5z" />
          </svg>
        </div>
      </template>
      <div style="max-width: 450px; font-size: 12px; word-break: break-all; line-height: 1.5;">
        {{ inputValue || '点击自定义/编辑描述' }}
      </div>
    </n-tooltip>
  </div>
</template>

<script setup lang="ts">
import { ref, nextTick, watch } from 'vue'
import { NInput, NTooltip } from 'naive-ui'
import type { InputInst } from 'naive-ui'

interface OnUpdateValue {
  (value: string): void
}

const props = defineProps<{
  value: string | number
  onUpdateValue?: OnUpdateValue
}>()

const isEdit = ref(false)
const inputRef = ref<InputInst | null>(null)
const inputValue = ref(String(props.value || ''))

watch(
    () => props.value,
    v => inputValue.value = String(v || '')
)

function handleOnClick() {
  isEdit.value = true
  nextTick(() => inputRef.value?.focus())
}

function handleChange() {
  props.onUpdateValue?.(String(inputValue.value).trim())
  isEdit.value = false
}
</script>
