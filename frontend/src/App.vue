<script setup lang="ts">
import {
  NConfigProvider, lightTheme, zhCN, dateZhCN, NMessageProvider, NDialogProvider,
} from 'naive-ui'
import { computed, onMounted, onUnmounted } from 'vue'
import AppSide from './components/AppSide.vue'
import AppEvents from './components/AppEvents.vue'
import PlayerBar from './components/PlayerBar.vue'
import LibraryView from './views/LibraryView.vue'
import PlaylistsView from './views/PlaylistsView.vue'
import FavoritesView from './views/FavoritesView.vue'
import HistoryView from './views/HistoryView.vue'
import GenView from './views/GenView.vue'
import BackgroundPicker from './components/BackgroundPicker.vue'
import LyricsPanel from './components/LyricsPanel.vue'
import { api } from './api'
import { store, updatePlayingId } from './store'
import { BUILTIN_BACKGROUNDS, type Backgrounds } from './types'

// 二次元樱粉 × 天蓝主题
const themeOverrides = {
  common: {
    primaryColor: '#ff6fa5',
    primaryColorHover: '#ff8fbe',
    primaryColorPressed: '#e85d93',
    primaryColorSuppl: '#ff8fbe',
    borderRadius: '10px',
    fontFamily: "'Segoe UI', 'Microsoft YaHei', system-ui, sans-serif",
    textColorBase: '#2f2a3d',
    textColor1: '#2f2a3d',
    textColor2: '#4a4460',
    textColor3: '#8a83a3',
    borderColor: '#f1e3f6',
    dividerColor: '#f1e3f6',
  },
}

// 背景样式：内置 CSS 渐变 或 用户图片
const bgStyle = computed(() => {
  const bg = store.background
  if (bg.id.startsWith('builtin:')) {
    const b = BUILTIN_BACKGROUNDS.find(x => x.id === bg.id) || BUILTIN_BACKGROUNDS[0]
    return { backgroundImage: b.css }
  }
  return bg.url ? { backgroundImage: `url("${bg.url}")` } : {}
})

async function loadBackground() {
  try {
    const res: Backgrounds = await api.getBackgrounds()
    const cur = res.current || 'builtin:sakura'
    const item = (res.items || []).find(i => i.id === cur)
    store.background = { id: cur, url: item ? api.backgroundURL(item) : '' }
  } catch {
    store.background = { id: 'builtin:sakura', url: '' }
  }
}

// 状态轮询（双模式统一）：桌面端调用本机绑定开销极小，远程端本身就是轮询。
let timer: number | undefined
let offEvent: (() => void) | undefined

async function refreshStatus() {
  try {
    const st = await api.status()
    if (st && st !== store.status) {
      store.status = st
      store.scanning = st.scanning
      updatePlayingId(st)
    }
  } catch { /* 网络瞬断时忽略 */ }
}

onMounted(() => {
  refreshStatus()
  loadBackground()
  timer = window.setInterval(refreshStatus, 500)
  // 播放器事件（Wails：Go 推送；HTTP：SSE）→ 立即刷新，减少轮询延迟
  offEvent = api.onPlayerEvent(() => refreshStatus())
})

onUnmounted(() => {
  if (timer) window.clearInterval(timer)
  offEvent?.()
})

const viewComponent = computed(() => {
  switch (store.view) {
    case 'playlists': return PlaylistsView
    case 'favorites': return FavoritesView
    case 'history': return HistoryView
    case 'gen': return GenView
    default: return LibraryView
  }
})
</script>

<template>
  <NConfigProvider :theme="lightTheme" :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NDialogProvider>
        <div class="app-bg" :style="bgStyle" />
        <div class="app-bg-mask" />
        <div class="app">
          <AppEvents />
          <aside class="app-side">
            <AppSide />
          </aside>
          <main class="app-main">
            <component :is="viewComponent" class="main-content" />
          </main>
          <footer class="app-bar">
            <PlayerBar />
          </footer>
          <LyricsPanel v-if="store.lyricsOpen" />
          <BackgroundPicker v-model:show="store.backgroundPickerOpen" @changed="loadBackground" />
        </div>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>