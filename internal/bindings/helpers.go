package bindings

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
)

var (
	errBadRequest    = errors.New("参数错误：需要 song_id / playlist_id / queue 之一")
	errEmptyPlaylist = errors.New("歌单为空")
	errEmptyQueue    = errors.New("队列为空")
	errSongNotFound  = errors.New("歌曲不存在")
	errNoContext     = errors.New("应用上下文未就绪")
	errNoCover       = errors.New("无封面")
)

// coverToDataURL 读取封面文件并转 base64 data URL。
func coverToDataURL(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	ext := filepath.Ext(path)
	if ext == ".jpg" || ext == ".jpeg" {
		return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(b)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b)
}
