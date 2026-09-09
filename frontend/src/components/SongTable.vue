<script setup lang="ts">
// 歌曲表格：Naive UI NDataTable 封装。
// 单击行播放（以当前列表为队列），当前播放行高亮，行内收藏按钮。
import { computed, h } from 'vue'
import { NButton, NDataTable, NIcon, useMessage, type DataTableColumns } from 'naive-ui'
import { Heart, HeartOutline } from '@vicons/ionicons5'
import type { Song } from '../types'
import { api } from '../api'
import { store, playAt } from '../store'
import { formatSec } from '../types'

const props = defineProps<{
  songs: Song[]
  loading?: boolean
  // 行操作按钮：'favorite' 收藏 | 'remove' 移除（歌单详情用）
  action?: 'favorite' | 'remove'
  emptyText?: string
}>()
const emit = defineEmits<{
  (e: 'remove', song: Song): void
  (e: 'favorited', song: Song, fav: boolean): void
}>()

const message = useMessage()

async function toggleFav(song: Song, ev: MouseEvent) {
  ev.stopPropagation()
  try {
    const fav = await api.toggleFavorite(song.id)
    song.favorited = fav
    message.success(fav ? '已收藏' : '已取消收藏')
    emit('favorited', song, fav)
  } catch (err: any) {
    message.error('操作失败：' + (err?.message || err))
  }
}

const rowClassName = (row: Song) => (row.id === store.playingId ? 'playing-row' : '')

const columns = computed<DataTableColumns<Song>>(() => {
  const cols: DataTableColumns<Song> = [
    {
      title: '#',
      key: 'idx',
      width: 56,
      render: (row, index) => h('span', { class: 'cell-idx' },
        row.id === store.playingId ? '▶' : String(index + 1)),
    },
    {
      title: '标题',
      key: 'title',
      minWidth: 200,
      ellipsis: { tooltip: true },
      render: (row) => h('span', { class: row.id === store.playingId ? 'cell-title playing' : 'cell-title' },
        row.title || '(未知)'),
    },
    {
      title: '艺术家',
      key: 'artist',
      minWidth: 120,
      ellipsis: { tooltip: true },
      render: (row) => row.artist || '—',
    },
    {
      title: '专辑',
      key: 'album',
      minWidth: 120,
      ellipsis: { tooltip: true },
      render: (row) => row.album || '—',
    },
    {
      title: '时长',
      key: 'duration',
      width: 80,
      render: (row) => h('span', { class: 'cell-dur' }, formatSec(row.duration)),
    },
  ]
  if (props.action) {
    cols.push({
      title: '操作',
      key: 'action',
      width: 56,
      render: (row, index) => {
        if (props.action === 'favorite') {
          return h(NButton, {
            quaternary: true, circle: true, size: 'small',
            onClick: (e: MouseEvent) => toggleFav(row, e),
            style: { color: row.favorited ? '#e6b450' : undefined },
          }, { icon: () => h(NIcon, { component: row.favorited ? Heart : HeartOutline }) })
        }
        return h(NButton, {
          quaternary: true, circle: true, size: 'small', type: 'error',
          onClick: (e: MouseEvent) => { e.stopPropagation(); emit('remove', row) },
        }, { icon: () => h(NIcon, { component: HeartOutline, style: { color: '#e8808a' } }) })
      },
    })
  }
  return cols
})

function onRowClick(row: Song) {
  playAt(props.songs, props.songs.indexOf(row))
}
</script>

<template>
  <NDataTable
    :columns="columns"
    :data="songs"
    :loading="loading"
    flex-height
    :row-key="(r: Song) => r.id"
    :row-class-name="rowClassName"
    :row-props="(row: Song) => ({ onClick: () => onRowClick(row) })"
    :bordered="false"
    :single-line="false"
    size="small"
    :empty-text="emptyText || '还没有歌曲，先去曲库导入音乐文件夹吧'"
    scroll-x="auto"
  />
</template>

<style scoped>
.cell-idx {
  color: var(--fg-dim);
  font-size: 12px;
}
.cell-title {
  color: var(--fg);
}
.cell-title.playing {
  color: var(--accent);
}
.cell-dur {
  color: var(--fg-dim);
  font-size: 12px;
}
:deep(.playing-row > td) {
  background: var(--accent-weak) !important;
}
:deep(.n-data-table-tr:hover > td) {
  cursor: pointer;
}
</style>