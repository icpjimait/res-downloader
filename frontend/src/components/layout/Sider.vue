<template>
  <div class="dark-sidebar" style="--wails-draggable:drag">
    <!-- Brand -->
    <div class="brand">
      <div class="relative">
        <img src="@/assets/image/logo.png" alt="logo" />
        <span v-if="showUpdate" class="absolute -right-1 -top-1 w-2 h-2 bg-red-500 rounded-full" style="animation: pulse-dot 1.6s infinite;"></span>
      </div>
      <div class="brand-text">
        <b>{{ store.appInfo.AppName || 'Res Downloader' }}</b>
        <span>RESOURCE STUDIO</span>
      </div>
    </div>

    <!-- Nav -->
    <div class="nav-group">
      <div
          class="nav-item"
          :class="{ active: currentRoute === 'index' }"
          @click="navigate('index')"
          style="--wails-draggable:no-drag"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M4 6h16M4 12h16M4 18h10"/></svg>
        {{ t('menu.index') }}
      </div>
      <div
          class="nav-item"
          :class="{ active: currentRoute === 'setting' }"
          @click="navigate('setting')"
          style="--wails-draggable:no-drag"
      >
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 00.33 1.82l.06.06a2 2 0 11-2.83 2.83l-.06-.06a1.65 1.65 0 00-1.82-.33 1.65 1.65 0 00-1 1.51V21a2 2 0 01-4 0v-.09A1.65 1.65 0 009 19.4a1.65 1.65 0 00-1.82.33l-.06.06a2 2 0 11-2.83-2.83l.06-.06A1.65 1.65 0 005 15a1.65 1.65 0 00-1.51-1H3.5a2 2 0 010-4h.09A1.65 1.65 0 005 9a1.65 1.65 0 00-.33-1.82l-.06-.06a2 2 0 112.83-2.83l.06.06A1.65 1.65 0 009 5a1.65 1.65 0 001-1.51V3.5a2 2 0 014 0v.09A1.65 1.65 0 0015 5a1.65 1.65 0 001.82-.33l.06-.06a2 2 0 112.83 2.83l-.06.06A1.65 1.65 0 0019 9c.13.36.36.68.66.93"/></svg>
        {{ t('menu.setting') }}
      </div>
    </div>

    <!-- Footer -->
    <div class="sidebar-footer">
      <div class="nav-item" @click="handleAction('github')" style="--wails-draggable:no-drag">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 00-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0020 4.77 5.07 5.07 0 0019.91 1S18.73.65 16 2.48a13.38 13.38 0 00-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 005 4.77a5.44 5.44 0 00-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 009 18.13V22"/></svg>
        github
      </div>

      <div class="nav-item" @click="handleAction('about')" style="--wails-draggable:no-drag">
        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4M12 8h.01"/></svg>
        {{ t('menu.about') }}
      </div>
    </div>
  </div>
  <Footer v-model:showModal="showAppInfo"/>
</template>

<script lang="ts" setup>
import {computed, onMounted, ref, watch} from "vue"
import {useRoute, useRouter} from "vue-router"
import {useIndexStore} from "@/stores"
import Footer from "@/components/Footer.vue"
import {BrowserOpenURL} from "../../../wailsjs/runtime"
import {useI18n} from "vue-i18n"
import request from "@/api/request"
import {compareVersions} from "@/func"

const {t} = useI18n()
const route = useRoute()
const router = useRouter()
const showAppInfo = ref(false)
const store = useIndexStore()
const showUpdate = ref(false)

const currentRoute = computed(() => route.fullPath.substring(1))

watch(() => route.path, () => {})

onMounted(() => {
  request({
    url: 'https://res.putyy.com/version.json?v=' + Date.now(),
    method: 'get',
  }).then((res) => {
    showUpdate.value = compareVersions(res.version, store.appInfo.Version) === 1
  })
})

const navigate = (key: string) => {
  router.push({path: "/" + key})
}

const handleAction = (key: string) => {
  if (key === "about") {
    showAppInfo.value = true
    return
  }
  if (key === "github") {
    BrowserOpenURL("https://github.com/putyy/res-downloader")
    return
  }

}
</script>