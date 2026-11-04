<script setup lang="ts">
// 曲库视图：搜索 + 导入/扫描 + 歌曲表格
import { onMounted, ref, watch } from 'vue'
import { NButton, NIcon, NInput, NProgress, NSpace, useMessage } from 'naive-ui'
import { FolderOpenOutline, RefreshOutline, SearchOutline } from '@vicons/ionicons5'
import SongTable from '../components/SongTable.vue'
import { api } from '../api'
import { store } from '../store'
import type { Song } from '../types'

const message = useMessage()
const songs = ref<Song[]>([])
const loading = ref(false)
const q = ref('')
const importing = ref(false)

let timer: number | undefined

async function load(qv = '') {
  loading.value = true
  songs.value = await api.getSongs(qv) || []
  loading.value = false
}

function onSearch(v: string) {
  if (timer) window.clearTimeout(timer)
  timer = window.setTimeout(() => load(v), 250)
}

async function doImport() {
  importing.value = true
  const dir = await api.importFolder().catch(() => null)
  importing.value = false
  if (dir) message.success(`已导入：${dir}`)
  else if (api.mode === 'http') message.info('请在桌面端导入音乐文件夹')
}

async function doScan() {
  const r = await api.scan()
  if (r?.ok === false) message.info(r.message || '扫描进行中')
  else message.success('开始扫描…')
}

onMounted(() => load())

// 扫描完成（新增/变更歌曲）后自动重载列表
watch(() => store.scanVersion, () => load(q.value))
</script>

<template>
  <div class="page">
    <div class="toolbar">
      <NInput v-model:value="q" class="search" placeholder="搜索标题 / 艺术家 / 专辑…" clearable @update:value="onSearch">
        <template #prefix><NIcon :component="SearchOutline" /></template>
      </NInput>
      <NSpace>
        <NButton v-if="api.mode === 'wails'" :loading="importing" @click="doImport">
          <template #icon><NIcon :component="FolderOpenOutline" /></template>
          导入文件夹
        </NButton>
        <NButton quaternary @click="doScan">
          <template #icon><NIcon :component="RefreshOutline" /></template>
          刷新曲库
        </NButton>
      </NSpace>
    </div>

    <NProgress
      v-if="store.scanning"
      type="line"
      :percentage="100"
      :show-indicator="false"
      color="#4cc38a"
      class="scan-progress"
    />

    <div class="hint" v-if="store.scanning">正在扫描…</div>

    <SongTable :songs="songs" :loading="loading" action="favorite" />
  </div>
</template>

<style scoped>
.page {
  display: flex;
  flex-direction: column;
  min-height: 0;
  height: 100%;
  gap: 12px;
}
.toolbar {
  display: flex;
  gap: 10px;
  align-items: center;
}
.search {
  max-width: 340px;
}
.scan-progress {
  margin-top: -6px;
}
.hint {
  font-size: 12px;
  color: var(--fg-dim);
}

</style>