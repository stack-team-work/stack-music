// 与 Go 绑定/HTTP API 对齐的 JSON 类型定义。

export interface Song {
  id: number
  path: string
  title: string
  artist: string
  album: string
  genre: string
  year: number
  track: number
  duration: number // 秒
  format: string
  size: number
  mod_time: number
  hash: string
  has_cover: boolean
  favorited?: boolean
}

export type PlayState = 'playing' | 'paused' | 'stopped'

export interface Status {
  state: PlayState
  position: number // 秒
  duration: number // 秒
  volume: number // 0-100
  mode: number // 0 顺序 1 列表循环 2 单曲循环 3 随机
  mode_text: string
  current: Song | null
  queue_pos: number
  queue_len: number
  scanning: boolean
  song_count: number
  library_roots: string[]
}

export interface LyricLine {
  time: number // 秒
  text: string
}

export interface Playlist {
  id: number
  name: string
  created_at: number
  song_count?: number
  song_ids?: number[]
}

export interface PlayRequest {
  song_id?: number
  playlist_id?: number
  queue?: number[]
  start?: number
}

// Wails player:event 载荷（Go player.Event 序列化）
export interface PlayerEvent {
  Kind: number // 0 切歌 1 状态变化 2 错误
  Song?: Song | null
  State?: number // 0 stopped 1 playing 2 paused
  Message?: string
}

export interface ScanDone {
  ok: boolean
  added: number
  updated: number
  removed: number
}

export interface ScanProgress {
  done: number
  total: number
}

export interface LibraryInfo {
  roots: string[]
  song_count: number
  scanning: boolean
}

export function formatSec(sec: number): string {
  if (!isFinite(sec) || sec < 0) sec = 0
  const m = Math.floor(sec / 60)
  const s = Math.floor(sec % 60)
  return `${m}:${String(s).padStart(2, '0')}`
}

export const MODE_NAMES = ['顺序播放', '列表循环', '单曲循环', '随机播放']
// ---------- AI 音乐生成 ----------

export type GenMode = 'text' | 'lyrics' | 'instrumental'
export type GenProvider = 'minimax' | 'suno' | 'local'

export interface GenRequest {
  mode: GenMode
  prompt: string
  lyrics: string
  style: string
  mood: string
  lang: string
  title: string
}

export interface GenTask {
  id: number
  mode: GenMode
  prompt: string
  lyrics: string
  style: string
  mood: string
  lang: string
  title: string
  provider: string
  model: string
  remote_task_id: string
  status: 'pending' | 'queued' | 'generating' | 'done' | 'failed'
  audio_path: string
  song_id: number
  fail_reason: string
  created_at: number
  updated_at: number
}

export interface GenConfig {
  provider: GenProvider
  base_url: string
  api_key: string
  model: string
  defaults: Record<string, { base_url: string; model: string }>
}

export const GEN_STYLES = [
  'Pop', 'Hip Hop', 'Rock', 'Electronic', 'Acoustic',
  'R&B', 'Jazz', 'Classical', 'Country', 'Lofi',
] as const

export const GEN_MOODS = ['欢快', '伤感', '激昂', '平静', '浪漫', '神秘'] as const
export const GEN_LANGS = ['中文', '英文', '日文', '韩文', '粤语', '纯音乐'] as const
