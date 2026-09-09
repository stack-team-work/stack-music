// Package bindings 是 Wails 暴露给前端 Vue 应用的 Go 绑定层。
// 桌面窗口内前端直接调用这些方法；浏览器远程模式由 internal/server 走 HTTP 同构接口。
package bindings

import (
	"context"
	"log"
	"math"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-music/internal/config"
	"go-music/internal/library"
	"go-music/internal/lyrics"
	"go-music/internal/model"
	"go-music/internal/player"
	"go-music/internal/store"
)

// Status 播放器状态快照（与 server 的 HTTP 状态保持字段一致）。
type Status struct {
	State        string      `json:"state"`
	Position     float64     `json:"position"`
	Duration     float64     `json:"duration"`
	Volume       float64     `json:"volume"`
	Mode         int         `json:"mode"`
	ModeText     string      `json:"mode_text"`
	Current      *model.Song `json:"current"`
	QueuePos     int         `json:"queue_pos"`
	QueueLen     int         `json:"queue_len"`
	Scanning     bool        `json:"scanning"`
	SongCount    int         `json:"song_count"`
	LibraryRoots []string    `json:"library_roots"`
}

// PlayRequest 播放请求（song_id / playlist_id / queue 三选一）。
type PlayRequest struct {
	SongID     int64   `json:"song_id"`
	PlaylistID int64   `json:"playlist_id"`
	Queue      []int64 `json:"queue"`
	Start      int     `json:"start"`
}

// LyricLine 一行歌词（时间为秒）。
type LyricLine struct {
	Time float64 `json:"time"`
	Text string  `json:"text"`
}

// Progress 扫描进度事件载荷。
type Progress struct {
	Done  int `json:"done"`
	Total int `json:"total"`
}

// Backend Wails 绑定实例（全部方法供前端直接调用）。
type Backend struct {
	cfg     *config.Config
	st      *store.Store
	lib     *library.Library
	pl      *player.Player
	dataDir string

	ctx context.Context
	mu  sync.Mutex // 保护扫描等操作

	running bool // 事件转发协程是否已启动
}

// New 构造绑定层。
func New(cfg *config.Config, st *store.Store, lib *library.Library, pl *player.Player, dataDir string) *Backend {
	return &Backend{cfg: cfg, st: st, lib: lib, pl: pl, dataDir: dataDir}
}

// OnStartup Wails 生命周期回调：保存上下文、启动事件转发、记录播放历史。
func (b *Backend) OnStartup(ctx context.Context) {
	b.ctx = ctx
	b.startEventLoop()
	b.pl.OnSongPlayed = func(so *model.Song) {
		if so != nil {
			_ = b.st.AddHistory(so.ID)
		}
	}
	// 扫描进度推送给前端
	b.lib.OnProgress = func(done, total int) {
		runtime.EventsEmit(b.ctx, "scan:progress", &Progress{Done: done, Total: total})
	}
	// 已有曲库目录时启动自动扫描（增量，快速；后台执行不阻塞窗口）
	if len(b.cfg.Libraries) > 0 {
		go b.Scan()
	}
}

// startEventLoop 把播放器事件转发为 Wails 事件。
func (b *Backend) startEventLoop() {
	if b.running {
		return
	}
	b.running = true
	go func() {
		for ev := range b.pl.Events() {
			runtime.EventsEmit(b.ctx, "player:event", &ev)
		}
	}()
}

// --- 播放控制 ---

// status 构建状态快照（带收藏标记）。
func (b *Backend) status() *Status {
	pos, dur := b.pl.Position()
	mode := b.pl.Mode()
	st := b.pl.State()
	s := &Status{
		State:    st.String(),
		Position: pos.Seconds(),
		Duration: dur.Seconds(),
		Volume:   b.pl.Volume(),
		Mode:     int(mode),
		ModeText: mode.String(),
		Current:  b.pl.Current(),
		QueuePos: b.pl.QueuePos(),
		QueueLen: len(b.pl.Queue()),
		Scanning: b.lib.Scanning(),
	}
	if n, err := b.st.SongCount(); err == nil {
		s.SongCount = n
	}
	s.LibraryRoots = append([]string(nil), b.cfg.Libraries...)
	return s
}

// Status 当前播放器状态。
func (b *Backend) Status() *Status { return b.status() }

// Play 播放：song_id 单曲；playlist_id 整单播放；queue 显式队列。
func (b *Backend) Play(req PlayRequest) (*Status, error) {
	switch {
	case req.PlaylistID > 0:
		songs, err := b.st.PlaylistSongs(req.PlaylistID)
		if err != nil {
			return nil, err
		}
		if len(songs) == 0 {
			return nil, errEmptyPlaylist
		}
		start := 0
		if req.SongID > 0 {
			for i, so := range songs {
				if so.ID == req.SongID {
					start = i
					break
				}
			}
		}
		b.pl.PlayQueue(songs, start)
	case len(req.Queue) > 0:
		songs := make([]*model.Song, 0, len(req.Queue))
		for _, id := range req.Queue {
			if so, err := b.st.SongByID(id); err == nil && so != nil {
				songs = append(songs, so)
			}
		}
		if len(songs) == 0 {
			return nil, errEmptyQueue
		}
		b.pl.PlayQueue(songs, req.Start)
	case req.SongID > 0:
		so, err := b.st.SongByID(req.SongID)
		if err != nil {
			return nil, err
		}
		if so == nil {
			return nil, errSongNotFound
		}
		b.pl.PlayQueue([]*model.Song{so}, 0)
	default:
		return nil, errBadRequest
	}
	return b.status(), nil
}

// Pause 暂停。Resume 继续。Next 下一首。Prev 上一首。
func (b *Backend) Pause() *Status  { b.pl.Pause(); return b.status() }
func (b *Backend) Resume() *Status { b.pl.Play(); return b.status() }
func (b *Backend) Next() *Status   { b.pl.Next(); return b.status() }
func (b *Backend) Prev() *Status   { b.pl.Prev(); return b.status() }
func (b *Backend) Toggle() *Status { b.pl.Toggle(); return b.status() }

// Seek 跳转（秒）。
func (b *Backend) Seek(position float64) *Status {
	if position < 0 {
		position = 0
	}
	b.pl.SeekTo(position)
	return b.status()
}

// SetVolume 音量 0-100。
func (b *Backend) SetVolume(volume float64) *Status {
	if volume < 0 || volume > 100 || math.IsNaN(volume) {
		return b.status()
	}
	b.pl.SetVolume(volume)
	return b.status()
}

// SetMode 播放模式 0-3。
func (b *Backend) SetMode(mode int) *Status {
	if mode < 0 || mode > 3 {
		return b.status()
	}
	b.pl.SetMode(model.PlayMode(mode))
	return b.status()
}

// --- 曲库 ---

// SongInfo 前端用的歌曲信息。
type SongInfo struct {
	model.Song
	Favorited bool `json:"favorited"`
}

func (b *Backend) toSongInfo(so *model.Song) *SongInfo {
	if so == nil {
		return nil
	}
	fav, _ := b.st.IsFavorite(so.ID)
	return &SongInfo{Song: *so, Favorited: fav}
}

// GetSongs 曲库列表（q 支持标题/艺术家/专辑模糊搜索）。
func (b *Backend) GetSongs(q string) []*SongInfo {
	songs, err := b.st.Songs(q)
	if err != nil {
		return nil
	}
	out := make([]*SongInfo, 0, len(songs))
	for _, so := range songs {
		out = append(out, b.toSongInfo(so))
	}
	return out
}

// Library 曲库信息。
func (b *Backend) Library() map[string]any {
	n, _ := b.st.SongCount()
	return map[string]any{
		"roots":      b.cfg.Libraries,
		"song_count": n,
		"scanning":   b.lib.Scanning(),
	}
}

// Scan 后台触发扫描（幂等：进行中时返回提示）。
func (b *Backend) Scan() map[string]any {
	if b.lib.Scanning() {
		return map[string]any{"ok": false, "message": "已有扫描进行中"}
	}
	go func() {
		added, updated, removed, err := b.lib.Scan(b.cfg.Libraries)
		msg := "扫描完成"
		if err != nil {
			msg = "扫描失败：" + err.Error()
		}
		log.Printf("%s: 新增 %d, 更新 %d, 移除 %d", msg, added, updated, removed)
		runtime.EventsEmit(b.ctx, "scan:done", map[string]any{
			"ok": err == nil, "added": added, "updated": updated, "removed": removed,
		})
	}()
	return map[string]any{"ok": true, "scanning": true}
}

// ImportFolder 打开系统目录选择框并把目录加入曲库。
func (b *Backend) ImportFolder() (string, error) {
	if b.ctx == nil {
		return "", errNoContext
	}
	dir, err := runtime.OpenDirectoryDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "选择音乐文件夹",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // 用户取消
	}
	b.addLibrary(dir)
	b.Scan()
	return dir, nil
}

func (b *Backend) addLibrary(dir string) {
	for _, lib := range b.cfg.Libraries {
		if lib == dir {
			return
		}
	}
	b.cfg.Libraries = append(b.cfg.Libraries, dir)
	_ = b.cfg.Save(b.dataDir)
}

// --- 歌单 ---

// GetPlaylists 全部歌单（含歌曲数）。
func (b *Backend) GetPlaylists() []*model.Playlist {
	pls, err := b.st.Playlists()
	if err != nil {
		return nil
	}
	return pls
}

// CreatePlaylist 新建歌单。
func (b *Backend) CreatePlaylist(name string) (int64, error) {
	return b.st.CreatePlaylist(name)
}

// DeletePlaylist 删除歌单。
func (b *Backend) DeletePlaylist(id int64) error { return b.st.DeletePlaylist(id) }

// GetPlaylistSongs 歌单内歌曲。
func (b *Backend) GetPlaylistSongs(id int64) []*SongInfo {
	songs, err := b.st.PlaylistSongs(id)
	if err != nil {
		return nil
	}
	out := make([]*SongInfo, 0, len(songs))
	for _, so := range songs {
		out = append(out, b.toSongInfo(so))
	}
	return out
}

// AddSongToPlaylist 追加歌曲到歌单。
func (b *Backend) AddSongToPlaylist(playlistID, songID int64) error {
	return b.st.AddSongToPlaylist(playlistID, songID)
}

// RemoveSongFromPlaylist 从歌单移除歌曲。
func (b *Backend) RemoveSongFromPlaylist(playlistID, songID int64) error {
	return b.st.RemoveSongFromPlaylist(playlistID, songID)
}

// --- 收藏 / 历史 ---

// GetFavorites 收藏列表。
func (b *Backend) GetFavorites() []*SongInfo {
	songs, err := b.st.Favorites()
	if err != nil {
		return nil
	}
	out := make([]*SongInfo, 0, len(songs))
	for _, so := range songs {
		out = append(out, b.toSongInfo(so))
	}
	return out
}

// ToggleFavorite 切换收藏，返回新状态。
func (b *Backend) ToggleFavorite(songID int64) (bool, error) {
	cur, err := b.st.IsFavorite(songID)
	if err != nil {
		return false, err
	}
	next := !cur
	if err := b.st.SetFavorite(songID, next); err != nil {
		return false, err
	}
	return next, nil
}

// GetHistory 播放历史（去重，最近 100 条）。
func (b *Backend) GetHistory() []*SongInfo {
	songs, err := b.st.History(100)
	if err != nil {
		return nil
	}
	out := make([]*SongInfo, 0, len(songs))
	for _, so := range songs {
		out = append(out, b.toSongInfo(so))
	}
	return out
}

// --- 歌词 / 封面 ---

// GetLyrics 歌曲的 LRC 歌词（找不到返回空数组）。
func (b *Backend) GetLyrics(songID int64) []LyricLine {
	so, err := b.st.SongByID(songID)
	if err != nil || so == nil {
		return nil
	}
	lines := lyrics.Load(so)
	if lines == nil {
		return nil
	}
	out := make([]LyricLine, 0, len(lines))
	for _, l := range lines {
		out = append(out, LyricLine{Time: l.Time.Seconds(), Text: l.Text})
	}
	return out
}

// GetCover 歌曲封面（base64 data URL，无封面返回空串）。
func (b *Backend) GetCover(songID int64) string {
	so, err := b.st.SongByID(songID)
	if err != nil || so == nil || !so.HasCover {
		return ""
	}
	p := b.lib.CoverPath(so)
	if p == "" {
		return ""
	}
	return coverToDataURL(p)
}
