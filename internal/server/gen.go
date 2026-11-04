package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"stack-music/internal/config"
	"stack-music/internal/gen"
)

// --- AI 音乐生成（浏览器远程模式同构接口）---

func (s *Server) handleGenGetConfig(c *gin.Context) {
	g := s.cfg.Gen
	if g.Provider == "" {
		g.Provider = gen.ProviderLocal
	}
	c.JSON(http.StatusOK, gin.H{
		"provider": g.Provider,
		"base_url": g.BaseURL,
		"api_key":  g.APIKey,
		"model":    g.Model,
		"defaults": gin.H{
			gen.ProviderMinimax: gin.H{"base_url": gen.DefaultBaseURL(gen.ProviderMinimax), "model": gen.DefaultModel(gen.ProviderMinimax)},
			gen.ProviderSuno:    gin.H{"base_url": gen.DefaultBaseURL(gen.ProviderSuno), "model": gen.DefaultModel(gen.ProviderSuno)},
			gen.ProviderLocal:   gin.H{"base_url": "", "model": ""},
		},
	})
}

func (s *Server) handleGenSetConfig(c *gin.Context) {
	var req struct {
		Provider string `json:"provider"`
		BaseURL  string `json:"base_url"`
		APIKey   string `json:"api_key"`
		Model    string `json:"model"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	provider := strings.TrimSpace(req.Provider)
	switch provider {
	case gen.ProviderMinimax, gen.ProviderSuno, gen.ProviderLocal:
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "未知生成后端"})
		return
	}
	s.cfg.Gen.Provider = provider
	s.cfg.Gen.BaseURL = strings.TrimSpace(req.BaseURL)
	s.cfg.Gen.APIKey = strings.TrimSpace(req.APIKey)
	s.cfg.Gen.Model = strings.TrimSpace(req.Model)
	if err := s.cfg.Save(config.DataDir()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleGenSubmit(c *gin.Context) {
	if s.genMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "生成服务未就绪"})
		return
	}
	var req gen.Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	task, err := s.genMgr.Submit(&req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, task)
}

func (s *Server) handleGenTasks(c *gin.Context) {
	if s.genMgr == nil {
		c.JSON(http.StatusOK, gin.H{"tasks": []any{}})
		return
	}
	tasks, err := s.genMgr.Tasks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (s *Server) handleGenDelete(c *gin.Context) {
	if s.genMgr == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "生成服务未就绪"})
		return
	}
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效 id"})
		return
	}
	if err := s.genMgr.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
