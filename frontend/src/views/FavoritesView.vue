<script setup lang="ts">
// 收藏视图：收藏歌曲列表
import { onMounted, ref } from 'vue'
import SongTable from '../components/SongTable.vue'
import { api } from '../api'
import type { Song } from '../types'

const songs = ref<Song[]>([])
const loading = ref(false)

async function load() {
  loading.value = true
  songs.value = await api.getFavorites() || []
  loading.value = false
}

onMounted(load)
</script>

<template>
  <div class="page">
    <h2 class="page-title">收藏</h2>
    <SongTable :songs="songs" :loading="loading" action="favorite" empty-text="还没有收藏歌曲，点击歌曲行的 ♥ 收藏" @favorited="load" />
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