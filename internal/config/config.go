// Package config 负责播放器运行配置的加载与持久化。
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// GenConfig AI 音乐生成配置。
type GenConfig struct {
	Provider string `json:"provider"` // minimax / suno / local
	BaseURL  string `json:"base_url"` // 官方或中转接口地址
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
}

// Config 播放器运行配置，保存于 data/config.json。
type Config struct {
	// Port 本地 Web 服务端口（局域网远程控制用）。
	Port int `json:"port"`
	// Volume 音量 0-100。
	Volume float64 `json:"volume"`
	// Mode 播放模式，对应 model.PlayMode 的值。
	Mode int `json:"mode"`
	// Libraries 已导入的音乐文件夹（绝对路径）。
	Libraries []string `json:"libraries"`
	// Gen AI 音乐生成配置。
	Gen GenConfig `json:"gen"`
}

// Default 返回默认配置。
func Default() *Config {
	return &Config{
		Port:   8080,
		Volume: 100,
		Mode:   1, // 列表循环
		Gen:    GenConfig{Provider: "local"},
	}
}

// Load 从 dataDir/config.json 加载配置，文件不存在时返回默认值。
func Load(dataDir string) *Config {
	cfg := Default()
	path := filepath.Join(dataDir, "config.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return cfg
	}
	if err := json.Unmarshal(b, cfg); err != nil {
		return cfg
	}
	if cfg.Port <= 0 {
		cfg.Port = 8080
	}
	return cfg
}

// Save 将配置写入 dataDir/config.json。
func (c *Config) Save(dataDir string) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, "config.json"), b, 0o644)
}

// DataDir 返回应用数据目录：优先使用 exe 所在目录下的 data，
// 开发模式（go run）退回当前工作目录下的 data。
func DataDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Join(filepath.Dir(exe), "data")
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
		if err := os.MkdirAll(dir, 0o755); err == nil {
			return dir
		}
	}
	dir := filepath.Join(".", "data")
	_ = os.MkdirAll(dir, 0o755)
	return dir
}
