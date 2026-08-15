<template>
  <NConfigProvider class="h-full" :theme="appTheme" :locale="uiLocale" :theme-overrides="appThemeOverrides">
    <NaiveProvider>
      <div class="w-full h-full flex flex-col">
        <!-- Window Title Bar -->
        <div class="w-full flex justify-between items-center px-4 flex-shrink-0" style="height: 38px; background: var(--panel); border-bottom: 1px solid var(--border-soft); --wails-draggable:drag; user-select: none;">
          <div class="flex items-center gap-2" style="color: var(--text-dim); font-size: 12.5px; font-weight: 600;">
            <img src="@/assets/image/logo.png" style="width: 18px; height: 18px; border-radius: 4px;" />
            <span>{{ store.appInfo.AppName || 'Res Downloader' }}</span>
          </div>
          <Screen style="--wails-draggable:no-drag" v-if="store.envInfo.platform!=='darwin'" />
        </div>
        
        <!-- Main Content -->
        <div class="flex-1 min-h-0 overflow-hidden relative">
          <RouterView/>
        </div>
      </div>
    </NaiveProvider>
    <NGlobalStyle/>
    <NModalProvider/>
  </NConfigProvider>
</template>

<script setup lang="ts">
import NaiveProvider from '@/components/NaiveProvider.vue'
import Screen from '@/components/Screen.vue'
import {darkTheme, zhCN, enUS, type GlobalThemeOverrides} from 'naive-ui'
import {useIndexStore} from "@/stores"
import {computed, onMounted, watchEffect} from "vue"
import {useEventStore} from "@/stores/event"
import type {appType} from "@/types/app"
import {useI18n} from 'vue-i18n'

const store = useIndexStore()
const eventStore = useEventStore()
const {locale} = useI18n()

const appTheme = computed(() => store.globalConfig.Theme === 'darkTheme' ? darkTheme : null)

const darkThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#C9A15C',
    primaryColorHover: '#D4AE6B',
    primaryColorPressed: '#B8913D',
    primaryColorSuppl: '#C9A15C',
    bodyColor: '#12151C',
    cardColor: '#1A1E28',
    modalColor: '#1A1E28',
    popoverColor: '#1A1E28',
    tableColor: '#12151C',
    inputColor: '#1F2430',
    actionColor: '#1F2430',
    borderColor: '#2A2F3C',
    dividerColor: '#232836',
    textColorBase: '#E7E9EE',
    textColor1: '#E7E9EE',
    textColor2: '#8B92A5',
    textColor3: '#5B6172',
    placeholderColor: '#5B6172',
    hoverColor: '#1F2430',
    tableHeaderColor: '#12151C'
  },
  Input: {
    borderRadius: '7px',
    heightMedium: '36px',
    color: '#1F2430',
    colorFocus: '#1F2430',
    border: '1px solid #2A2F3C',
    borderFocus: '1px solid #C9A15C',
    boxShadowFocus: '0 0 0 2px rgba(201,161,92,0.15)',
    textColor: '#E7E9EE',
    placeholderColor: '#5B6172',
    caretColor: '#C9A15C'
  },
  Select: {
    peers: {
      InternalSelection: {
        borderRadius: '7px',
        heightMedium: '36px',
        color: '#1F2430',
        border: '1px solid #2A2F3C',
        borderFocus: '1px solid #C9A15C',
        boxShadowFocus: '0 0 0 2px rgba(201,161,92,0.15)',
        textColor: '#E7E9EE'
      }
    }
  },
  Button: {
    borderRadiusMedium: '7px',
    heightMedium: '36px',
    colorSecondary: '#1F2430',
    colorSecondaryHover: '#262C3A',
    textColorSecondary: '#E7E9EE',
    borderSecondary: '1px solid #2A2F3C'
  },
  Switch: {
    railColorActive: '#C9A15C',
    loadingColor: '#C9A15C'
  },
  DataTable: {
    borderColor: '#232836',
    tdColor: 'transparent',
    thColor: '#12151C',
    thTextColor: '#5B6172',
    tdTextColor: '#E7E9EE',
    borderRadius: '0px'
  },
  Tabs: {
    tabTextColorActiveLine: '#C9A15C',
    tabTextColorHoverLine: '#C9A15C',
    barColor: '#C9A15C'
  },
  Popover: {
    color: '#1A1E28',
    textColor: '#E7E9EE',
    borderRadius: '8px'
  },
  Dialog: {
    color: '#1A1E28',
    textColor: '#E7E9EE',
    titleTextColor: '#E7E9EE'
  },
  InputNumber: {
    borderRadius: '7px',
    heightMedium: '36px'
  },
  Tooltip: {
    color: '#1A1E28',
    textColor: '#E7E9EE'
  }
}

const lightThemeOverrides: GlobalThemeOverrides = {
  common: {
    primaryColor: '#C9A15C',
    primaryColorHover: '#D4AE6B',
    primaryColorPressed: '#B8913D',
    primaryColorSuppl: '#C9A15C',
    bodyColor: '#F8F9FA',
    cardColor: '#FFFFFF',
    modalColor: '#FFFFFF',
    popoverColor: '#FFFFFF',
    tableColor: '#F8F9FA',
    inputColor: '#F1F3F5',
    actionColor: '#F1F3F5',
    borderColor: '#E5E7EB',
    dividerColor: '#F3F4F6',
    textColorBase: '#1F2937',
    textColor1: '#1F2937',
    textColor2: '#4B5563',
    textColor3: '#9CA3AF',
    placeholderColor: '#9CA3AF',
    hoverColor: '#F1F3F5',
    tableHeaderColor: '#F8F9FA'
  },
  Input: {
    borderRadius: '7px',
    heightMedium: '36px',
    color: '#F1F3F5',
    colorFocus: '#F1F3F5',
    border: '1px solid #E5E7EB',
    borderFocus: '1px solid #C9A15C',
    boxShadowFocus: '0 0 0 2px rgba(201,161,92,0.15)',
    textColor: '#1F2937',
    placeholderColor: '#9CA3AF',
    caretColor: '#C9A15C'
  },
  Select: {
    peers: {
      InternalSelection: {
        borderRadius: '7px',
        heightMedium: '36px',
        color: '#F1F3F5',
        border: '1px solid #E5E7EB',
        borderFocus: '1px solid #C9A15C',
        boxShadowFocus: '0 0 0 2px rgba(201,161,92,0.15)',
        textColor: '#1F2937'
      }
    }
  },
  Button: {
    borderRadiusMedium: '7px',
    heightMedium: '36px',
    colorSecondary: '#F1F3F5',
    colorSecondaryHover: '#E5E7EB',
    textColorSecondary: '#1F2937',
    borderSecondary: '1px solid #E5E7EB'
  },
  Switch: {
    railColorActive: '#C9A15C',
    loadingColor: '#C9A15C'
  },
  DataTable: {
    borderColor: '#F3F4F6',
    tdColor: 'transparent',
    thColor: '#F8F9FA',
    thTextColor: '#9CA3AF',
    tdTextColor: '#1F2937',
    borderRadius: '0px'
  },
  Tabs: {
    tabTextColorActiveLine: '#C9A15C',
    tabTextColorHoverLine: '#C9A15C',
    barColor: '#C9A15C'
  },
  Popover: {
    color: '#FFFFFF',
    textColor: '#1F2937',
    borderRadius: '8px'
  },
  Dialog: {
    color: '#FFFFFF',
    textColor: '#1F2937',
    titleTextColor: '#1F2937'
  },
  InputNumber: {
    borderRadius: '7px',
    heightMedium: '36px'
  },
  Tooltip: {
    color: '#FFFFFF',
    textColor: '#1F2937'
  }
}

const appThemeOverrides = computed(() => store.globalConfig.Theme === 'darkTheme' ? darkThemeOverrides : lightThemeOverrides)

const uiLocale = computed(() => {
  locale.value = store.globalConfig.Locale
  if (store.globalConfig.Locale === "zh") {
    return zhCN
  }
  return enUS
})

onMounted(async () => {
  watchEffect(() => {
    if (store.globalConfig.Theme === 'darkTheme') {
      document.documentElement.classList.add('dark')
    } else {
      document.documentElement.classList.remove('dark')
    }
  })

  await store.init()

  eventStore.init()
  eventStore.addHandle({
    type: "message",
    event: (res: appType.Message) => {
      switch (res?.code) {
        case 0:
          window.$message?.error(res.message)
          break
        case 1:
          window.$message?.success(res.message)
          break
      }
    }
  })
})
</script>