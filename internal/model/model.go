// Package model 定义播放器核心数据结构。
package model

import (
	"fmt"
	"time"
)

// Song 表示曲库中的一首歌曲。
type Song struct {
	ID        int64   `json:"id"`
	Path      string  `json:"path"`   // 音频文件绝对路径
	Title     string  `json:"title"`  // 标题（无标签时取文件名）
	Artist    string  `json:"artist"` // 艺术家
	Album     string  `json:"album"`  // 专辑
	Genre     string  `json:"genre"`  // 风格
	Year      int     `json:"year"`
	Track     int     `json:"track"`
	Duration  float64 `json:"duration"` // 时长（秒），未探测时为 0
	Format    string  `json:"format"`   // mp3 / flac / wav / ogg
	Size      int64   `json:"size"`
	ModTime   int64   `json:"mod_time"` // 文件修改时间（Unix 秒），用于增量扫描
	Hash      string  `json:"hash"`     // 路径哈希，封面缓存文件名
	HasCover  bool    `json:"has_cover"`
	Favorited bool    `json:"favorited"` // 是否收藏
}

// DurationText 返回 mm:ss 格式的时长文本。
func (s *Song) DurationText() string {
	if s.Duration <= 0 {
		return "--:--"
	}
	d := time.Duration(s.Duration * float64(time.Second))
	return formatDuration(d)
}

// DisplayTitle 优先返回标签标题，否则返回文件名。
func (s *Song) DisplayTitle() string {
	return s.Title
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := int(d / time.Minute)
	sec := int(d % time.Minute / time.Second)
	return fmt.Sprintf("%02d:%02d", m, sec)
}

// PlayMode 播放模式。
type PlayMode int

const (
	ModeOrder    PlayMode = iota // 顺序播放：播完列表末尾停止
	ModeLoopList                 // 列表循环
	ModeLoopOne                  // 单曲循环
	ModeShuffle                  // 随机播放
)

func (m PlayMode) String() string {
	switch m {
	case ModeOrder:
		return "顺序播放"
	case ModeLoopList:
		return "列表循环"
	case ModeLoopOne:
		return "单曲循环"
	case ModeShuffle:
		return "随机播放"
	}
	return "未知"
}

// Playlist 歌单。
type Playlist struct {
	ID        int64   `json:"id"`
	Name      string  `json:"name"`
	CreatedAt int64   `json:"created_at"`
	SongCount int     `json:"song_count,omitempty"`
	SongIDs   []int64 `json:"song_ids,omitempty"`
}
