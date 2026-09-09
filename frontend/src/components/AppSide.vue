<script setup lang="ts">
import { NIcon } from 'naive-ui'
import {
  LibraryOutline, MusicalNotesOutline, HeartOutline, TimeOutline,
} from '@vicons/ionicons5'
import { store, type ViewName } from '../store'

const items: { key: ViewName; label: string; icon: any }[] = [
  { key: 'library', label: '曲库', icon: LibraryOutline },
  { key: 'playlists', label: '歌单', icon: MusicalNotesOutline },
  { key: 'favorites', label: '收藏', icon: HeartOutline },
  { key: 'history', label: '播放历史', icon: TimeOutline },
]
</script>

<template>
  <div class="logo">
    <span class="logo-dot">♪</span>
    <span>堆栈音乐</span>
    <span class="logo-mode" v-if="false" />
  </div>
  <div class="nav">
    <button
      v-for="it in items"
      :key="it.key"
      class="nav-item"
      :class="{ active: store.view === it.key }"
      @click="store.view = it.key"
    >
      <NIcon :component="it.icon" :size="18" />
      <span>{{ it.label }}</span>
    </button>
  </div>
  <div class="side-footer">v1.0.2 · Go + Wails</div>
</template>

<style scoped>
.logo {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 17px;
  font-weight: 700;
  padding: 6px 10px 16px;
}
.logo-dot {
  color: var(--accent);
  font-size: 20px;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 2px;
}
.nav-item {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 9px 12px;
  border: none;
  border-radius: 8px;
  background: transparent;
  color: var(--fg-dim);
  font-size: 14px;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s, color 0.15s;
}
.nav-item:hover {
  background: var(--bg-hover);
  color: var(--fg);
}
.nav-item.active {
  background: var(--accent-weak);
  color: var(--accent);
  font-weight: 600;
}
.side-footer {
  margin-top: auto;
  padding: 8px 10px;
  font-size: 12px;
  color: #4a5364;
}
</style>