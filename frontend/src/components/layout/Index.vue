<template>
<div class="w-full h-full" style="background: var(--bg);">
  <div class="h-full overflow-hidden flex">
      <Sider></Sider>
      <div class="flex-1 flex flex-col min-w-0 h-full">
        <RouterView v-slot="{ Component, route }">
          <keep-alive v-if="route.meta.keepAlive">
            <component :is="Component"></component>
          </keep-alive>
            <component v-else :is="Component"></component>
        </RouterView>
      </div>
  </div>
</div>
</template>
<script lang="ts" setup>
import Sider from "./Sider.vue"
</script>
