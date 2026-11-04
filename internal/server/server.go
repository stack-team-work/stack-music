// Package server 提供内嵌的本地 HTTP 服务：
// 播放器 REST API + 前端静态资源（与桌面窗口同一套 Vue 前端，浏览器可远程控制）。
package server

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-music/internal/background"
	"go-music/internal/config"
	"go-music/internal/gen"
	"go-music/internal/library"
	"go-music/internal/lyrics"
	"go-music/internal/model"
	"go-music/internal/player"
	"go-music/internal/store"
)

// Session 描述一次播放会话（请求播放时构造）。
type Session interface {
	// Queue 返回当前播放队列。
	Queue() []*model.Song
	// QueuePos 当前队列位置。
	QueuePos() int
	// PlayQueue 从 start 开始播放队列。
	PlayQueue([]*model.Song, int)
}

// Controller 是 Server 需要的播放器能力子集（由 *player.Player 实现）。
type Controller interface {
	Session
	Toggle()
	Pause()
	Play()
	Next()
	Prev()
	SeekTo(float64)
	SetVolume(float64)
	Volume() float64
	SetMode(model.PlayMode)
	Mode() model.PlayMode
	Current() *model.Song
	State() player.State
	Position() (time.Duration, time.Duration)
	Events() <-chan player.Event
}

// Server HTTP 服务。
type Server struct {
	cfg    *config.Config
	st     *store.Store
	lib    *library.Library
	pl     Controller
	genMgr *gen.Manager
	bgMgr  *background.Manager
	port   int
	assets fs.FS // 前端构建产物（frontend/dist）
}

// New 创建服务。assets 为前端静态资源（main 传入 go:embed 的 frontend/dist）。
func New(cfg *config.Config, st *store.Store, lib *library.Library, pl Controller, assets fs.FS) *Server {
	return &Server{cfg: cfg, st: st, lib: lib, pl: pl, port: cfg.Port, assets: assets}
}

// SetGenManager 注入 AI 音乐生成管理器（可选）。
func (s *Server) SetGenManager(m *gen.Manager) { s.genMgr = m }

// Start 启动 Gin 服务（阻塞直至出错）。
func (s *Server) Start() error {
	gin.SetMode(gin.ReleaseMode)
	return s.buildRouter().Run(fmt.Sprintf(":%d", s.port))
}

// buildRouter 构建路由（供测试使用）。
func (s *Server) buildRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 静态资源：非 /api 路径由 NoRoute 兜底提供，
	// 避免 FileServer 的 /*filepath 通配与 API 路由冲突。
	// 注意：Go 1.25 的 FileServer 会把 /index.html 规范化重定向（301），
	// 因此根路径直接读文件返回，不走 FileFromFS。
	indexHTML, _ := fs.ReadFile(s.assets, "index.html")
	// 所有静态响应禁缓存：保证浏览器/WebView 始终加载最新前端，避免旧页面问题。
	r.GET("/", func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexHTML)
	})
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "接口不存在"})
			return
		}
		c.Header("Cache-Control", "no-store")
		http.FileServer(http.FS(s.assets)).ServeHTTP(c.Writer, c.Request)
	})

	api := r.Group("/api")
	{
		api.GET("/status", s.handleStatus)
		api.GET("/events", s.handleEvents) // SSE
		api.GET("/songs", s.handleSongs)
		api.DELETE("/songs/:id", s.handleDeleteSong)
		api.GET("/favorites", s.handleFavorites)
		api.POST("/favorite", s.handleSetFavorite)
		api.GET("/playlists", s.handlePlaylists)
		api.POST("/playlists", s.handleCreatePlaylist)
		api.DELETE("/playlists/:id", s.handleDeletePlaylist)
		api.GET("/playlists/:id/songs", s.handlePlaylistSongs)
		api.POST("/playlists/:id/songs", s.handleAddToPlaylist)
		api.DELETE("/playlists/:id/songs/:songId", s.handleRemoveFromPlaylist)

		api.POST("/play", s.handlePlay)
		api.POST("/pause", s.handlePause)
		api.POST("/resume", s.handleResume)
		api.POST("/toggle", s.handleToggle)
		api.POST("/next", s.handleNext)
		api.POST("/prev", s.handlePrev)
		api.POST("/seek", s.handleSeek)
		api.POST("/volume", s.handleVolume)
		api.POST("/mode", s.handleMode)
		api.GET("/queue", s.handleQueue)

		api.GET("/history", s.handleHistory)
		api.GET("/lyrics/:id", s.handleLyrics)

		// AI 音乐生成
		api.GET("/gen/config", s.handleGenGetConfig)
		api.POST("/gen/config", s.handleGenSetConfig)
		api.POST("/gen/submit", s.handleGenSubmit)
		api.GET("/gen/tasks", s.handleGenTasks)
		api.DELETE("/gen/tasks/:id", s.handleGenDelete)

		// 界面背景
		api.GET("/backgrounds", s.handleGetBackgrounds)
		api.GET("/backgrounds/:id", s.handleBackgroundImage)
		api.POST("/backgrounds/upload", s.handleBackgroundUpload)
		api.POST("/backgrounds/current", s.handleSetBackground)
		api.DELETE("/backgrounds/:id", s.handleDeleteBackground)
		api.GET("/covers/:id", s.handleCover)
		api.POST("/scan", s.handleScan)
		api.GET("/library", s.handleLibrary)
	}
	return r
}

// --- 响应构造 ---

type statusResp struct {
	State       string    `json:"state"`
	Position    float64   `json:"position"`
	Duration    float64   `json:"duration"`
	Volume      float64   `json:"volume"`
	Mode        int       `json:"mode"`
	ModeText    string    `json:"mode_text"`
	Current     *songResp `json:"current"`
	QueuePos    int       `json:"queue_pos"`
	QueueLen    int       `json:"queue_len"`
	Scanning    bool      `json:"scanning"`
	SongCount   int       `json:"song_count"`
	LibraryRoot []string  `json:"library_roots"`
	CoversURL   string    `json:"covers_url"`
}

type songResp struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	Artist    string  `json:"artist"`
	Album     string  `json:"album"`
	Genre     string  `json:"genre"`
	Year      int     `json:"year"`
	Duration  float64 `json:"duration"`
	Format    string  `json:"format"`
	Favorited bool    `json:"favorited"`
	CoverURL  string  `json:"cover_url,omitempty"`
}

func (s *Server) toSongResp(so *model.Song) *songResp {
	if so == nil {
		return nil
	}
	fav, _ := s.st.IsFavorite(so.ID)
	r := &songResp{
		ID: so.ID, Title: so.Title, Artist: so.Artist, Album: so.Album,
		Genre: so.Genre, Year: so.Year, Duration: so.Duration,
		Format: so.Format, Favorited: fav,
	}
	if so.HasCover {
		r.CoverURL = fmt.Sprintf("/api/covers/%d", so.ID)
	}
	return r
}

func (s *Server) buildStatus() *statusResp {
	pos, dur := s.pl.Position()
	st := s.pl.State()
	resp := &statusResp{
		State:    st.String(),
		Position: pos.Seconds(),
		Duration: dur.Seconds(),
		Volume:   s.pl.Volume(),
		Mode:     int(s.pl.Mode()),
		ModeText: s.pl.Mode().String(),
		Current:  s.toSongResp(s.pl.Current()),
		QueuePos: s.pl.QueuePos(),
		QueueLen: len(s.pl.Queue()),
		Scanning: s.lib.Scanning(),
	}
	if n, err := s.st.SongCount(); err == nil {
		resp.SongCount = n
	}
	resp.LibraryRoot = append([]string(nil), s.cfg.Libraries...)
	if resp.Current != nil && resp.Current.CoverURL != "" {
		resp.CoversURL = resp.Current.CoverURL
	}
	return resp
}

func (s *Server) handleStatus(c *gin.Context) {
	c.JSON(http.StatusOK, s.buildStatus())
}

// handleEvents SSE 推送：播放事件或 20s 心跳时推送完整状态。
func (s *Server) handleEvents(c *gin.Context) {
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "SSE 不支持"})
		return
	}
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	events := s.pl.Events()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// 连接后立即推送一次当前状态
	body, _ := jsonMarshal(s.buildStatus())
	fmt.Fprintf(c.Writer, "data: %s\n\n", body)
	flusher.Flush()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
		case <-events:
		}
		body, _ := jsonMarshal(s.buildStatus())
		fmt.Fprintf(c.Writer, "data: %s\n\n", body)
		flusher.Flush()
	}
}

// --- 曲库 ---

func (s *Server) handleSongs(c *gin.Context) {
	songs, err := s.st.Songs(c.Query("q"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*songResp, 0, len(songs))
	for _, so := range songs {
		out = append(out, s.toSongResp(so))
	}
	c.JSON(http.StatusOK, gin.H{"songs": out, "total": len(out)})
}

func (s *Server) handleDeleteSong(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	_ = s.st.DeleteSong(id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- 收藏 ---

func (s *Server) handleFavorites(c *gin.Context) {
	songs, err := s.st.Favorites()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*songResp, 0, len(songs))
	for _, so := range songs {
		out = append(out, s.toSongResp(so))
	}
	c.JSON(http.StatusOK, gin.H{"songs": out})
}

func (s *Server) handleSetFavorite(c *gin.Context) {
	var req struct {
		SongID   int64 `json:"song_id"`
		Favorite *bool `json:"favorite"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SongID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	fav := true
	if req.Favorite != nil {
		fav = *req.Favorite
	}
	if err := s.st.SetFavorite(req.SongID, fav); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "favorited": fav})
}

// --- 歌单 ---

func (s *Server) handlePlaylists(c *gin.Context) {
	pls, err := s.st.Playlists()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"playlists": pls})
}

func (s *Server) handleCreatePlaylist(c *gin.Context) {
	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name 不能为空"})
		return
	}
	id, err := s.st.CreatePlaylist(req.Name)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "id": id})
}

func (s *Server) handleDeletePlaylist(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	_ = s.st.DeletePlaylist(id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handlePlaylistSongs(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	songs, err := s.st.PlaylistSongs(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*songResp, 0, len(songs))
	for _, so := range songs {
		out = append(out, s.toSongResp(so))
	}
	c.JSON(http.StatusOK, gin.H{"songs": out})
}

func (s *Server) handleAddToPlaylist(c *gin.Context) {
	pid, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	var req struct {
		SongID int64 `json:"song_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.SongID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if err := s.st.AddSongToPlaylist(pid, req.SongID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleRemoveFromPlaylist(c *gin.Context) {
	pid, err1 := strconv.ParseInt(c.Param("id"), 10, 64)
	sid, err2 := strconv.ParseInt(c.Param("songId"), 10, 64)
	if err1 != nil || err2 != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	if err := s.st.RemoveSongFromPlaylist(pid, sid); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- 播放控制 ---

type playReq struct {
	SongID     int64   `json:"song_id"`
	PlaylistID int64   `json:"playlist_id"`
	Queue      []int64 `json:"queue"`
	Start      int     `json:"start"`
}

func (s *Server) handlePlay(c *gin.Context) {
	var req playReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	switch {
	case req.PlaylistID > 0:
		songs, err := s.st.PlaylistSongs(req.PlaylistID)
		if err != nil || len(songs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "歌单为空"})
			return
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
		s.pl.PlayQueue(songs, start)
	case len(req.Queue) > 0:
		songs := make([]*model.Song, 0, len(req.Queue))
		for _, id := range req.Queue {
			if so, err := s.st.SongByID(id); err == nil && so != nil {
				songs = append(songs, so)
			}
		}
		if len(songs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "队列为空"})
			return
		}
		s.pl.PlayQueue(songs, req.Start)
	case req.SongID > 0:
		so, err := s.st.SongByID(req.SongID)
		if err != nil || so == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "歌曲不存在"})
			return
		}
		s.pl.PlayQueue([]*model.Song{so}, 0)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少 song_id / playlist_id / queue"})
		return
	}
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handlePause(c *gin.Context) {
	s.pl.Pause()
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleResume(c *gin.Context) {
	s.pl.Play()
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleToggle(c *gin.Context) {
	s.pl.Toggle()
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleNext(c *gin.Context) {
	s.pl.Next()
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handlePrev(c *gin.Context) {
	s.pl.Prev()
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleSeek(c *gin.Context) {
	var req struct {
		Position float64 `json:"position"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Position < 0 {
		req.Position = 0
	}
	s.pl.SeekTo(req.Position)
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleVolume(c *gin.Context) {
	var req struct {
		Volume float64 `json:"volume"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	if req.Volume < 0 {
		req.Volume = 0
	}
	if req.Volume > 100 {
		req.Volume = 100
	}
	s.pl.SetVolume(req.Volume)
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleMode(c *gin.Context) {
	var req struct {
		Mode int `json:"mode"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Mode < 0 || req.Mode > 3 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mode 须为 0-3"})
		return
	}
	s.pl.SetMode(model.PlayMode(req.Mode))
	c.JSON(http.StatusOK, s.buildStatus())
}

func (s *Server) handleQueue(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"pos":   s.pl.QueuePos(),
		"songs": queueToResp(s.pl.Queue(), s.toSongResp),
	})
}

// --- 历史 / 封面 ---

func (s *Server) handleHistory(c *gin.Context) {
	songs, err := s.st.History(50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	out := make([]*songResp, 0, len(songs))
	for _, so := range songs {
		out = append(out, s.toSongResp(so))
	}
	c.JSON(http.StatusOK, gin.H{"songs": out})
}

// handleLyrics 返回歌曲的 LRC 歌词行（无歌词返回空数组）。
func (s *Server) handleLyrics(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	so, err := s.st.SongByID(id)
	if err != nil || so == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "歌曲不存在"})
		return
	}
	lines := lyrics.Load(so)
	out := make([]gin.H, 0, len(lines))
	for _, l := range lines {
		out = append(out, gin.H{"time": l.Time.Seconds(), "text": l.Text})
	}
	c.JSON(http.StatusOK, gin.H{"lines": out})
}

func (s *Server) handleCover(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	so, err := s.st.SongByID(id)
	if err != nil || so == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "歌曲不存在"})
		return
	}
	rc, contentType, err := s.lib.OpenCover(so)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "无封面"})
		return
	}
	defer rc.Close()
	c.DataFromReader(http.StatusOK, -1, contentType, rc, nil)
}

// --- 扫描 / 曲库信息 ---

func (s *Server) handleScan(c *gin.Context) {
	if s.lib.Scanning() {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "已有扫描进行中"})
		return
	}
	go func() {
		added, updated, removed, err := s.lib.Scan(s.cfg.Libraries)
		log.Printf("扫描完成: 新增 %d, 更新 %d, 移除 %d, err=%v", added, updated, removed, err)
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true, "scanning": true})
}

func (s *Server) handleLibrary(c *gin.Context) {
	n, _ := s.st.SongCount()
	c.JSON(http.StatusOK, gin.H{
		"roots":      s.cfg.Libraries,
		"song_count": n,
		"scanning":   s.lib.Scanning(),
	})
}
