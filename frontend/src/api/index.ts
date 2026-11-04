// 双模式 API 层：
//   - Wails 桌面窗口：直接调用 Go 绑定（window.go.bindings.Backend.*），事件走 window.runtime
//   - 浏览器远程控制：fetch /api/* + SSE
// 两端接口签名一致，业务组件无感知。

import type {
  BackgroundItem, Backgrounds, GenConfig, GenRequest, GenTask, LibraryInfo, LyricLine,
  Playlist, PlayRequest, PlayerEvent, ScanDone, ScanProgress, Song, Status,
} from '../types'

const w = window as any

export type Mode = 'wails' | 'http'

export function detectMode(): Mode {
  return w.go && w.go.bindings && w.go.bindings.Backend ? 'wails' : 'http'
}

export const mode: Mode = detectMode()

// ---------- Wails 实现 ----------

function wailsCall(method: string, ...args: any[]): Promise<any> {
  const fn = w.go?.bindings?.Backend?.[method]
  if (!fn) return Promise.reject(new Error(`Go 绑定方法不存在: ${method}`))
  return Promise.resolve(fn(...args))
}

function onWailsEvent(name: string, cb: (data: any) => void): () => void {
  w.runtime?.EventsOn(name, cb)
  return () => w.runtime?.EventsOff(name)
}

// ---------- HTTP 实现 ----------

async function http<T = any>(path: string, method = 'GET', body?: any): Promise<T> {
  const r = await fetch(path, {
    method,
    headers: { 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => null)
    throw new Error(err?.error || `HTTP ${r.status}`)
  }
  return r.json() as Promise<T>
}

// Wails 返回 *Status/[]*SongInfo，HTTP 返回 {status字段}/{songs:[]}，统一形态
function adaptStatus(s: any): Status {
  return s as Status
}

function adaptSongs(res: any): Song[] {
  if (Array.isArray(res)) return res as Song[]
  return (res?.songs || []) as Song[]
}

// ---------- 统一接口 ----------

export const api = {
  mode,

  status: (): Promise<Status> =>
    mode === 'wails' ? wailsCall('Status').then(adaptStatus)
      : http('/api/status').then(adaptStatus),

  getSongs: (q = ''): Promise<Song[]> =>
    mode === 'wails' ? wailsCall('GetSongs', q).then(adaptSongs)
      : http(`/api/songs${q ? '?q=' + encodeURIComponent(q) : ''}`).then(adaptSongs),

  play: (req: PlayRequest): Promise<Status> =>
    mode === 'wails' ? wailsCall('Play', req).then(adaptStatus)
      : http('/api/play', 'POST', req).then(adaptStatus),

  pause: () => mode === 'wails' ? wailsCall('Pause').then(adaptStatus) : http('/api/pause', 'POST').then(adaptStatus),
  resume: () => mode === 'wails' ? wailsCall('Resume').then(adaptStatus) : http('/api/resume', 'POST').then(adaptStatus),
  next: () => mode === 'wails' ? wailsCall('Next').then(adaptStatus) : http('/api/next', 'POST').then(adaptStatus),
  prev: () => mode === 'wails' ? wailsCall('Prev').then(adaptStatus) : http('/api/prev', 'POST').then(adaptStatus),
  seek: (pos: number) => mode === 'wails' ? wailsCall('Seek', pos).then(adaptStatus) : http('/api/seek', 'POST', { position: pos }).then(adaptStatus),
  setVolume: (v: number) => mode === 'wails' ? wailsCall('SetVolume', v).then(adaptStatus) : http('/api/volume', 'POST', { volume: v }).then(adaptStatus),
  setMode: (m: number) => mode === 'wails' ? wailsCall('SetMode', m).then(adaptStatus) : http('/api/mode', 'POST', { mode: m }).then(adaptStatus),

  getPlaylists: (): Promise<Playlist[]> =>
    mode === 'wails' ? wailsCall('GetPlaylists') : http('/api/playlists').then(r => r.playlists),
  createPlaylist: (name: string): Promise<number> =>
    mode === 'wails' ? wailsCall('CreatePlaylist', name) : http('/api/playlists', 'POST', { name }).then(r => r.id),
  deletePlaylist: (id: number): Promise<void> =>
    mode === 'wails' ? wailsCall('DeletePlaylist', id) : http(`/api/playlists/${id}`, 'DELETE'),
  getPlaylistSongs: (id: number): Promise<Song[]> =>
    mode === 'wails' ? wailsCall('GetPlaylistSongs', id).then(adaptSongs) : http(`/api/playlists/${id}/songs`).then(adaptSongs),
  addSongToPlaylist: (pid: number, sid: number): Promise<void> =>
    mode === 'wails' ? wailsCall('AddSongToPlaylist', pid, sid) : http(`/api/playlists/${pid}/songs`, 'POST', { song_id: sid }),
  removeSongFromPlaylist: (pid: number, sid: number): Promise<void> =>
    mode === 'wails' ? wailsCall('RemoveSongFromPlaylist', pid, sid) : http(`/api/playlists/${pid}/songs/${sid}`, 'DELETE'),

  getFavorites: (): Promise<Song[]> =>
    mode === 'wails' ? wailsCall('GetFavorites').then(adaptSongs) : http('/api/favorites').then(adaptSongs),
  toggleFavorite: (sid: number): Promise<boolean> =>
    mode === 'wails' ? wailsCall('ToggleFavorite', sid) : http('/api/favorite', 'POST', { song_id: sid }).then(r => r.favorited),
  getHistory: (): Promise<Song[]> =>
    mode === 'wails' ? wailsCall('GetHistory').then(adaptSongs) : http('/api/history').then(adaptSongs),

  getLyrics: (sid: number): Promise<LyricLine[]> =>
    mode === 'wails' ? wailsCall('GetLyrics', sid)
      : http(`/api/lyrics/${sid}`).then((r: any) => r.lines || []).catch(() => []),

  // Wails 返回 base64 dataURL；HTTP 返回相对 URL（组件直接当 img src）
  getCover: (sid: number): Promise<string | null> =>
    mode === 'wails' ? wailsCall('GetCover', sid).then((d: string) => d || null) : Promise.resolve(`/api/covers/${sid}`),

  library: (): Promise<LibraryInfo> =>
    mode === 'wails' ? wailsCall('Library') : http('/api/library'),

  scan: (): Promise<any> =>
    mode === 'wails' ? wailsCall('Scan') : http('/api/scan', 'POST'),

  importFolder: (): Promise<string | null> =>
    mode === 'wails' ? wailsCall('ImportFolder') : Promise.resolve(null),

  // 事件订阅。HTTP 模式 SSE 载荷为状态快照，统一转成 Kind=1 (状态变化)。
  onPlayerEvent: (cb: (e: PlayerEvent & { _status?: Status }) => void): (() => void) => {
    if (mode === 'wails') return onWailsEvent('player:event', cb)
    const src = new EventSource('/api/events')
    src.onmessage = (ev) => {
      try {
        const st = JSON.parse(ev.data)
        cb({ Kind: 1, _status: st })
      } catch { /* ignore */ }
    }
    return () => src.close()
  },

  onScanProgress: (cb: (p: ScanProgress) => void): (() => void) => {
    if (mode === 'wails') return onWailsEvent('scan:progress', cb)
    return () => {} // HTTP 模式由 status.scanning + 轮询体现
  },

  onScanDone: (cb: (d: ScanDone) => void): (() => void) => {
    if (mode === 'wails') return onWailsEvent('scan:done', cb)
    return () => {}
  },

  // ---------- AI 音乐生成 ----------

  getGenConfig: (): Promise<GenConfig> =>
    mode === 'wails' ? wailsCall('GetGenConfig') : http('/api/gen/config'),

  setGenConfig: (cfg: Partial<GenConfig>): Promise<void> =>
    mode === 'wails' ? wailsCall('SetGenConfig', cfg) : http('/api/gen/config', 'POST', cfg),

  genSubmit: (req: GenRequest): Promise<GenTask> =>
    mode === 'wails' ? wailsCall('GenSubmit', req) : http('/api/gen/submit', 'POST', req),

  genTasks: (): Promise<GenTask[]> =>
    mode === 'wails' ? wailsCall('GenTasks') : http('/api/gen/tasks').then((r: any) => r.tasks || []),

  genDeleteTask: (id: number): Promise<void> =>
    mode === 'wails' ? wailsCall('GenDeleteTask', id) : http(`/api/gen/tasks/${id}`, 'DELETE'),

  // 生成任务事件（Wails 推送；HTTP 模式由轮询兜底）
  onGenEvent: (cb: (name: string, task: GenTask) => void): (() => void) => {
    if (mode !== 'wails') return () => {}
    const offs = ['gen:queued', 'gen:done', 'gen:failed'].map(n =>
      onWailsEvent(n, (task: GenTask) => cb(n, task)))
    return () => offs.forEach(off => off())
  },

  // ---------- 界面背景 ----------

  getBackgrounds: (): Promise<Backgrounds> =>
    mode === 'wails' ? wailsCall('GetBackgrounds') : http('/api/backgrounds'),

  setBackground: (id: string): Promise<void> =>
    mode === 'wails' ? wailsCall('SetBackground', id) : http('/api/backgrounds/current', 'POST', { id }),

  importBackground: (): Promise<BackgroundItem | null> =>
    mode === 'wails'
      ? wailsCall('ImportBackground').then((it: BackgroundItem | null) => it || null)
      : Promise.resolve(null), // 浏览器模式由组件上传

  uploadBackground: async (file: File): Promise<BackgroundItem> => {
    const fd = new FormData()
    fd.append('file', file)
    const r = await fetch('/api/backgrounds/upload', { method: 'POST', body: fd })
    if (!r.ok) {
      const err = await r.json().catch(() => null)
      throw new Error(err?.error || `HTTP ${r.status}`)
    }
    const data = await r.json()
    return data.item as BackgroundItem
  },

  deleteBackground: (id: string): Promise<void> =>
    mode === 'wails' ? wailsCall('DeleteBackground', id) : http(`/api/backgrounds/${id}`, 'DELETE'),

  // 背景图片地址：Wails 绑定返回绝对地址，HTTP 模式返回相对地址
  backgroundURL: (item: BackgroundItem): string => item.url || '',
}