<template>
  <!-- 工具按钮组：扁平图标，无独立描边/阴影，hover 时图标 morph 为文案（取代 tooltip）。 -->
  <div class="flex items-center gap-1 mx-3">
    <g-dropdown-menu v-if="isDev" trigger="hover" :items="videoList" @select="(item) => toDoc(item.value)">
      <icon-button icon="lucide:clapperboard" label="教程" />
    </g-dropdown-menu>

    <icon-button v-if="settings.header.search.visible" icon="lucide:search" label="搜索" @click="handleCommand" />

    <icon-button icon="lucide:settings" label="设置" @click="toggleSetting" />

    <icon-button v-if="settings.header.refresh.visible" icon="lucide:refresh-cw" label="刷新"
      :icon-class="showRefreshAnmite ? 'animate-spin' : ''" @click="toggleRefresh" />

    <icon-button :icon="themeStore.isDark ? 'lucide:sun' : 'lucide:moon'" label="主题"
      @click="themeStore.toggleTheme(!themeStore.isDark)" />

    <gva-setting v-model:drawer="showSettingDrawer"></gva-setting>
    <command-menu ref="command" />
  </div>
</template>

<script setup>
import { useThemeStore } from '@/pinia'
import { storeToRefs } from 'pinia'
import GvaSetting from '@/view/layout/setting/index.vue'
import { ref } from 'vue'
import { emitter } from '@/utils/bus.js'
import CommandMenu from '@/components/commandMenu/index.vue'
import IconButton from '@/components/iconButton/index.vue'
import { toDoc } from '@/utils/doc'
import { isDev } from '@/utils/env.js'

const themeStore = useThemeStore()
const { settings } = storeToRefs(themeStore)
const showSettingDrawer = ref(false)
const showRefreshAnmite = ref(false)
const toggleRefresh = () => {
  showRefreshAnmite.value = true
  emitter.emit('reload')
  setTimeout(() => {
    showRefreshAnmite.value = false
  }, 1000)
}

const toggleSetting = () => {
  showSettingDrawer.value = true
}

const first = ref('')
const command = ref()

const handleCommand = () => {
  command.value.open()
}
const initPage = () => {
  // 判断当前用户的操作系统
  if (window.localStorage.getItem('osType') === 'WIN') {
    first.value = 'Ctrl'
  } else {
    first.value = '⌘'
  }
  // 当用户同时按下ctrl和k键的时候
  const handleKeyDown = (e) => {
    if (e.ctrlKey && e.key === 'k') {
      // 阻止浏览器默认事件
      e.preventDefault()
      handleCommand()
    }
  }
  window.addEventListener('keydown', handleKeyDown)
}

initPage()
</script>
