<template>
  <NModal
      style="--wails-draggable:no-drag"
      :show="showModal"
      :on-update:show="changeShow"
      display-directive="show"
      :on-after-enter="onAfterEnter"
      :on-after-leave="onAfterLeave"
  >
    <div class="relative flex flex-col overflow-hidden bg-white dark:bg-[#0f172a] shadow-2xl rounded-2xl border border-slate-200 dark:border-slate-800 mx-auto" 
         style="width: fit-content; min-width: 50vw; max-width: 90vw; max-height: 90vh;">
      
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 shrink-0 border-b border-slate-700/50" 
           style="background: linear-gradient(135deg, #1e293b 0%, #334155 100%);">
        <h4 class="text-white font-semibold text-[15px] flex items-center gap-2 m-0 truncate max-w-[80%]">
          <svg class="w-5 h-5 text-blue-400 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M15 10l4.553-2.276A1 1 0 0121 8.618v6.764a1 1 0 01-1.447.894L15 14v-4z"></path><rect x="3" y="6" width="12" height="12" rx="2"></rect></svg>
          {{ t('index.preview') }} - <span class="text-slate-300 font-normal truncate">{{ previewRow?.Description || previewRow?.Url }}</span>
        </h4>
        <button type="button" @click="changeShow(false)" class="text-slate-400 hover:text-white transition-colors bg-transparent border-none cursor-pointer outline-none p-1 shrink-0">
          <svg class="w-6 h-6" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round"><path d="M6 6L18 18M6 18L18 6" /></svg>
        </button>
      </div>
      
      <!-- Body -->
      <div class="flex-grow overflow-auto flex justify-center items-center bg-slate-100 dark:bg-black/40 relative p-4" style="min-height: 200px;">
        <video
            class="video-js vjs-default-skin shadow-lg rounded outline-none"
            ref="videoPlayer"
            controls
            preload="auto"
        ></video>
      </div>
    </div>
  </NModal>
</template>

<script setup lang="ts">
import { ref} from "vue"
import "video.js/dist/video-js.css"
import videojs from "video.js"
import flvjs from "flv.js"
import axios from "axios"
// @ts-ignore
import { getDecryptionArray } from '@/assets/js/decrypt.js'
import type Player from "video.js/dist/types/player"
import {useI18n} from 'vue-i18n'

const {t} = useI18n()
const videoPlayer = ref<HTMLElement | any>(null)
let player: Player | null = null
let flvPlayer: flvjs.Player | null = null
let sourceBuffer: SourceBuffer | null = null
let isLoading = false
let isOver = false
let startByte = 0
const chunkSize = 5 * 1024 * 1024
let endByte = startByte + chunkSize - 1
let decodeArr: any = null
let mediaSource: MediaSource
let rowUrl = ''

const props = defineProps<{
  showModal: boolean
  previewRow: any
}>()
const emits = defineEmits(["update:showModal"])

const changeShow = (value: boolean) => emits("update:showModal", value)

const onAfterEnter = () => {
  if (props.previewRow.DecodeKey) {
    playVideoWithoutTotalLength()
  } else if (props.previewRow.Classify === "live") {
    playFlvStream()
  } else {
    setupVideoJsPlayer()
  }
}

const onAfterLeave = () => {
  if (props.previewRow.Classify === "live" && flvPlayer) {
    flvPlayer.unload()
    flvPlayer.detachMediaElement()
    flvPlayer.destroy()
    flvPlayer = null
  } else if (player) {
    player.pause()
  }
  if (startByte){
    videoPlayer.value?.pause()
    videoPlayer.value?.removeEventListener("seeking", handleSeeking)
    videoPlayer.value?.removeEventListener("timeupdate", handleTimeupdate)
  }
}

const playFlvStream = () => {
  try {
    if (!flvjs.isSupported() || !videoPlayer.value) return

    flvPlayer = flvjs.createPlayer({ type: "flv", url: window?.$baseUrl + "/api/preview?url=" + encodeURIComponent(props.previewRow.Url) })
    flvPlayer.attachMediaElement(videoPlayer.value)
    flvPlayer.load()
    flvPlayer.play()
  }catch (e) {

  }
}

const setupVideoJsPlayer = () => {
  if (!videoPlayer.value) return

  if (!player) {
    player = videojs(videoPlayer.value, {
      controls: true,
      autoplay: false,
      preload: "auto",
    })
  }

  player.src({
    src: window?.$baseUrl + "/api/preview?url=" + encodeURIComponent(props.previewRow.Url),
    type: props.previewRow.ContentType,
    withCredentials: true,
  })
  player.play()
}

const playVideoWithoutTotalLength = () => {
  rowUrl = window?.$baseUrl + "/api/preview?url=" + encodeURIComponent(buildUrlWithParams(props.previewRow.Url))
  mediaSource = new MediaSource()
  videoPlayer.value.src = URL.createObjectURL(mediaSource)
  videoPlayer.value.play()
  isOver = false
  startByte = 0
  endByte = startByte + chunkSize - 1
  decodeArr = getDecryptionArray(props.previewRow.DecodeKey)
  sourceBuffer = null
  mediaSource.addEventListener("sourceopen", () => {
    sourceBuffer = mediaSource.addSourceBuffer('video/mp4; codecs="avc1.42E01E, mp4a.40.2"')
    downloadChunk()
  })

  videoPlayer.value.addEventListener("seeking", handleSeeking)
  videoPlayer.value.addEventListener("timeupdate", handleTimeupdate)
}

const buildUrlWithParams = (url: string) => {
  const parsedUrl = new URL(url)
  const queryParams = parsedUrl.searchParams
  if (queryParams.has("encfilekey") && queryParams.has("token")) {
    return `${parsedUrl.origin}${parsedUrl.pathname}?encfilekey=${queryParams.get("encfilekey")}&token=${queryParams.get("token")}`
  }
  return url
}

const handleSeeking = () => {
  const currentTime = videoPlayer.value.currentTime
  const bufferedEnd = videoPlayer.value.buffered.end(videoPlayer.value.buffered.length - 1)

  if (currentTime > bufferedEnd && !isLoading && !isOver) {
    downloadChunk()
  }
}

const handleTimeupdate = () => {
  if (videoPlayer.value.buffered.length > 0) {
    const bufferedEnd = videoPlayer.value.buffered.end(videoPlayer.value.buffered.length - 1);
    const timeToEnd = bufferedEnd - videoPlayer.value.currentTime;

    // 如果剩余播放时间不足10秒，加载更多数据
    if (timeToEnd < 10 && !isLoading && !isOver) {
      downloadChunk()
    }
  }
}

const downloadChunk = () => {
  if (sourceBuffer?.updating) return;

  isLoading = true
  try {
    axios.get(rowUrl, { headers: { Range: `bytes=${startByte}-${endByte}` }, responseType: "arraybuffer" })
        .then(response => {
          let chunk = new Uint8Array(response.data)

          // 解密前 13702 字节
          for (let i = 0; i < chunk.byteLength && startByte + i < decodeArr.length; i++) {
            chunk[i] ^= decodeArr[startByte + i]
          }

          // 更新字节范围，准备请求下一个分片
          startByte = endByte + 1
          endByte = startByte + chunkSize - 1

          if (sourceBuffer && !sourceBuffer.updating) {
            sourceBuffer.appendBuffer(chunk);
          } else {
            console.error("SourceBuffer is updating, cannot append buffer right now.");
          }
          isLoading = false
          if (response.data.byteLength === 0) {
            isOver = true
            mediaSource?.endOfStream()
          }
        })
        .catch(() => {
          isLoading = false
          isOver = true
        })
}catch (e) {
    isLoading = false
    isOver = true
  }
}

</script>

<style scoped>
:deep(.video-js) {
  width: auto !important;
  height: auto !important;
  max-width: 100% !important;
  max-height: 75vh !important;
  background-color: transparent !important;
  border-radius: 12px !important;
  overflow: hidden !important;
}
:deep(.vjs-tech) {
  position: static !important;
  width: auto !important;
  height: auto !important;
  max-width: 100% !important;
  max-height: 75vh !important;
  border-radius: 12px !important;
  overflow: hidden !important;
}
:deep(.vjs-poster) {
  background-size: contain !important;
}
</style>