<template>
  <div class="h-full flex flex-col overflow-hidden" style="background: var(--bg);" :key="renderKey">
    <!-- Settings Tabs -->
    <div class="settings-tabs" style="--wails-draggable:no-drag">
      <div class="settings-tab" :class="{ active: activeTab === 'basic' }" @click="activeTab = 'basic'">{{ t('setting.basic_setting') }}</div>
      <div class="settings-tab" :class="{ active: activeTab === 'advanced' }" @click="activeTab = 'advanced'">{{ t('setting.advanced_setting') }}</div>
    </div>

    <!-- Content -->
    <div class="flex-1 overflow-y-auto" style="--wails-draggable:no-drag">

      <!-- ══════ BASIC SETTINGS ══════ -->
      <div v-show="activeTab === 'basic'" class="settings-wrap">

        <!-- Storage Card -->
        <div class="dark-card">
          <div class="dark-card-head"><div class="bar"></div><b>{{ t('setting.save_dir') }}</b></div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.save_dir') }}</b>
              <span>{{ t('setting.save_dir') }}</span>
            </div>
            <div class="field-control">
              <NInput :value="formValue.SaveDirectory" :placeholder="t('setting.save_dir')" style="min-width: 260px;" class="mono"/>
              <button class="btn-ghost" @click="selectDir">{{ t('common.select') }}</button>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('menu.locale') }}</b>
              <span>{{ t('menu.locale') }}</span>
            </div>
            <div class="field-control">
              <NSelect v-model:value="formValue.Locale" :options="[{label: '简体中文', value: 'zh'}, {label: 'English', value: 'en'}]" style="min-width: 140px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('menu.theme') }}</b>
              <span>{{ t('menu.theme') }}</span>
            </div>
            <div class="field-control">
              <NSelect v-model:value="formValue.Theme" :options="[{label: 'Dark', value: 'darkTheme'}, {label: 'Light', value: 'lightTheme'}]" style="min-width: 140px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.filename_rules') }}</b>
              <span>{{ t('setting.filename_rules_tip') }}</span>
            </div>
            <div class="field-control">
              <NInputNumber v-model:value="formValue.FilenameLen" :min="0" :max="9999" placeholder="0" style="width: 100px;"/>
              <NSwitch v-model:value="formValue.FilenameTime"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.quality') }}</b>
              <span>{{ t('setting.quality_tip') }}</span>
            </div>
            <div class="field-control">
              <NSelect v-model:value="formValue.Quality" :options="options" style="min-width: 160px;"/>
            </div>
          </div>
        </div>

        <!-- Capture Behavior Card -->
        <div class="dark-card">
          <div class="dark-card-head"><div class="bar"></div><b>{{ t('setting.full_intercept') }}</b></div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.auto_proxy') }}</b>
              <span>{{ t('setting.auto_proxy_tip') }}</span>
            </div>
            <div class="field-control"><NSwitch v-model:value="formValue.AutoProxy"/></div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.full_intercept') }}</b>
              <span>{{ t('setting.full_intercept_tip') }}</span>
            </div>
            <div class="field-control"><NSwitch v-model:value="formValue.WxAction"/></div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.insert_tail') }}</b>
              <span>{{ t('setting.insert_tail_tip') }}</span>
            </div>
            <div class="field-control"><NSwitch v-model:value="formValue.InsertTail"/></div>
          </div>
        </div>

        <!-- Danger Zone -->
        <div class="dark-card" style="display: flex; align-items: center; justify-content: space-between;">
          <div class="field-info">
            <b>{{ t('index.start_err_positiveText') }}</b>
            <span>{{ t('index.reset_app_tip') }}</span>
          </div>
          <n-popconfirm @positive-click="resetHandle">
            <template #trigger>
              <button class="btn-danger-outline">{{ t('index.start_err_positiveText') }}</button>
            </template>
            {{ t('index.reset_app_tip') }}
          </n-popconfirm>
        </div>
      </div>

      <!-- ══════ ADVANCED SETTINGS ══════ -->
      <div v-show="activeTab === 'advanced'" class="settings-wrap">

        <!-- Network Card -->
        <div class="dark-card">
          <div class="dark-card-head"><div class="bar"></div><b>Host / Port</b></div>

          <div class="field-row">
            <div class="field-info">
              <b>Host</b>
              <span>{{ t('setting.restart_tip') }}</span>
            </div>
            <div class="field-control">
              <NInput v-model:value="formValue.Host" placeholder="127.0.0.1" class="mono" style="min-width: 200px;" :status="hostValidationFeedback ? 'error' : undefined"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>Port</b>
              <span>{{ t('setting.restart_tip') }}</span>
            </div>
            <div class="field-control">
              <NInput v-model:value="formValue.Port" placeholder="8899" class="mono" style="min-width: 200px;" :status="portValidationFeedback ? 'error' : undefined"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.upstream_proxy') }}</b>
              <span>{{ t('setting.upstream_proxy_tip') }}</span>
            </div>
            <div class="field-control">
              <NInput v-model:value="formValue.UpstreamProxy" placeholder="http://127.0.0.1:7890" class="mono" style="min-width: 220px;"/>
              <NSwitch v-model:value="formValue.OpenProxy"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.download_proxy') }}</b>
              <span>{{ t('setting.download_proxy_tip') }}</span>
            </div>
            <div class="field-control"><NSwitch v-model:value="formValue.DownloadProxy"/></div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.connections') }}</b>
              <span>{{ t('setting.connections_tip') }}</span>
            </div>
            <div class="field-control">
              <NInputNumber v-model:value="formValue.TaskNumber" :min="2" :max="64" style="width: 100px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.down_number') }}</b>
              <span>{{ t('setting.down_number_tip') }}</span>
            </div>
            <div class="field-control">
              <NInputNumber v-model:value="formValue.DownNumber" :min="1" :max="10" style="width: 100px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>UserAgent</b>
              <span>{{ t('setting.user_agent_tip') }}</span>
            </div>
            <div class="field-control">
              <NInput v-model:value="formValue.UserAgent" placeholder="UserAgent" class="mono" style="min-width: 280px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>Headers</b>
              <span>{{ t('setting.use_headers_tip') }}</span>
            </div>
            <div class="field-control">
              <NInput v-model:value="formValue.UseHeaders" placeholder="default" class="mono" style="min-width: 280px;"/>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.min_image_size') }}</b>
              <span>{{ t('setting.min_image_size_tip') }}</span>
            </div>
            <div class="field-control">
              <NInputNumber v-model:value="formValue.MinImageSize" :min="0" :max="1000000" style="width: 120px;">
                <template #suffix>KB</template>
              </NInputNumber>
            </div>
          </div>

          <div class="field-row">
            <div class="field-info">
              <b>{{ t('setting.min_video_size') }}</b>
              <span>{{ t('setting.min_video_size_tip') }}</span>
            </div>
            <div class="field-control">
              <NInputNumber v-model:value="formValue.MinVideoSize" :min="0" :max="1000000" style="width: 120px;">
                <template #suffix>KB</template>
              </NInputNumber>
            </div>
          </div>
        </div>

        <!-- Domain Rule Card -->
        <div class="dark-card">
          <div class="dark-card-head"><div class="bar"></div><b>{{ t('setting.domain_rule') }}</b></div>
          <NInput
              v-model:value="formValue.Rule"
              type="textarea"
              rows="4"
              :placeholder="t('setting.domain_rule_tip')"
              class="mono"
          />
        </div>

        <!-- Intercept Rules Card -->
        <div class="dark-card">
          <div class="dark-card-head"><div class="bar"></div><b>{{ t('setting.mime_map') }}</b></div>
          <NInput
              v-model:value="MimeMap"
              type="textarea"
              rows="10"
              placeholder='{"video/mp4": { "Type": "video","Suffix": ".mp4"}}'
              class="mono"
          />
        </div>
      </div>
    </div>
  </div>
</template>

<script lang="ts" setup>
import {ref, watch, computed, onMounted, nextTick} from "vue"
import {useIndexStore} from "@/stores"
import type {appType} from "@/types/app"
import appApi from "@/api/app"
import {useI18n} from 'vue-i18n'
import {isValidHost, isValidPort} from '@/func'
import * as bind from "../../wailsjs/go/core/Bind"

const {t} = useI18n()
const store = useIndexStore()

const activeTab = ref('basic')

const options = computed(() => {
  const text = t("setting.quality_value") || ""
  return text
      .split(",")
      .map((value, index) => ({ value: index, label: value }))
})

const formValue = ref<appType.Config>({ ...store.globalConfig })
const MimeMap = ref("")
const renderKey = ref(999)

const hostValidationFeedback = ref("")
const portValidationFeedback = ref("")

let isSyncing = false

const syncFromStore = async () => {
  isSyncing = true
  formValue.value = { ...store.globalConfig }
  if (store.globalConfig?.MimeMap) {
    MimeMap.value = JSON.stringify(store.globalConfig.MimeMap, null, 2)
  }
  await nextTick()
  isSyncing = false
}

onMounted(() => {
  syncFromStore()
})

watch(() => store.globalConfig, () => {
  syncFromStore()
}, { deep: true })

watch(formValue, () => {
  if (isSyncing || !formValue.value) return

  if (typeof formValue.value.Port === 'string') {
    formValue.value.Port = formValue.value.Port.trim()
  }
  if (typeof formValue.value.Host === 'string') {
    formValue.value.Host = formValue.value.Host.trim()
  }

  if (formValue.value.Host && !isValidHost(formValue.value.Host)) {
    hostValidationFeedback.value = t("setting.host_format_error")
    return
  } else {
    hostValidationFeedback.value = ''
  }

  if (formValue.value.Port && !isValidPort(parseInt(formValue.value.Port))) {
    portValidationFeedback.value = t("setting.port_format_error")
    return
  } else {
    portValidationFeedback.value = ''
  }
  store.setConfig(formValue.value)
}, {deep: true})

watch(MimeMap, () => {
  if (isSyncing) return
  try {
    store.setConfig({
      MimeMap: JSON.parse(MimeMap.value)
    })
  } catch (e) {}
})

watch(() => store.globalConfig.Locale, () => {
  formValue.value.Locale = store.globalConfig.Locale
  renderKey.value++
})

const selectDir = () => {
  appApi.openDirectoryDialog().then((res: any) => {
    if (res.code === 1) {
      formValue.value.SaveDirectory = res.data.folder
    }
  }).catch((err: any) => {
    window?.$message?.error(err)
  })
}

const resetHandle = ()=>{
  localStorage.clear()
  bind.ResetApp()
}
</script>