// Package library 负责音乐文件夹扫描、标签提取、封面缓存与时长探测。
package library

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/dhowden/tag"

	"go-music/internal/model"
	"go-music/internal/store"
)

// 支持的音频扩展名。
var audioExts = map[string]bool{
	".mp3": true, ".flac": true, ".wav": true, ".ogg": true,
}

// Library 音乐库管理。
type Library struct {
	mu        sync.Mutex
	store     *store.Store
	dataDir   string
	coversDir string
	scanning  bool
	// OnProgress 扫描进度回调（已处理数，总数），可能为 nil。
	OnProgress func(done, total int)
}

// New 创建音乐库管理器。
func New(st *store.Store, dataDir string) *Library {
	return &Library{
		store:     st,
		dataDir:   dataDir,
		coversDir: filepath.Join(dataDir, "covers"),
	}
}

// Scanning 是否正在扫描。
func (l *Library) Scanning() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.scanning
}

// Scan 扫描根目录（后台可重复调用），增量更新曲库：
// 按 path + 修改时间 + 大小判断是否重新提取标签；文件不存在且位于库目录内的旧记录会被删除。
// 返回 新增/更新/删除 数量。
func (l *Library) Scan(roots []string) (added, updated, removed int, err error) {
	l.mu.Lock()
	if l.scanning {
		l.mu.Unlock()
		return 0, 0, 0, fmt.Errorf("已有扫描任务进行中")
	}
	l.scanning = true
	l.mu.Unlock()
	defer func() {
		l.mu.Lock()
		l.scanning = false
		l.mu.Unlock()
	}()

	known, err := l.store.AllPathsModTimes()
	if err != nil {
		return 0, 0, 0, err
	}

	var files []string
	for _, root := range roots {
		if root == "" {
			continue
		}
		root = filepath.Clean(root)
		info, err := os.Stat(root)
		if err != nil || !info.IsDir() {
			continue
		}
		err = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // 跳过无权限目录
			}
			if d.IsDir() {
				// 跳过常见隐藏目录，避免扫描缓慢
				name := d.Name()
				if strings.HasPrefix(name, ".") && name != "." {
					return filepath.SkipDir
				}
				return nil
			}
			if audioExts[strings.ToLower(filepath.Ext(path))] {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			continue
		}
	}

	total := len(files)
	processed := 0
	processedFn := func() {
		processed++
		if l.OnProgress != nil {
			l.OnProgress(processed, total)
		}
	}

	found := make(map[string]bool, len(files))
	for _, path := range files {
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		found[path] = true
		mod := fi.ModTime().Unix()
		if known[path] == mod && fi.Size() > 0 {
			processedFn()
			continue // 未变化
		}
		so, err := l.indexFile(path, fi.Size(), mod)
		if err != nil {
			processedFn()
			continue // 解码失败的文件跳过
		}
		if _, err := l.store.UpsertSong(so); err != nil {
			processedFn()
			continue
		}
		if _, existed := known[path]; existed {
			updated++
		} else {
			added++
		}
		processedFn()
	}

	// 清理：位于库目录内但文件已消失的旧记录
	removedByLib := 0
	for path := range known {
		if found[path] {
			continue
		}
		if !withinRoots(path, roots) {
			continue
		}
		if so, err := l.store.SongByPath(path); err == nil && so != nil {
			_ = l.store.DeleteSong(so.ID)
			removedByLib++
		}
	}

	return added, updated, removedByLib, nil
}

func withinRoots(path string, roots []string) bool {
	for _, r := range roots {
		if r == "" {
			continue
		}
		rel, err := filepath.Rel(filepath.Clean(r), path)
		if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// indexFile 提取单个文件的标签、封面与时长。
func (l *Library) indexFile(path string, size, mod int64) (*model.Song, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	format := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	so := &model.Song{
		Path:    path,
		Size:    size,
		ModTime: mod,
		Format:  format,
		Hash:    pathHash(path),
	}

	// 标签（wav 无标签支持，跳过）
	if format != "wav" {
		m, err := tag.ReadFrom(f)
		if err == nil {
			so.Title = m.Title()
			so.Artist = m.Artist()
			so.Album = m.Album()
			so.Genre = m.Genre()
			so.Year = m.Year()
			so.Track, _ = m.Track()
			if pic := m.Picture(); pic != nil && len(pic.Data) > 0 {
				ext := sniffImageExt(pic.Data)
				_ = os.MkdirAll(l.coversDir, 0o755)
				coverPath := store.CoverPath(l.dataDir, so.Hash, ext)
				if err := os.WriteFile(coverPath, pic.Data, 0o644); err == nil {
					so.HasCover = true
				}
			}
		}
	}

	// 时长探测
	if dur, ok := probeDuration(format, f); ok && dur > 0 {
		so.Duration = dur
	}

	// 标题兜底：取文件名
	if so.Title == "" {
		so.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	return so, nil
}

// pathHash 计算路径哈希，用作封面缓存主键。
func pathHash(path string) string {
	sum := sha1.Sum([]byte(path))
	return hex.EncodeToString(sum[:])[:16]
}

func sniffImageExt(data []byte) string {
	if len(data) >= 8 && data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G' {
		return "png"
	}
	if len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "jpg"
	}
	return "jpg"
}

// CoverPath 返回歌曲封面缓存路径；无封面返回空串。
func (l *Library) CoverPath(so *model.Song) string {
	if so == nil || !so.HasCover {
		return ""
	}
	// 探测实际存在的文件（png 或 jpg）
	for _, ext := range []string{"jpg", "png"} {
		p := store.CoverPath(l.dataDir, so.Hash, ext)
		if fi, err := os.Stat(p); err == nil && fi.Size() > 0 {
			return p
		}
	}
	return ""
}

// OpenCover 打开封面文件用于 HTTP 响应。
func (l *Library) OpenCover(so *model.Song) (io.ReadSeekCloser, string, error) {
	p := l.CoverPath(so)
	if p == "" {
		return nil, "", os.ErrNotExist
	}
	ext := strings.TrimPrefix(filepath.Ext(p), ".")
	if ext == "jpg" {
		ext = "jpeg"
	}
	f, err := os.Open(p)
	return f, "image/" + ext, err
}
