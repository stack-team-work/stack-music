<script setup lang="ts">
// 播放历史视图
import { onMounted, ref } from 'vue'
import SongTable from '../components/SongTable.vue'
import { api } from '../api'
import type { Song } from '../types'

const songs = ref<Song[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  songs.value = await api.getHistory() || []
  loading.value = false
}

onMounted(load)
</script>

<template>
  <div class="page">
    <h2 class="page-title">播放历史</h2>
    <SongTable :songs="songs" :loading="loading" action="favorite" empty-text="还没有播放记录" @favorited="load" />
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
.page-title {
  margin: 0;
  font-size: 18px;
}
</style>