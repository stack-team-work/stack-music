// 全局响应式状态（播放状态 + 当前视图）。
import { reactive } from 'vue'
import type { Status, Song } from './types'

export type ViewName = 'library' | 'playlists' | 'favorites' | 'history' | 'gen'

export const store = reactive({
  view: 'library' as ViewName,
  status: null as Status | null,
  // 当前"正在浏览"的歌曲列表（单击行的播放队列来源）
  browsing: [] as Song[],
  // 当前视图的歌曲列表（曲库/收藏/历史/歌单详情共用）
  songs: [] as Song[],
  searching: false,
  scanning: false,
  lyricsOpen: false,
  // 播放中歌曲 ID（高亮）
  playingId: -1,
  // 扫描完成次数：视图 watch 它重新加载列表
  scanVersion: 0,
})

// 播放某一行：以当前浏览列表为队列
export async function playAt(list: Song[], index: number) {
  const ids = list.map(s => s.id)
  const start = Math.max(index, 0)
  const api = (await import('./api')).api
  store.status = await api.play({ queue: ids, start })
}

// 播放列表时刷新 playingId
export function updatePlayingId(status: Status | null) {
  store.playingId = status?.current?.id ?? -1
}