<script setup lang="ts">
// 歌单视图：列表 <-> 详情（歌曲表格 + 添加/移除）
import { computed, onMounted, ref, watch } from 'vue'
import {
  NButton, NIcon, NInput, NList, NListItem, NModal, NSpace, NThing, useMessage,
} from 'naive-ui'
import {
  AddOutline, ChevronBackOutline, MusicalNotesOutline, TrashOutline, CloseOutline,
} from '@vicons/ionicons5'
import SongTable from '../components/SongTable.vue'
import { api } from '../api'
import { store } from '../store'
import type { Playlist, Song } from '../types'

const message = useMessage()

// ---- 列表态 ----
const playlists = ref<Playlist[]>([])
async function loadPlaylists() {
  playlists.value = await api.getPlaylists() || []
}
onMounted(loadPlaylists)

// 扫描完成后歌单可能新增歌曲，刷新数量
watch(() => store.scanVersion, loadPlaylists)

// ---- 新建 ----
const showNew = ref(false)
const newName = ref('')
async function create() {
  const name = newName.value.trim()
  if (!name) { message.warning('请输入歌单名称'); return }
  await api.createPlaylist(name)
  showNew.value = false
  newName.value = ''
  loadPlaylists()
}

async function del(id: number, name: string) {
  await api.deletePlaylist(id)
  message.info(`已删除歌单「${name}」`)
  loadPlaylists()
}

// ---- 详情态 ----
const curId = ref(0)
const curName = ref('')
const detailSongs = ref<Song[]>([])
const detailLoading = ref(false)

async function openDetail(pl: Playlist) {
  curId.value = pl.id
  curName.value = pl.name
  detailLoading.value = true
  detailSongs.value = await api.getPlaylistSongs(pl.id) || []
  detailLoading.value = false
  pl.song_count = detailSongs.value.length
}
function backToList() {
  curId.value = 0
  loadPlaylists()
}

async function removeFrom(song: Song) {
  await api.removeSongFromPlaylist(curId.value, song.id)
  detailSongs.value = detailSongs.value.filter(s => s.id !== song.id)
  message.info('已移出歌单')
}

// ---- 添加歌曲弹窗 ----
const showAdd = ref(false)
const allSongs = ref<Song[]>([])
const added = ref<Set<number>>(new Set())
const addQ = ref('')

async function openAdd() {
  added.value = new Set(detailSongs.value.map(s => s.id))
  addQ.value = ''
  allSongs.value = await api.getSongs() || []
  showAdd.value = true
}
const filteredSongs = computed(() => {
  const q = addQ.value.trim().toLowerCase()
  if (!q) return allSongs.value
  return allSongs.value.filter(s =>
    (s.title + s.artist + s.album).toLowerCase().includes(q))
})
async function pickAdd(song: Song) {
  if (added.value.has(song.id)) return
  await api.addSongToPlaylist(curId.value, song.id)
  added.value.add(song.id)
  detailSongs.value = await api.getPlaylistSongs(curId.value) || []
  message.success(`已添加「${song.title}」`)
}
</script>

<template>
  <div class="page">
    <!-- 列表态 -->
    <template v-if="curId === 0">
      <div class="head">
        <h2 class="page-title">歌单</h2>
        <NButton type="primary" size="small" @click="showNew = true">
          <template #icon><NIcon :component="AddOutline" /></template>
          新建歌单
        </NButton>
      </div>
      <NList v-if="playlists.length" bordered class="pl-list">
        <NListItem v-for="pl in playlists" :key="pl.id" class="pl-item">
          <NThing>
            <template #avatar>
              <div class="pl-avatar"><NIcon :component="MusicalNotesOutline" :size="22" /></div>
            </template>
            <template #header>
              <span class="pl-name" @click="openDetail(pl)">{{ pl.name }}</span>
            </template>
            <template #description>
              <span class="pl-count">{{ pl.song_count ?? 0 }} 首</span>
            </template>
            <template #action>
              <NButton quaternary circle size="small" type="error" @click="del(pl.id, pl.name)">
                <template #icon><NIcon :component="TrashOutline" /></template>
              </NButton>
            </template>
          </NThing>
        </NListItem>
      </NList>
      <div v-else class="empty">还没有歌单，点击右上角「新建歌单」创建</div>
    </template>

    <!-- 详情态 -->
    <template v-else>
      <div class="head">
        <NButton quaternary size="small" @click="backToList">
          <template #icon><NIcon :component="ChevronBackOutline" /></template>
          返回
        </NButton>
        <h2 class="page-title detail-title">{{ curName }} · {{ detailSongs.length }} 首</h2>
        <NSpace>
          <NButton size="small" @click="openAdd">
            <template #icon><NIcon :component="AddOutline" /></template>
            添加歌曲
          </NButton>
        </NSpace>
      </div>
      <SongTable :songs="detailSongs" :loading="detailLoading" action="remove" empty-text="歌单为空，点击「添加歌曲」加入" @remove="removeFrom" />
    </template>
  </div>

  <!-- 新建歌单 -->
  <NModal v-model:show="showNew" preset="dialog" title="新建歌单"
    positive-text="创建" negative-text="取消"
    @positive-click="create" @negative-click="showNew = false">
    <NInput v-model:value="newName" placeholder="歌单名称" @keyup.enter="create" />
  </NModal>

  <!-- 添加歌曲 -->
  <NModal v-model:show="showAdd" preset="card" title="添加歌曲到当前歌单" style="width: 480px" @close="showAdd = false">
    <NInput v-model:value="addQ" placeholder="搜索…" clearable class="add-search" />
    <div class="add-list">
      <div
        v-for="s in filteredSongs"
        :key="s.id"
        class="add-row"
        :class="{ added: added.has(s.id) }"
        @click="pickAdd(s)"
      >
        <span class="add-mark">{{ added.has(s.id) ? '✓' : '+' }}</span>
        <span class="add-title">{{ s.title }}</span>
        <span class="add-artist">{{ s.artist }}</span>
      </div>
      <div v-if="filteredSongs.length === 0" class="empty">曲库为空或没有匹配歌曲</div>
    </div>
    <template #footer>
      <NButton quaternary @click="showAdd = false">
        <template #icon><NIcon :component="CloseOutline" /></template>
        关闭
      </NButton>
    </template>
  </NModal>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  gap: 12px;
}
.head {
  display: flex;
  align-items: center;
  gap: 12px;
}
.page-title {
  margin: 0;
  font-size: 18px;
}
.detail-title {
  flex: 1;
}
.pl-list {
  overflow-y: auto;
  flex: 1;
}
.pl-item {
  cursor: pointer;
}
.pl-avatar {
  width: 44px;
  height: 44px;
  border-radius: 8px;
  background: var(--bg-hover);
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--accent);
}
.pl-name {
  cursor: pointer;
  font-weight: 600;
}
.pl-name:hover {
  color: var(--accent);
}
.pl-count {
  color: var(--fg-dim);
  font-size: 12px;
}
.empty {
  color: var(--fg-dim);
  text-align: center;
  padding: 40px 0;
}
.add-search {
  margin-bottom: 10px;
}
.add-list {
  max-height: 420px;
  overflow-y: auto;
}
.add-row {
  display: flex;
  gap: 10px;
  align-items: center;
  padding: 8px 10px;
  border-radius: 6px;
  cursor: pointer;
}
.add-row:hover {
  background: var(--bg-hover);
}
.add-row.added {
  opacity: 0.45;
  cursor: default;
}
.add-mark {
  color: var(--accent);
  width: 16px;
  text-align: center;
}
.add-title {
  flex: 1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.add-artist {
  color: var(--fg-dim);
  font-size: 12px;
}
</style>