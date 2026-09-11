// Package gen 提供 AI 音乐生成能力：
//   - 可配置的多后端 Provider（MiniMax / Suno 官方或中转，base_url/api_key/model 均可配）
//   - 内置本地合成器（无 API Key 时的离线兜底，生成简单 BGM 级音乐）
//   - Manager 任务管理：提交、轮询、下载、自动入库 My Music
package gen

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 模式。
const (
	ModeText         = "text"         // 描述生成歌曲
	ModeLyrics       = "lyrics"       // 歌词生成歌曲
	ModeInstrumental = "instrumental" // 纯音乐
)

// Provider 名称。
const (
	ProviderMinimax = "minimax"
	ProviderSuno    = "suno"
	ProviderLocal   = "local"
)

// Request 生成请求参数（前后端统一）。
type Request struct {
	Mode   string `json:"mode"`
	Prompt string `json:"prompt"`
	Lyrics string `json:"lyrics"`
	Style  string `json:"style"`
	Mood   string `json:"mood"`
	Lang   string `json:"lang"`
	Title  string `json:"title"`
}

// Validate 校验请求合法性。
func (r *Request) Validate() error {
	switch r.Mode {
	case ModeText, ModeInstrumental:
		if strings.TrimSpace(r.Prompt) == "" {
			return errors.New("请填写音乐描述")
		}
	case ModeLyrics:
		if strings.TrimSpace(r.Lyrics) == "" {
			return errors.New("请填写歌词")
		}
	default:
		return fmt.Errorf("未知生成模式: %q", r.Mode)
	}
	if len([]rune(r.Lyrics)) > 3000 {
		return errors.New("歌词过长（最多 3000 字）")
	}
	if len([]rune(r.Prompt)) > 1000 {
		return errors.New("描述过长（最多 1000 字）")
	}
	return nil
}

// TitleOrDefault 标题兜底。
func (r *Request) TitleOrDefault() string {
	if t := strings.TrimSpace(r.Title); t != "" {
		return t
	}
	if r.Mode == ModeLyrics {
		// 取歌词首行
		if line := strings.TrimSpace(strings.SplitN(r.Lyrics, "\n", 2)[0]); line != "" {
			return truncateRunes(line, 24)
		}
	}
	if p := strings.TrimSpace(r.Prompt); p != "" {
		return truncateRunes(p, 24)
	}
	return "未命名作品"
}

func truncateRunes(s string, n int) string {
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	return string(rs[:n])
}

// Config 生成后端配置（对应 config.Gen）。
type Config struct {
	Provider string `json:"provider"` // minimax / suno / local
	BaseURL  string `json:"base_url"` // 官方地址或中转地址（可带路径与查询参数）
	APIKey   string `json:"api_key"`
	Model    string `json:"model"` // 模型名（中转可自定义）
}

// DefaultBaseURL 各 Provider 的默认接口地址。
func DefaultBaseURL(provider string) string {
	switch provider {
	case ProviderMinimax:
		return "https://api.minimax.chat/v1/music_generation"
	case ProviderSuno:
		return "https://api.sunoapi.org/api/v1/generate"
	default:
		return ""
	}
}

// DefaultModel 各 Provider 的默认模型名。
func DefaultModel(provider string) string {
	switch provider {
	case ProviderMinimax:
		return "music-01"
	case ProviderSuno:
		return "chirp-v3-5"
	default:
		return ""
	}
}

// Provider 音乐生成后端接口。
type Provider interface {
	Name() string
	// Submit 提交任务：
	//   - 远端：返回 remoteID，audioURL 为空，随后由 Manager 轮询 Poll
	//   - 本地合成器：同步生成完成，返回 audioURL=本地文件路径，remoteID 为空
	Submit(ctx context.Context, req *Request) (remoteID, audioURL string, err error)
	// Poll 查询远端任务状态：status 取 model.Gen* 常量；audioURL 非空表示完成。
	Poll(ctx context.Context, remoteID string) (status, audioURL, errMsg string, err error)
}

// NewProvider 按配置构造 Provider。
func NewProvider(cfg Config) (Provider, error) {
	switch cfg.Provider {
	case ProviderLocal, "":
		return &localProvider{}, nil
	case ProviderMinimax, ProviderSuno:
		if strings.TrimSpace(cfg.BaseURL) == "" {
			cfg.BaseURL = DefaultBaseURL(cfg.Provider)
		}
		if strings.TrimSpace(cfg.Model) == "" {
			cfg.Model = DefaultModel(cfg.Provider)
		}
		return newRESTProvider(cfg), nil
	default:
		return nil, fmt.Errorf("未知生成后端: %q", cfg.Provider)
	}
}
