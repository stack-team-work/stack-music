package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"stack-music/internal/background"
	"stack-music/internal/config"
)

// --- 界面背景（浏览器远程模式）---

// SetBackgroundManager 注入背景管理器（可选）。
func (s *Server) SetBackgroundManager(m *background.Manager) { s.bgMgr = m }

func (s *Server) handleGetBackgrounds(c *gin.Context) {
	cur := s.cfg.Background
	if cur == "" {
		cur = background.BuiltinDefault
	}
	items := []background.Item{}
	if s.bgMgr != nil {
		items = s.bgMgr.List()
	}
	c.JSON(http.StatusOK, gin.H{"current": cur, "items": items})
}

// handleBackgroundImage 返回背景图片文件。
func (s *Server) handleBackgroundImage(c *gin.Context) {
	if s.bgMgr == nil {
		c.Status(http.StatusNotFound)
		return
	}
	id := c.Param("id")
	p, ok := s.bgMgr.FilePath(id)
	if !ok {
		c.Status(http.StatusNotFound)
		return
	}
	// 供 Wails 窗口（不同 origin）直接以 img/css 引用
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Cache-Control", "public, max-age=3600")
	c.File(p)
}

// handleBackgroundUpload 浏览器端上传背景图（multipart）。
func (s *Server) handleBackgroundUpload(c *gin.Context) {
	if s.bgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "背景服务未就绪"})
		return
	}
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少文件字段"})
		return
	}
	if file.Size > background.MaxSize {
		c.JSON(http.StatusBadRequest, gin.H{"error": "图片过大（最大 15MB）"})
		return
	}
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "读取上传文件失败"})
		return
	}
	defer f.Close()
	item, err := s.bgMgr.ImportReader(f, file.Filename)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// 上传即设为当前背景
	s.cfg.Background = item.ID
	_ = s.cfg.Save(config.DataDir())
	c.JSON(http.StatusOK, gin.H{"item": item, "current": item.ID})
}

// handleSetBackground 切换背景。
func (s *Server) handleSetBackground(c *gin.Context) {
	var req struct {
		ID string `json:"id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.ID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少背景 id"})
		return
	}
	id := strings.TrimSpace(req.ID)
	if s.bgMgr != nil && !s.bgMgr.Exists(id) {
		c.JSON(http.StatusNotFound, gin.H{"error": "背景不存在"})
		return
	}
	s.cfg.Background = id
	if err := s.cfg.Save(config.DataDir()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "current": id})
}

// handleDeleteBackground 删除用户背景。
func (s *Server) handleDeleteBackground(c *gin.Context) {
	if s.bgMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "背景服务未就绪"})
		return
	}
	id := c.Param("id")
	if err := s.bgMgr.Delete(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if s.cfg.Background == id {
		s.cfg.Background = background.BuiltinDefault
		_ = s.cfg.Save(config.DataDir())
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
