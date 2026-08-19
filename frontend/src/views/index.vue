<template>
  <div class="h-full flex flex-col overflow-hidden" style="background: var(--bg);">
    <!-- ══════ TOPBAR ══════ -->
    <div class="flex items-stretch gap-4 px-5 py-3 flex-shrink-0 border-b" style="border-color: var(--border-soft); background: rgba(255,255,255,0.01); --wails-draggable:no-drag;" id="header">
      <!-- Capture pill button -->
      <button
          class="capture-btn-big"
          :class="{ off: !isProxy }"
          @click="isProxy ? close() : open()"
      >
        <span class="capture-dot"></span>
        <span>{{ isProxy ? t('index.close_grab') : t('index.open_grab') }}</span>
      </button>

      <div class="flex flex-col justify-center gap-2">
        <!-- Resource count badge -->
        <div class="flex items-center">
          <span v-if="data.length > 0" class="text-xs px-2 py-0.5 rounded-md" style="background: var(--panel-2); color: var(--text-dim); border: 1px solid var(--border);">
            {{ t('index.total_resources', {count: data.length}) }}
          </span>
        </div>

        <!-- Filter tags -->
        <div class="filter-tags-grid">
          <div
              v-for="item in classify"
              :key="item.value"
              class="filter-tag"
              :class="{ active: selectedTypes.includes(item.value) }"
              @click="toggleType(item.value)"
          >
            {{ item.label }}
          </div>
        </div>
      </div>

      <div class="flex-1"></div>

      <div class="flex items-center gap-3">
        <!-- Search -->
        <div class="icon-btn" :style="descriptionSearchValue || urlSearchValue ? 'color: var(--accent);' : ''" @click="showSearchModal = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><circle cx="11" cy="11" r="8"/><path d="M21 21l-4.35-4.35"/></svg>
        </div>

        <!-- Clear -->
        <button v-if="!rememberChoice" class="btn-ghost danger" @click="showClearModal = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 6h18M8 6V4a1 1 0 011-1h6a1 1 0 011 1v2m3 0l-1 14a2 2 0 01-2 2H7a2 2 0 01-2-2L4 6"/></svg>
          {{ t('index.clear_list') }}
        </button>
        <button v-else class="btn-ghost danger" @click="clear">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><path d="M3 6h18M8 6V4a1 1 0 011-1h6a1 1 0 011 1v2m3 0l-1 14a2 2 0 01-2 2H7a2 2 0 01-2-2L4 6"/></svg>
          {{ t('index.clear_list') }}
        </button>

        <!-- Batch download -->
        <button class="btn-solid" @click="batchDown">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 3v12m0 0l-4-4m4 4l4-4M4 21h16"/></svg>
          {{ t('index.batch_download') }}
        </button>

        <!-- More actions -->
        <div class="icon-btn" @click="showMoreActionsModal = true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8"><rect x="3" y="3" width="7" height="7" rx="1"/><rect x="14" y="3" width="7" height="7" rx="1"/><rect x="3" y="14" width="7" height="7" rx="1"/><rect x="14" y="14" width="7" height="7" rx="1"/></svg>
        </div>
      </div>
    </div>

    <!-- ══════ PULSE STRIP ══════ -->
    <div class="pulse-strip" v-if="isProxy">
      <span class="pulse-label">{{ t('index.open_grab') }}</span>
      <div class="pulse-wave">
        <svg viewBox="0 0 400 20" preserveAspectRatio="none">
          <polyline points="0,10 10,10 15,4 20,16 25,10 40,10 45,7 50,13 55,10 80,10 85,2 90,18 95,10 130,10 135,6 140,14 145,10 200,10 205,10 210,4 215,16 220,10 235,10 240,7 245,13 250,10 280,10 285,2 290,18 295,10 330,10 335,6 340,14 345,10 400,10"
            fill="none" stroke="#4FD1C5" stroke-width="1.4" opacity="0.7"/>
          <polyline points="400,10 410,10 415,4 420,16 425,10 440,10 445,7 450,13 455,10 480,10 485,2 490,18 495,10 530,10 535,6 540,14 545,10 600,10 605,10 610,4 615,16 620,10 635,10 640,7 645,13 650,10 680,10 685,2 690,18 695,10 730,10 735,6 740,14 745,10 800,10"
            fill="none" stroke="#4FD1C5" stroke-width="1.4" opacity="0.7"/>
        </svg>
      </div>
      <span class="pulse-count">{{ pulseCount }} / s</span>
    </div>

    <!-- ══════ DATA TABLE ══════ -->
    <div class="flex-1 min-h-0 overflow-hidden">
      <NDataTable
          :columns="columns"
          :data="filteredData"
          :bordered="false"
          :max-height="tableHeight"
          :row-key="rowKey"
          :virtual-scroll="true"
          :header-height="42"
          :height-for-row="()=> 48"
          :checked-row-keys="checkedRowKeysValue"
          @update:checked-row-keys="handleCheck"
          style="--wails-draggable:no-drag"
      />
    </div>

    <!-- ══════ FOOTER LINKS ══════ -->
    <div class="footer-links" id="bottom">
      <a @click="BrowserOpenURL(certUrl)">{{ t('footer.cert_download') }}</a>
      <a @click="BrowserOpenURL('https://github.com/putyy/res-downloader')">{{ t('footer.source_code') }}</a>
      <a @click="BrowserOpenURL('https://github.com/putyy/res-downloader/issues')">{{ t('footer.help') }}</a>
      <a @click="BrowserOpenURL('https://github.com/putyy/res-downloader/releases')">{{ t('footer.update_log') }}</a>
    </div>

    <Preview v-model:showModal="showPreviewRow" :previewRow="previewRow" @download="handlePreviewDownload"/>
    <ShowLoading :loadingText="loadingText" :isLoading="loading"/>
    <ImportJson v-model:showModal="showImport" @submit="handleImport"/>
    <Password v-model:showModal="showPassword" @submit="handlePassword"/>

    <!-- Search Modal -->
    <NModal v-model:show="showSearchModal" preset="card" style="width: 400px; --wails-draggable:no-drag; background: var(--bg);" :title="t('index.search')">
      <div class="flex flex-col gap-4">
        <NInput v-model:value="descriptionSearchValue" :placeholder="t('index.search_description')" clearable />
        <NInput v-model:value="urlSearchValue" placeholder="URL / Domain" clearable />
      </div>
    </NModal>

    <!-- Clear Modal -->
    <NModal v-model:show="showClearModal" preset="card" style="width: 400px; --wails-draggable:no-drag; background: var(--bg);" :title="t('index.clear_list')">
      <div>
        <p class="text-[15px] mb-4" style="color: var(--text);">{{ t("index.clear_list_tip") }}</p>
        <NCheckbox v-model:checked="rememberChoiceTmp">
          <span style="color: var(--text-dim);">{{ t('index.remember_clear_choice') }}</span>
        </NCheckbox>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3" style="background: var(--panel);">
          <button class="btn-ghost" @click="showClearModal = false">{{ t('common.no') }}</button>
          <button class="btn-danger" @click="()=>{rememberChoice=rememberChoiceTmp;clear();showClearModal=false}">{{ t('common.yes') }}</button>
        </div>
      </template>
    </NModal>

    <!-- More Actions Modal -->
    <NModal v-model:show="showMoreActionsModal" preset="card" style="width: 320px; --wails-draggable:no-drag; background: var(--bg);" :title="t('index.more_operation')">
      <div class="flex flex-col gap-2">
        <button class="btn-ghost w-full justify-start text-[15px] py-2" @click="batchCancel(); showMoreActionsModal = false;">
          {{ t('index.cancel_down') }}
        </button>
        <button class="btn-ghost w-full justify-start text-[15px] py-2" @click="batchExport(); showMoreActionsModal = false;">
          {{ t('index.batch_export') }}
        </button>
        <button class="btn-ghost w-full justify-start text-[15px] py-2" @click="showImport=true; showMoreActionsModal = false;">
          {{ t('index.batch_import') }}
        </button>
        <button class="btn-ghost w-full justify-start text-[15px] py-2" @click="batchExport('url'); showMoreActionsModal = false;">
          {{ t('index.export_url') }}
        </button>
      </div>
    </NModal>
  </div>
</template>

<script lang="ts" setup>
import {NButton, NIcon, NImage, NInput, NSpace, NTooltip, NPopover, NGradientText} from "naive-ui"
import {computed, h, onMounted, onUnmounted, ref, watch} from "vue"
import type {appType} from "@/types/app"
import type {DataTableRowKey, ImageRenderToolbarProps, DataTableFilterState, DataTableBaseColumn} from "naive-ui"
import Preview from "@/components/Preview.vue"
import ShowLoading from "@/components/ShowLoading.vue"
// @ts-ignore
import {getDecryptionArray} from '@/assets/js/decrypt.js'
import {useIndexStore} from "@/stores"
import appApi from "@/api/app"
import Action from "@/components/Action.vue"
import ActionDesc from "@/components/ActionDesc.vue"
import ImportJson from "@/components/ImportJson.vue"
import {useEventStore} from "@/stores/event"
import {BrowserOpenURL, ClipboardSetText, EventsOn} from "../../wailsjs/runtime"
import Password from "@/components/Password.vue"
import ShowOrEdit from "@/components/ShowOrEdit.vue"
import {useI18n} from 'vue-i18n'
import {useDialog} from 'naive-ui'
import * as bind from "../../wailsjs/go/core/Bind"
import {Quit} from "../../wailsjs/runtime"
import {DialogOptions} from "naive-ui/es/dialog/src/DialogProvider"
import {formatSize} from "@/func"

const {t} = useI18n()
const eventStore = useEventStore()
const dialog = useDialog()
const isProxy = computed(() => {
  return store.isProxy
})
const certUrl = computed(() => {
  return store.baseUrl + "/api/cert"
})
const data = ref<any[]>([])
const filteredData = computed(() => {
  let result = data.value

  if (selectedTypes.value.length > 0 && !selectedTypes.value.includes('all')) {
    result = result.filter(item => selectedTypes.value.includes(item.Classify))
  }

  if (descriptionSearchValue.value) {
    result = result.filter(item => item.Description?.toLowerCase().includes(descriptionSearchValue.value.toLowerCase()))
  }

  if (urlSearchValue.value) {
    result = result.filter(item => item.Url?.toLowerCase().includes(urlSearchValue.value.toLowerCase()) || item.Domain?.toLowerCase().includes(urlSearchValue.value.toLowerCase()))
  }

  return result
})

const store = useIndexStore()
const tableHeight = ref(800)

// ── Filter tags state ──
const selectedTypes = ref<string[]>(['all'])

const classifyAlias: { [key: string]: any } = {
  image: computed(() => t("index.image")),
  audio: computed(() => t("index.audio")),
  video: computed(() => t("index.video")),
  m3u8: computed(() => t("index.m3u8")),
  live: computed(() => t("index.live")),
  xls: computed(() => t("index.xls")),
  doc: computed(() => t("index.doc")),
  pdf: computed(() => t("index.pdf")),
  stream: computed(() => t("index.stream")),
  font: computed(() => t("index.font"))
}

const dwStatus = computed<any>(() => {
  return {
    ready: t("index.ready"),
    pending: t("index.pending"),
    running: t("index.running"),
    error: t("index.error"),
    done: t("index.done"),
    handle: t("index.handle")
  }
})

const maxConcurrentDownloads = computed(() => {
  return store.globalConfig.DownNumber
})

const classify = ref([
  {
    value: "all",
    label: computed(() => t("index.all")),
  },
])

const descriptionSearchValue = ref("")
const urlSearchValue = ref("")
const rememberChoice = ref(false)
const rememberChoiceTmp = ref(false)

// ── Pulse counter ──
const pulseCount = ref(0)
let pulseBuffer = 0
let pulseTimer: ReturnType<typeof setInterval> | null = null

// ── Table columns (render functions use dark theme classes) ──
const columns = ref<any[]>([
  {
    type: "selection",
  },
  {
    title: () => {
      if (checkedRowKeysValue.value.length > 0) {
        return h('span', {style: 'color: var(--accent); font-weight: 600;'}, t("index.choice") + `(${checkedRowKeysValue.value.length})`)
      }
      return t('index.domain')
    },
    key: "Domain",
    width: 130,
    minWidth: 80,
    maxWidth: 500,
    resizable: true,
    render: (row: appType.MediaInfo) => {
      return h(NTooltip, {
        trigger: 'hover',
        placement: 'top'
      }, {
        trigger: () => h('span', {
          style: 'color: var(--text-dim); cursor: default; display: block; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;'
        }, row.Domain),
        default: () => h('span', {class: 'mono', style: 'font-size: 11px; word-break: break-all;'}, row.Url)
      })
    }
  },
  {
    title: computed(() => t("index.type")),
    key: "Classify",
    width: 80,
    render: (row: appType.MediaInfo) => {
      const label = classifyAlias[row.Classify]?.value ?? row.Classify
      return h('span', {class: `type-pill ${row.Classify}`}, label)
    }
  },
  {
    title: computed(() => t("index.preview")),
    key: "Url",
    width: 70,
    render: (row: appType.MediaInfo) => {
      if (row.Classify === "image") {
        return h("div", {
          style: "width: 100%;max-height:80px;overflow:hidden;"
        }, h(NImage, {
          objectFit: "contain",
          lazy: true,
          "render-toolbar": renderToolbar,
          src: row.Url
        }))
      }
      if (row.Classify === "audio" || row.Classify === "video" || row.Classify === "m3u8" || row.Classify === "live") {
        return h('div', {
          style: 'cursor: pointer; display: flex; align-items: center; gap: 4px; color: var(--teal); font-size: 12px;',
          onClick: () => {
            if (row.UrlSign && downloadHistory.value[row.UrlSign]) {
              row.Status = "done"
              row.SavePath = downloadHistory.value[row.UrlSign]
            }
            previewRow.value = row
            showPreviewRow.value = true
          }
        }, [
          h('svg', {
            viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '1.8',
            style: 'width: 14px; height: 14px;'
          }, [h('polygon', {points: '5 3 19 12 5 21 5 3'})]),
          t("index.preview")
        ])
      }
      return h('span', {style: 'color: var(--text-faint); font-size: 11px;'}, '—')
    }
  },
  {
    title: computed(() => t("index.status")),
    key: "Status",
    width: 90,
    render: (row: appType.MediaInfo, index: number) => {
      const statusClass = row.Status || 'ready'
      const label = row.Status === "running" ? row.SavePath : dwStatus.value[row.Status as keyof typeof dwStatus]
      return h('div', {
        class: `status-cell ${statusClass}`,
        onClick: () => {
          if (row.SavePath && row.Status === "done") {
            appApi.openFolder({filePath: row.SavePath})
          } else if (row.Status === "ready") {
            download(row, index)
          }
        }
      }, [
        h('span', {class: 'status-dot'}),
        h('span', {style: 'white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 100px;'}, label)
      ])
    }
  },
  {
    title: computed(() => t("index.description")),
    key: "Description",
    width: 180,
    minWidth: 100,
    maxWidth: 800,
    resizable: true,
    render: (row: appType.MediaInfo, index: number) => {
      return h(ShowOrEdit, {
        value: row.Description,
        onUpdateValue(v: string) {
          data.value[index].Description = v
          cacheData()
        }
      })
    }
  },
  {
    title: computed(() => t("index.resource_size")),
    key: "Size",
    width: 100,
    sorter: (row1: appType.MediaInfo, row2: appType.MediaInfo) => row1.Size - row2.Size,
    render(row: appType.MediaInfo) {
      return h('span', {class: 'mono', style: 'color: var(--text-dim); font-size: 12px;'}, formatSize(row.Size))
    }
  },
  {
    title: computed(() => t("index.save_path")),
    key: "SavePath",
    minWidth: 150,
    resizable: true,
    render(row: appType.MediaInfo) {
      const pathText = row.Status === "running" ? "" : row.SavePath;
      const pathElement = h("span",
          {
            class: "mono",
            style: row.FileMissing ? 'color: var(--red); font-size: 11.5px; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block;' : "color: var(--text-dim); font-size: 11.5px; max-width: 220px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: block;"
          },
          pathText
      );
      if (pathText) {
        return h(NTooltip, { trigger: 'hover', placement: 'top', interactive: true }, {
          trigger: () => pathElement,
          default: () => h('div', { class: 'flex items-center gap-2' }, [
            h('span', { class: 'mono', style: 'font-size: 11px; word-break: break-all; max-width: 400px; display: block;' }, pathText),
            h('div', {
              class: 'icon-btn',
              style: 'width: 20px; height: 20px; flex-shrink: 0;',
              title: t("common.copy"),
              onClick: () => {
                ClipboardSetText(pathText).then((is: boolean) => {
                  if (is) {
                    window?.$message?.success(t("common.copy_success"))
                  }
                })
              }
            }, [
              h('svg', { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2' }, [
                h('rect', { x: '9', y: '9', width: '13', height: '13', rx: '2', ry: '2' }),
                h('path', { d: 'M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1' })
              ])
            ]),
            h('div', {
              class: 'icon-btn',
              style: 'width: 20px; height: 20px; flex-shrink: 0;',
              title: t("index.open_folder"),
              onClick: () => {
                if (row.SavePath && row.Status === "done") {
                  appApi.openFolder({filePath: row.SavePath})
                }
              }
            }, [
              h('svg', { viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor', 'stroke-width': '2' }, [
                h('path', { d: 'M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z' })
              ])
            ])
          ])
        });
      }
      return pathElement;
    }
  },
  {
    key: "actions",
    width: 130,
    render(row: appType.MediaInfo, index: number) {
      return h('div', {class: 'row-actions'}, [
        h(Action, {key: index, row: row, index: index, onAction: dataAction})
      ])
    },
    title() {
      return h(ActionDesc)
    }
  }
])

const checkedRowKeysValue = ref<DataTableRowKey[]>([])
const showPreviewRow = ref(false)
const previewRow = ref<appType.MediaInfo>()
const loading = ref(false)
const loadingText = ref("")
const showSearchModal = ref(false)
const showClearModal = ref(false)
const showMoreActionsModal = ref(false)
const showImport = ref(false)
const showPassword = ref(false)
const downloadQueue = ref<appType.MediaInfo[]>([])
const downloadHistory = ref<Record<string, string>>({})
let activeDownloads = 0
let isOpenProxy = false
let isInstall = false

// ── Type tag toggle logic ──
const toggleType = (type: string) => {
  if (type === 'all') {
    selectedTypes.value = ['all']
    appApi.setType(['all'])
    return
  }
  // Remove 'all' if selecting specific type
  let current = selectedTypes.value.filter(t => t !== 'all')
  if (current.includes(type)) {
    current = current.filter(t => t !== type)
  } else {
    current.push(type)
  }
  if (current.length === 0) {
    current = ['all']
  }
  selectedTypes.value = current
  appApi.setType(current)
}

onMounted(() => {
  EventsOn("second-instance-launched", () => {
    window?.$message?.info(t("common.already_running"), {duration: 4000})
  })

  const historyStr = localStorage.getItem("downloadHistory")
  if (historyStr) {
    try { downloadHistory.value = JSON.parse(historyStr) } catch (e) {}
  }

  const mediaData = localStorage.getItem("mediaData")
  if (mediaData) {
    data.value = JSON.parse(mediaData)
    data.value.forEach((item) => {
      if (item.Status === "done" && item.SavePath) {
        if (bind && typeof bind.FileExists === 'function') {
          bind.FileExists(item.SavePath).then((exists: boolean) => {
            item.FileMissing = !exists
          }).catch(() => {})
        }
      }
    })
  }

  try {
    window.addEventListener("resize", () => {
      resetTableHeight()
    })
    loading.value = true
    handleInstall().then((is: boolean) => {
      isInstall = true
      loading.value = false
    })

    checkLoading()
    watch(showPassword, () => {
      if (!showPassword.value) {
        checkLoading()
      }
    })
  } catch (e) {
    window.$message?.error(JSON.stringify(e), {duration: 5000})
  }

  buildClassify()

  const temp = localStorage.getItem("resources-type")
  if (temp) {
    selectedTypes.value = JSON.parse(temp).res
  } else {
    appApi.setType(selectedTypes.value)
  }

  // The user requested not to load previously cached resources on startup
  localStorage.removeItem("resources-data")

  const choiceCache = localStorage.getItem("remember-clear-choice")
  if (choiceCache === "1") {
    rememberChoice.value = true
  }

  watch(rememberChoice, () => {
    if (rememberChoice.value) {
      localStorage.setItem("remember-clear-choice", "1")
    } else {
      localStorage.removeItem("remember-clear-choice")
    }
  })

  watch(selectedTypes, () => {
    localStorage.setItem("resources-type", JSON.stringify({res: selectedTypes.value}))
    appApi.setType(selectedTypes.value)
  })

  resetTableHeight()

  // Pulse counter
  pulseTimer = setInterval(() => {
    pulseCount.value = pulseBuffer
    pulseBuffer = 0
  }, 1000)

  eventStore.addHandle({
    type: "newResources",
    event: (res: any) => {
      if (!isProxy.value) return
      pulseBuffer++
      if (downloadHistory.value[res.UrlSign]) {
        res.Status = "done"
        res.SavePath = downloadHistory.value[res.UrlSign]
        if (bind && typeof bind.FileExists === 'function') {
          bind.FileExists(res.SavePath).then((exists: boolean) => {
            res.FileMissing = !exists
          }).catch(() => {})
        }
      }

      let isExist = false
      data.value.forEach((item) => {
        if (item.UrlSign === res.UrlSign) {
          isExist = true
        }
      })
      if (isExist) return

      if (store.globalConfig.InsertTail) {
        data.value.push(res)
      } else {
        data.value.unshift(res)
      }
      cacheData()
    }
  })

  eventStore.addHandle({
    type: "downloadProgress",
    event: (res: { Id: string, SavePath: string, Status: string, Message: string }) => {
      switch (res.Status) {
        case "running":
          updateItem(res.Id, item => {
            item.SavePath = res.Message
            item.Status = 'running'
          })
          break
        case "done":
          updateItem(res.Id, item => {
            item.SavePath = res.SavePath
            item.Status = 'done'
            item.FileMissing = false
            if (item.UrlSign && res.SavePath) {
              downloadHistory.value[item.UrlSign] = res.SavePath
              localStorage.setItem("downloadHistory", JSON.stringify(downloadHistory.value))
            }
          })
          if (activeDownloads > 0) {
            activeDownloads--
          }
          cacheData()
          checkQueue()
          break
        case "error":
          updateItem(res.Id, item => {
            item.SavePath = res.Message
            item.Status = 'error'
          })
          if (activeDownloads > 0) {
            activeDownloads--
          }
          cacheData()
          checkQueue()
          break
      }
    }
  })
})

onUnmounted(() => {
  if (pulseTimer) clearInterval(pulseTimer)
})

watch(() => {
  return store.globalConfig.MimeMap
}, () => {
  buildClassify()
})

const updateItem = (id: string, updater: (item: any) => void) => {
  const item = data.value.find(i => i.Id === id)
  if (item) updater(item)
  if (previewRow.value && previewRow.value.Id === id) {
    updater(previewRow.value)
  }
}

function cacheData() {
  localStorage.setItem("resources-data", JSON.stringify(data.value))
}

const resetTableHeight = () => {
  try {
    const headerHeight = document.getElementById("header")?.offsetHeight || 0
    const bottomHeight = document.getElementById("bottom")?.offsetHeight || 0
    const pulseHeight = isProxy.value ? 34 : 0
    const height = document.documentElement.clientHeight || window.innerHeight
    tableHeight.value = height - headerHeight - bottomHeight - pulseHeight - 42 - 10
  } catch (e) {
    console.log(e)
  }
}

const buildClassify = () => {
  const mimeMap = store.globalConfig.MimeMap ?? {}
  const seen = new Set()
  classify.value = [
    {value: "all", label: computed(() => t("index.all"))},
    ...Object.values(mimeMap)
        .filter(({Type}) => {
          if (seen.has(Type)) return false
          seen.add(Type)
          return true
        })
        .map(({Type}) => ({
          value: Type,
          label: classifyAlias[Type] ?? Type,
        })),
  ]
}

const dataAction = (row: appType.MediaInfo, index: number, type: string) => {
  switch (type) {
    case "down":
      download(row, index)
      break
    case "cancel":
      if (row.Status === "pending") {
        const queueIndex = downloadQueue.value.findIndex(item => item.Id === row.Id)
        if (queueIndex !== -1) {
          downloadQueue.value.splice(queueIndex, 1)
        }
        updateItem(row.Id, item => {
          item.Status = 'ready'
          item.SavePath = ''
        })
        cacheData()
      } else if (row.Status === "running") {
        appApi.cancel({id: row.Id}).then((res) => {
          updateItem(row.Id, item => {
            item.Status = 'ready'
            item.SavePath = ''
          })
          if (activeDownloads > 0) {
            activeDownloads--
          }
          cacheData()
          checkQueue()
          if (res.code === 0) {
            window?.$message?.error(res.message)
            return
          }
        })
      }
      break
    case "copy":
      ClipboardSetText(row.Url).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "json":
      ClipboardSetText(encodeURIComponent(JSON.stringify(row))).then((is: boolean) => {
        if (is) {
          window?.$message?.success(t("common.copy_success"))
        } else {
          window?.$message?.error(t("common.copy_fail"))
        }
      })
      break
    case "open":
      BrowserOpenURL(row.Url)
      break
    case "decode":
      decodeWxFile(row, index)
      break
    case "delete":
      if (row.Status === "pending" || row.Status === "running") {
        window?.$message?.error(t("index.delete_tip"))
        return
      }
      appApi.delete({sign: [row.UrlSign]}).then(() => {
        const realIndex = data.value.findIndex(item => item.Id === row.Id)
        if (realIndex !== -1) {
          data.value.splice(realIndex, 1)
          cacheData()
        }
      })
      break
  }
}

const renderToolbar = ({nodes}: ImageRenderToolbarProps) => {
  return [
    nodes.rotateCounterclockwise,
    nodes.rotateClockwise,
    nodes.resizeToOriginalSize,
    nodes.zoomOut,
    nodes.zoomIn,
    nodes.close
  ]
}

const rowKey = (row: appType.MediaInfo) => {
  return row.Id
}

const handleCheck = (rowKeys: DataTableRowKey[]) => {
  checkedRowKeysValue.value = rowKeys
}

const batchDown = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  data.value.forEach((item, index) => {
    if (checkedRowKeysValue.value.includes(item.Id) && item.Classify !== 'live' && item.Classify !== 'm3u8') {
      download(item, index)
    }
  })
  checkedRowKeysValue.value = []
}

const batchCancel = async () => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  loading.value = true
  const cancelTasks: Promise<any>[] = []
  data.value.forEach((item) => {
    if (!checkedRowKeysValue.value.includes(item.Id)) {
      return
    }
    if (item.Status === "pending") {
      const queueIndex = downloadQueue.value.findIndex(qItem => qItem.Id === item.Id)
      if (queueIndex !== -1) {
        downloadQueue.value.splice(queueIndex, 1)
      }
      item.Status = 'ready'
      item.SavePath = ''
      return
    }
    if (item.Status === "running") {
      if (activeDownloads > 0) {
        activeDownloads--
      }
      cancelTasks.push(appApi.cancel({id: item.Id}).then(() => {
        item.Status = 'ready'
        item.SavePath = ''
        checkQueue()
      }))
    }
  })
  await Promise.allSettled(cancelTasks)
  loading.value = false
  checkedRowKeysValue.value = []
  cacheData()
}

const batchExport = (type?: string) => {
  if (checkedRowKeysValue.value.length <= 0) {
    window?.$message?.error(t("index.use_data"))
    return
  }
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  loadingText.value = t("common.loading")
  loading.value = true

  let jsonData = data.value.filter(item => checkedRowKeysValue.value.includes(item.Id))

  if (type === "url") {
    jsonData = jsonData.map(item => item.Url)
  } else {
    jsonData = jsonData.map(item => encodeURIComponent(JSON.stringify(item)))
  }

  appApi.batchExport({content: jsonData.join("\n")}).then((res: appType.Res) => {
    loading.value = false
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    window?.$message?.success(t("index.import_success"))
    window?.$message?.info(t("index.save_path") + "：" + res.data?.file_name, {
      duration: 5000
    })
  })
}

const uint8ArrayToBase64 = (bytes: any) => {
  return window.btoa(Array.from(bytes, (byte: any) => String.fromCharCode(byte)).join(''))
}

const handlePreviewDownload = (row: appType.MediaInfo) => {
  if (row.Classify === 'live' || row.Classify === 'm3u8') {
    window?.$message?.error(t("index.download_no_tip"))
    return
  }
  const index = data.value.findIndex(item => item.Id === row.Id)
  if (index !== -1) {
    download(data.value[index], index)
  } else {
    download(row, 0)
  }
}

const download = (row: appType.MediaInfo, index: number) => {
  if (!store.globalConfig.SaveDirectory) {
    window?.$message?.error(t("index.save_path_empty"))
    return
  }
  if (data.value.some(item => item.Id === row.Id && item.Status === "running")) {
    return
  }
  if (downloadQueue.value.some(item => item.Id === row.Id || item.Url === row.Url)) {
    return
  }
  if (activeDownloads >= maxConcurrentDownloads.value) {
    row.Status = "pending"
    downloadQueue.value.push(row)
    window?.$message?.info(t("index.download_queued", {count: downloadQueue.value.length}))
    return
  }
  startDownload(row, index)
}

const startDownload = (row: appType.MediaInfo, index: number) => {
  activeDownloads++
  const decodeStr = row.DecodeKey
      ? uint8ArrayToBase64(getDecryptionArray(row.DecodeKey))
      : ""
  appApi.download({...row, decodeStr}).then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
    }
  })
}

const checkQueue = () => {
  if (downloadQueue.value.length > 0 && activeDownloads < maxConcurrentDownloads.value) {
    const nextItem = downloadQueue.value.shift()
    if (nextItem) {
      const index = data.value.findIndex(item => item.Id === nextItem.Id)
      if (index !== -1) {
        startDownload(nextItem, index)
      }
    }
  }
}

const open = () => {
  isOpenProxy = true
  store.openProxy().then((res: appType.Res) => {
    if (res.code === 1) {
      return
    }
    if (["darwin", "linux"].includes(store.envInfo.platform)) {
      showPassword.value = true
    } else {
      window.$message?.error(res.message)
    }
  })
}

const close = () => {
  store.unsetProxy()
}

const clear = async () => {
  const newData = [] as any[]
  const signs: string[] = []
  if (checkedRowKeysValue.value.length > 0) {
    data.value.forEach((item) => {
      if (checkedRowKeysValue.value.includes(item.Id) && item.Status !== "pending" && item.Status !== "running") {
        signs.push(item.UrlSign)
      } else {
        newData.push(item)
      }
    })
    checkedRowKeysValue.value = []
  } else {
    data.value.forEach((item) => {
      if (item.Status === "pending" || item.Status === "running") {
        newData.push(item)
      } else {
        signs.push(item.UrlSign)
      }
    })
  }
  await appApi.delete({sign: signs})
  data.value = newData
  cacheData()
}

const decodeWxFile = (row: appType.MediaInfo, index: number) => {
  if (!row.DecodeKey) {
    window?.$message?.error(t("index.video_decode_no"))
    return
  }
  appApi.openFileDialog().then((res: appType.Res) => {
    if (res.code === 0) {
      window?.$message?.error(res.message)
      return
    }
    if (res.data.file) {
      loadingText.value = t("index.video_decode_loading")
      loading.value = true
      appApi.wxFileDecode({
        ...row,
        filename: res.data.file,
        decodeStr: uint8ArrayToBase64(getDecryptionArray(row.DecodeKey))
      }).then((res: appType.Res) => {
        loading.value = false
        if (res.code === 0) {
          window?.$message?.error(res.message)
          return
        }
        data.value[index].SavePath = res.data.save_path
        data.value[index].Status = "done"
        cacheData()
        window?.$message?.success(t("index.video_decode_success"))
      })
    }
  })
}

const handleImport = (content: string) => {
  if (!content) {
    window?.$message?.error(t("view.import_empty"))
    return
  }
  let newItems = [] as any[]
  content.split("\n").forEach((line) => {
    try {
      let res = JSON.parse(decodeURIComponent(line))
      if (res && res?.Id) {
        res.Id = res.Id + Math.floor(Math.random() * 100000)
        res.SavePath = ""
        res.Status = "ready"
        newItems.push(res)
      }
    } catch (e) {
      console.log(e)
    }
  })
  if (newItems.length > 0) {
    data.value = [...newItems, ...data.value]
    cacheData()
  }
  showImport.value = false
}

const handlePassword = async (password: string, isCache: boolean) => {
  const res = await appApi.setSystemPassword({password, isCache})
  if (res.code === 0) {
    window.$message?.error(res.message)
    return
  }
  if (isOpenProxy) {
    showPassword.value = false
    store.openProxy()
    return
  }
  handleInstall().then((is: boolean) => {
    if (is) {
      showPassword.value = false
    }
  })
}

const handleInstall = async () => {
  isOpenProxy = false
  const res = await appApi.install()
  if (res.code === 1) {
    store.globalConfig.AutoProxy && store.openProxy()
    return true
  }
  window.$message?.error(res.message, {duration: 5000})
  if (store.envInfo.platform === "windows" && res.message.includes("Access is denied")) {
    window.$message?.error(t("index.win_install_tip"))
  } else if (["darwin", "linux"].includes(store.envInfo.platform)) {
    showPassword.value = true
  }
  return false
}

const checkLoading = () => {
  setTimeout(() => {
    if (loading.value && !isInstall && !showPassword.value) {
      dialog.warning({
        title: t("index.start_err_tip"),
        content: t("index.start_err_content"),
        positiveText: t("index.start_err_positiveText"),
        negativeText: t("index.start_err_negativeText"),
        draggable: false,
        closeOnEsc: false,
        closable: false,
        maskClosable: false,
        onPositiveClick: () => {
          bind.ResetApp()
        },
        onNegativeClick: () => {
          Quit()
        }
      } as DialogOptions)
    }
  }, 6000)
}
</script>