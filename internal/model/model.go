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
	Source    string  `json:"source"`    // file / generated
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

// SongSource 歌曲来源。
const (
	SourceFile      = "file"      // 导入的本地文件
	SourceGenerated = "generated" // AI 生成的音乐
)

// GenTask AI 音乐生成任务。
type GenTask struct {
	ID           int64  `json:"id"`
	Mode         string `json:"mode"`           // text / lyrics / instrumental
	Prompt       string `json:"prompt"`         // 文字描述
	Lyrics       string `json:"lyrics"`         // 歌词（mode=lyrics）
	Style        string `json:"style"`          // 风格
	Mood         string `json:"mood"`           // 情绪
	Lang         string `json:"lang"`           // 语言
	Title        string `json:"title"`          // 歌曲标题
	Provider     string `json:"provider"`       // minimax / suno / local
	Model        string `json:"model"`          // 模型名（中转可自定义）
	RemoteTaskID string `json:"remote_task_id"` // 平台侧任务 ID
	Status       string `json:"status"`         // pending/queued/generating/done/failed
	AudioPath    string `json:"audio_path"`     // 生成音频本地路径
	SongID       int64  `json:"song_id"`        // 入库后的歌曲 ID
	FailReason   string `json:"fail_reason"`    // 失败原因
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
}

// GenTask 状态。
const (
	GenPending    = "pending"
	GenQueued     = "queued"
	GenGenerating = "generating"
	GenDone       = "done"
	GenFailed     = "failed"
)
