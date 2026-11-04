<script setup lang="ts">
import { NIcon } from 'naive-ui'
import {
  LibraryOutline, MusicalNotesOutline, HeartOutline, TimeOutline, SparklesOutline,
} from '@vicons/ionicons5'
import { store, type ViewName } from '../store'

const items: { key: ViewName; label: string; icon: any }[] = [
  { key: 'library', label: '曲库', icon: LibraryOutline },
  { key: 'gen', label: 'AI 音乐', icon: SparklesOutline },
  { key: 'playlists', label: '歌单', icon: MusicalNotesOutline },
  { key: 'favorites', label: '收藏', icon: HeartOutline },
  { key: 'history', label: '播放历史', icon: TimeOutline },
]
</script>

<template>
  <div class="brand">
    <div class="brand-mark">♪</div>
    <div class="brand-text">
      <div class="brand-name">Stack Music</div>
      <div class="brand-sub">Music Player</div>
    </div>
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

  <div class="side-footer">v1.2.1 · Go + Wails</div>
</template>

<style scoped>
.brand {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: 2px 4px 14px;
  padding: 12px;
  border-radius: 14px;
  background: linear-gradient(135deg, rgba(255, 214, 232, 0.75), rgba(214, 237, 255, 0.75));
  border: 1px solid rgba(255, 255, 255, 0.8);
  box-shadow: 0 6px 18px rgba(255, 150, 190, 0.18);
}
.brand-mark {
  width: 38px;
  height: 38px;
  border-radius: 12px;
  background: var(--accent-grad);
  color: #fff;
  font-size: 22px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(255, 111, 165, 0.35);
  flex-shrink: 0;
}
.brand-text {
  min-width: 0;
}
.brand-name {
  font-size: 16px;
  font-weight: 800;
  letter-spacing: 0.5px;
  background: var(--accent-grad);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
  white-space: nowrap;
}
.brand-sub {
  font-size: 10px;
  color: var(--fg-dim);
  letter-spacing: 0.4px;
  white-space: nowrap;
}
.nav {
  display: flex;
  flex-direction: column;
  gap: 3px;
}
.nav-item {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
  border: none;
  border-radius: 10px;
  background: transparent;
  color: var(--fg-dim);
  font-size: 14px;
  cursor: pointer;
  text-align: left;
  transition: background 0.15s, color 0.15s;
}
.nav-item:hover {
  background: rgba(255, 228, 240, 0.6);
  color: var(--fg);
}
.nav-item.active {
  background: var(--accent-grad);
  color: #fff;
  font-weight: 600;
  box-shadow: 0 4px 14px rgba(255, 111, 165, 0.28);
}
.nav-item.active::before {
  content: '';
  position: absolute;
  left: -10px;
  top: 50%;
  transform: translateY(-50%);
  width: 3px;
  height: 18px;
  border-radius: 2px;
  background: var(--accent);
}
.side-footer {
  margin-top: auto; /* 固定在侧栏最底部 */
  padding: 10px 10px 4px;
  border-top: 1px dashed var(--border-strong);
  font-size: 12px;
  color: #b9afd1;
}
</style>
