<script setup lang="ts">
// 歌词面板：右侧滑出浮层，逐行高亮 + 自动滚动 + 点击跳转。
import { computed, nextTick, ref, watch } from 'vue'
import { NButton, NEmpty, NIcon, NSpin } from 'naive-ui'
import { CloseOutline } from '@vicons/ionicons5'
import { api } from '../api'
import { store } from '../store'
import type { LyricLine } from '../types'

const lines = ref<LyricLine[]>([])
const loading = ref(false)
const currentIdx = ref(-1)
const listEl = ref<HTMLElement | null>(null)

watch(() => store.status?.current?.id, async (id) => {
  currentIdx.value = -1
  lines.value = []
  if (!id) return
  loading.value = true
  lines.value = await api.getLyrics(id) || []
  loading.value = false
}, { immediate: true })

// 进度驱动高亮 + 滚动
watch(() => store.status?.position, async () => {
  const pos = store.status?.position ?? 0
  let idx = -1
  for (let i = 0; i < lines.value.length; i++) {
    if (lines.value[i].time <= pos) idx = i
  }
  if (idx !== currentIdx.value) {
    currentIdx.value = idx
    await nextTick()
    const el = listEl.value?.querySelector(`[data-idx="${Math.max(idx, 0)}"]`)
    el?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  }
})

function jumpTo(line: LyricLine) {
  api.seek(line.time)
}

const title = computed(() => store.status?.current?.title || '歌词')
</script>

<template>
  <div class="lyrics">
    <div class="lyrics-head">
      <span class="lyrics-title">{{ title }}</span>
      <NButton quaternary circle size="small" @click="store.lyricsOpen = false">
        <template #icon><NIcon :component="CloseOutline" /></template>
      </NButton>
    </div>
    <div class="lyrics-body" ref="listEl">
      <NSpin :show="loading">
        <div v-if="!loading && lines.length === 0" class="lyrics-empty">
          <NEmpty description="未找到歌词文件&#10;请将同名 .lrc 文件放在歌曲目录下" />
        </div>
        <div
          v-for="(l, i) in lines"
          :key="i"
          :data-idx="i"
          class="lyric-line"
          :class="{ active: i === currentIdx }"
          @click="jumpTo(l)"
        >
          {{ l.text }}
        </div>
      </NSpin>
    </div>
  </div>
</template>

<style scoped>
.lyrics {
  position: fixed;
  right: 0;
  top: 0;
  bottom: 0;
  width: 340px;
  background: var(--bg-panel);
  border-left: 1px solid var(--border);
  z-index: 50;
  display: flex;
  flex-direction: column;
  animation: slide-in 0.22s ease;
}
@keyframes slide-in {
  from { transform: translateX(40px); opacity: 0; }
  to { transform: translateX(0); opacity: 1; }
}
.lyrics-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 14px;
  border-bottom: 1px solid var(--border);
}
.lyrics-title {
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.lyrics-body {
  flex: 1;
  overflow-y: auto;
  padding: 18px 20px;
  text-align: center;
  font-size: 14px;
}
.lyrics-empty {
  display: flex;
  justify-content: center;
  margin-top: 60px;
}
</style>