<script setup lang="ts">
import { NConfigProvider, darkTheme, zhCN, dateZhCN, NMessageProvider, NDialogProvider } from 'naive-ui'
import { computed, onMounted, onUnmounted } from 'vue'
import AppSide from './components/AppSide.vue'
import AppEvents from './components/AppEvents.vue'
import PlayerBar from './components/PlayerBar.vue'
import LibraryView from './views/LibraryView.vue'
import PlaylistsView from './views/PlaylistsView.vue'
import FavoritesView from './views/FavoritesView.vue'
import HistoryView from './views/HistoryView.vue'
import GenView from './views/GenView.vue'
import LyricsPanel from './components/LyricsPanel.vue'
import { api } from './api'
import { store, updatePlayingId } from './store'

const themeOverrides = {
  common: {
    primaryColor: '#4cc38a',
    primaryColorHover: '#5fd39c',
    primaryColorPressed: '#3ab07a',
    primaryColorSuppl: '#4cc38a',
    borderRadius: '8px',
    fontFamily: "'Segoe UI', 'Microsoft YaHei', system-ui, sans-serif",
  },
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
  <NConfigProvider :theme="darkTheme" :locale="zhCN" :date-locale="dateZhCN" :theme-overrides="themeOverrides">
    <NMessageProvider>
      <NDialogProvider>
        <div class="app">
          <AppEvents />
          <AppSide class="app-side" />
          <main class="app-main">
            <component :is="viewComponent" class="main-content" />
          </main>
          <PlayerBar class="app-bar" />
          <LyricsPanel v-if="store.lyricsOpen" />
        </div>
      </NDialogProvider>
    </NMessageProvider>
  </NConfigProvider>
</template>