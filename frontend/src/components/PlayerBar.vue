<script setup lang="ts">
// 底部播放条：封面 / 曲目信息 / 进度 / 控制按钮 / 音量 / 收藏 / 歌词 / 加歌单
import { computed, ref, watch } from 'vue'
import {
  NButton, NIcon, NSlider, NTooltip, NSelect, NInput, NModal, useMessage,
  type SelectOption,
} from 'naive-ui'
import {
  PlayBackOutline, PlayForwardOutline, PlayOutline, PauseOutline,
  RepeatOutline, Repeat, ShuffleOutline, ListOutline,
  HeartOutline, Heart, VolumeHighOutline, VolumeMuteOutline,
  MusicalNotesOutline, AddOutline, DiscOutline, MusicalNotes, ImagesOutline,
} from '@vicons/ionicons5'
import { api } from '../api'
import { store } from '../store'
import { formatSec, MODE_NAMES, type Playlist } from '../types'

const message = useMessage()

// ---- 进度条（本地拖动值优先）----
const seekDrag = ref<number | null>(null)
const seekValue = computed(() =>
  seekDrag.value ?? store.status?.position ?? 0)
const maxPos = computed(() => store.status?.duration || 1)

function onSeekUpdate(v: number | null) {
  if (v === null) {
    if (seekDrag.value !== null) api.seek(seekDrag.value)
    seekDrag.value = null
    return
  }
  seekDrag.value = v
}

const positionText = computed(() => formatSec(seekValue.value))
const durationText = computed(() => formatSec(maxPos.value))

// ---- 播放按钮 ----
const isPlaying = computed(() => store.status?.state === 'playing')
const playBtnIcon = computed(() => isPlaying.value ? PauseOutline : PlayOutline)
function togglePlay() {
  if (isPlaying.value) {
    api.pause()
  } else {
    api.resume()
  }
}

// ---- 模式 ----
const modeIcon = computed(() => {
  switch (store.status?.mode) {
    case 0: return ListOutline
    case 1: return RepeatOutline
    case 2: return Repeat
    default: return ShuffleOutline
  }
})
function cycleMode() {
  const next = ((store.status?.mode ?? 1) + 1) % 4
  api.setMode(next)
}

// ---- 音量 ----
const volIcon = computed(() =>
  (store.status?.volume ?? 0) <= 0 ? VolumeMuteOutline : VolumeHighOutline)
function onVolumeUpdate(v: number | null) {
  if (v !== null) api.setVolume(v)
}

// ---- 封面 ----
const coverSrc = ref('')
watch(() => store.status?.current?.id, async (id) => {
  coverSrc.value = ''
  if (!id) return
  const c = await api.getCover(id)
  if (c) coverSrc.value = c
}, { immediate: true })

// ---- 收藏当前歌曲 ----
const fav = computed(() => store.status?.current?.favorited ?? false)
async function toggleFav() {
  const cur = store.status?.current
  if (!cur) return
  const f = await api.toggleFavorite(cur.id).catch(() => null)
  if (f === null) { message.error('收藏失败'); return }
  cur.favorited = f
  message.success(f ? '已收藏 ♥' : '已取消收藏')
}

// ---- 加入歌单 ----
const showAddModal = ref(false)
const playlists = ref<Playlist[]>([])
const plOptions = computed<SelectOption[]>(() =>
  playlists.value.map(p => ({ label: `${p.name}（${p.song_count ?? 0}首）`, value: p.id })))
const newPlName = ref('')
const selectedPl = ref<number | null>(null)

async function openAddModal() {
  playlists.value = await api.getPlaylists() || []
  newPlName.value = ''
  selectedPl.value = null
  showAddModal.value = true
}
async function confirmAdd() {
  const cur = store.status?.current
  if (!cur) return
  try {
    if (newPlName.value.trim()) {
      const id = await api.createPlaylist(newPlName.value.trim())
      await api.addSongToPlaylist(id, cur.id)
      message.success('已新建歌单并加入')
    } else if (selectedPl.value) {
      await api.addSongToPlaylist(selectedPl.value, cur.id)
      message.success('已加入歌单')
    } else {
      message.warning('请选择歌单或输入新歌单名')
      return
    }
    showAddModal.value = false
  } catch (err: any) {
    message.error('操作失败：' + (err?.message || err))
  }
}

// ---- 歌词开关 ----
const lyricsBtn = computed(() => store.lyricsOpen ? DiscOutline : MusicalNotesOutline)
</script>

<template>
  <!-- 进度行 -->
  <div class="progress-row">
    <span class="time">{{ positionText }}</span>
    <NSlider
      :value="seekValue"
      :max="maxPos"
      :step="0.5"
      :tooltip="false"
      @update:value="onSeekUpdate"
    />
    <span class="time">{{ durationText }}</span>
  </div>

  <!-- 控制行 -->
  <div class="control-row">
    <div class="cover-wrap">
      <img v-if="coverSrc" class="cover-thumb" :src="coverSrc" :alt="store.status?.current?.title || ''" />
      <div v-else class="cover-thumb cover-fallback">
        <NIcon :component="MusicalNotes" :size="24" />
      </div>
    </div>

    <div class="meta">
      <div class="meta-title">{{ store.status?.current?.title || '未在播放' }}</div>
      <div class="meta-sub">
        {{ store.status?.current?.artist }}{{ store.status?.current?.album ? ' · ' + store.status?.current?.album : '' }}
      </div>
    </div>

    <div class="ctl">
      <NTooltip>
        <template #trigger>
          <NButton quaternary circle :title="store.status?.mode_text" @click="cycleMode">
            <template #icon><NIcon :component="modeIcon" :size="20" /></template>
          </NButton>
        </template>
        {{ store.status?.mode_text || MODE_NAMES[1] }}
      </NTooltip>
      <NButton quaternary circle @click="api.prev()">
        <template #icon><NIcon :component="PlayBackOutline" :size="22" /></template>
      </NButton>
      <NButton circle type="primary" size="large" class="play-btn" @click="togglePlay">
        <template #icon><NIcon :component="playBtnIcon" :size="22" /></template>
      </NButton>
      <NButton quaternary circle @click="api.next()">
        <template #icon><NIcon :component="PlayForwardOutline" :size="22" /></template>
      </NButton>
      <NTooltip>
        <template #trigger>
          <NButton quaternary circle @click="store.lyricsOpen = !store.lyricsOpen">
            <template #icon><NIcon :component="lyricsBtn" :size="20" /></template>
          </NButton>
        </template>
        {{ store.lyricsOpen ? '关闭歌词' : '显示歌词' }}
      </NTooltip>
    </div>

    <div class="vol">
      <NIcon :component="volIcon" :size="20" />
      <NSlider
        class="vol-slider"
        :value="store.status?.volume ?? 100"
        :max="100"
        :tooltip="false"
        @update:value="onVolumeUpdate"
      />
    </div>

    <div class="my">
      <NButton quaternary circle :title="fav ? '取消收藏' : '收藏'" @click="toggleFav">
        <template #icon>
          <NIcon :component="fav ? Heart : HeartOutline" :size="20"
            :style="fav ? { color: '#e6b450' } : undefined" />
        </template>
      </NButton>
      <NButton quaternary circle title="加入歌单" :disabled="!store.status?.current" @click="openAddModal">
        <template #icon><NIcon :component="AddOutline" :size="20" /></template>
      </NButton>
      <NButton quaternary circle title="切换背景" @click="store.backgroundPickerOpen = true">
        <template #icon><NIcon :component="ImagesOutline" :size="20" /></template>
      </NButton>
    </div>
  </div>

  <!-- 加入歌单弹窗 -->
  <NModal v-model:show="showAddModal" preset="card" title="加入歌单" style="width: 380px">
    <NSelect v-model:value="selectedPl" :options="plOptions" placeholder="选择已有歌单…" clearable />
    <div class="or">或新建歌单</div>
    <NInput v-model:value="newPlName" placeholder="新歌单名称" @keyup.enter="confirmAdd" />
    <template #footer>
      <NButton type="primary" @click="confirmAdd">加入</NButton>
    </template>
  </NModal>
</template>

<style scoped>
.progress-row {
  display: flex;
  align-items: center;
  gap: 12px;
}
.progress-row .time {
  font-size: 12px;
  color: var(--fg-dim);
  min-width: 38px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.control-row {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-top: 2px;
}
.cover-thumb {
  width: 52px;
  height: 52px;
  border-radius: 10px;
  object-fit: cover;
  flex-shrink: 0;
  border: 2px solid #fff;
  box-shadow: 0 3px 12px rgba(255, 140, 180, 0.35);
}
.meta {
  min-width: 0;
  flex: 0 1 auto;
}
.meta-title {
  font-weight: 600;
  font-size: 14px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
}
.meta-sub {
  font-size: 12px;
  color: var(--fg-dim);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  max-width: 220px;
}
.ctl {
  margin: 0 auto;
  display: flex;
  align-items: center;
  gap: 4px;
}
.play-btn {
  width: 46px;
  height: 46px;
  background: var(--accent-grad) !important;
  border: none !important;
  color: #fff !important;
  box-shadow: 0 4px 14px rgba(255, 111, 165, 0.35);
}
.play-btn:hover {
  filter: brightness(1.05);
}
.vol {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--fg-dim);
}
.vol-slider {
  width: 96px;
}
.my {
  display: flex;
  gap: 2px;
}
.or {
  color: var(--fg-dim);
  font-size: 12px;
  margin: 12px 0 4px;
}
</style>