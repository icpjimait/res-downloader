<template>
  <NModal
      :show="showModal"
      :on-update:show="changeShow"
      style="--wails-draggable:no-drag; width: 640px;"
      preset="card"
      content-style="padding: 0; background: var(--bg);"
      footer-style="background: var(--panel); border-top: 1px solid var(--border-soft); padding: 14px 24px; display: flex; justify-content: space-between; align-items: center;"
  >
    <template #header>
      <div class="flex items-center gap-2 text-[17px] font-semibold" style="color: var(--text);">
        <i class="fa fa-info-circle text-[19px]" style="color: var(--accent);"></i> {{ t('footer.title') }}
      </div>
    </template>

    <div class="p-6">
      <div class="border rounded-xl p-6 flex shadow-sm" style="background: var(--panel); border-color: var(--border-soft);">
        <div class="flex-1">
          <div class="flex flex-row items-end mb-2">
            <div class="text-3xl font-bold" style="color: var(--text);">{{ store.appInfo.AppName }}</div>
            <div class="text-xs pl-3 font-medium pb-1" style="color: var(--text-faint);">
              v{{ store.appInfo.Version }}
            </div>
          </div>
          <div class="text-sm leading-relaxed mb-4" style="color: var(--text-dim);">
            {{ t('footer.description') }}
          </div>
          <div class="text-sm font-bold mb-2 mt-4" style="color: var(--accent);">
            {{ t('footer.support') }}
          </div>
          <div class="flex flex-wrap gap-2 text-xs font-medium" style="color: var(--text-dim);">
            <span v-for="item in t('footer.application').split(',')" class="px-2.5 py-1 rounded-md border" style="background: var(--panel-2); border-color: var(--border-soft);">{{ item }}</span>
          </div>
        </div>
        <div class="pl-6 flex items-center justify-center">
          <img src="@/assets/image/logo.png" alt="Logo" class="h-24 w-24 rounded-2xl shadow-sm border" style="border-color: var(--border-soft);"/>
        </div>
      </div>
    </div>

    <template #footer>
      <div class="text-xs font-medium" style="color: var(--text-faint);">
        {{ store.appInfo.Copyright }}
      </div>
      <div class="flex gap-3 text-sm">
        <button class="footer-link-btn" @click="toWebsite('https://s.gowas.cn/d/4089')">{{ t('footer.forum') }}</button>
        <button class="footer-link-btn" @click="toWebsite(certUrl)">{{ t('footer.cert_download') }}</button>
        <button class="footer-link-btn" @click="toWebsite('https://github.com/putyy/res-downloader')">{{ t('footer.source_code') }}</button>
        <button class="footer-link-btn" @click="toWebsite('https://github.com/putyy/res-downloader/issues')">{{ t('footer.help') }}</button>
        <button class="footer-link-btn" @click="toWebsite('https://github.com/putyy/res-downloader/releases')">{{ t('footer.update_log') }}</button>
      </div>
    </template>
  </NModal>
</template>

<style scoped>
.footer-link-btn {
  color: var(--text-dim);
  transition: color 0.2s;
  font-weight: 500;
}
.footer-link-btn:hover {
  color: var(--accent);
}
</style>

<script lang="ts" setup>
import {useIndexStore} from "@/stores"
import {BrowserOpenURL} from "../../wailsjs/runtime"
import {computed} from "vue"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const store = useIndexStore()
const props = defineProps(["showModal"])
const emits = defineEmits(["update:showModal"])
const certUrl = computed(()=>{
  return store.baseUrl + "/api/cert"
})
const changeShow = (value: boolean) => {
  emits('update:showModal', value)
}

const toWebsite = (url: string) => {
  BrowserOpenURL(url)
}
</script>