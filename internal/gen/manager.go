package gen

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go-music/internal/library"
	"go-music/internal/model"
	"go-music/internal/store"
)

const (
	pollInterval = 8 * time.Second
	taskTimeout  = 30 * time.Minute
)

// Manager AI 音乐生成任务管理器。
type Manager struct {
	st      *store.Store
	lib     *library.Library
	dataDir string
	// cfgFn 每次提交时读取当前配置（支持运行时修改 Provider/Key）。
	cfgFn func() Config
	// OnEvent 事件回调（gen:queued / gen:done / gen:failed），可空。
	OnEvent func(name string, payload any)

	mu     sync.Mutex
	active map[int64]bool
	http   *http.Client
}

// NewManager 创建管理器。
func NewManager(st *store.Store, lib *library.Library, dataDir string, cfgFn func() Config) *Manager {
	return &Manager{
		st:      st,
		lib:     lib,
		dataDir: dataDir,
		cfgFn:   cfgFn,
		active:  make(map[int64]bool),
		http:    &http.Client{Timeout: 5 * time.Minute},
	}
}

func (m *Manager) emit(name string, payload any) {
	if m.OnEvent != nil {
		m.OnEvent(name, payload)
	}
}

func (m *Manager) fail(task *model.GenTask, reason string) {
	task.Status = model.GenFailed
	task.FailReason = reason
	_ = m.st.UpdateGenTask(task)
	m.emit("gen:failed", task)
}

// Submit 创建任务并异步处理。
func (m *Manager) Submit(req *Request) (*model.GenTask, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	cfg := m.cfgFn()
	if cfg.Provider == "" {
		cfg.Provider = ProviderLocal
	}
	if cfg.Provider != ProviderLocal && strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("未配置 API Key：请先在「AI 音乐 → 设置」中填写，或切换到本地合成器")
	}
	task := &model.GenTask{
		Mode:     req.Mode,
		Prompt:   req.Prompt,
		Lyrics:   req.Lyrics,
		Style:    req.Style,
		Mood:     req.Mood,
		Lang:     req.Lang,
		Title:    req.TitleOrDefault(),
		Provider: cfg.Provider,
		Model:    cfg.Model,
		Status:   model.GenPending,
	}
	id, err := m.st.CreateGenTask(task)
	if err != nil {
		return nil, err
	}
	task.ID = id

	m.mu.Lock()
	m.active[id] = true
	m.mu.Unlock()

	go m.process(id)
	m.emit("gen:queued", task)
	return task, nil
}

// process 单个任务的完整生命周期。
func (m *Manager) process(taskID int64) {
	defer func() {
		m.mu.Lock()
		delete(m.active, taskID)
		m.mu.Unlock()
	}()

	task, err := m.st.GenTaskByID(taskID)
	if err != nil || task == nil {
		return
	}
	cfg := m.cfgFn()
	provider, err := NewProvider(cfg)
	if err != nil {
		m.fail(task, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), taskTimeout)
	defer cancel()

	remoteID, audioURL, err := provider.Submit(ctx, taskToRequest(task))
	if err != nil {
		m.fail(task, err.Error())
		return
	}

	// 本地合成器：直接完成
	if audioURL != "" {
		m.finish(task, audioURL)
		return
	}

	task.RemoteTaskID = remoteID
	task.Status = model.GenQueued
	_ = m.st.UpdateGenTask(task)
	m.emit("gen:queued", task)

	// 远端：轮询直到完成/失败/超时
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	// 首次稍等后查询
	select {
	case <-ctx.Done():
		m.fail(task, "生成超时（30 分钟）")
		return
	case <-time.After(3 * time.Second):
	}

	for {
		status, url, msg, err := provider.Poll(ctx, remoteID)
		if err == nil {
			if status != task.Status && status != "" {
				task.Status = status
				_ = m.st.UpdateGenTask(task)
				m.emit("gen:queued", task)
			}
			switch status {
			case model.GenDone:
				m.finish(task, url)
				return
			case model.GenFailed:
				m.fail(task, msg)
				return
			}
		}
		select {
		case <-ctx.Done():
			m.fail(task, "生成超时（30 分钟）")
			return
		case <-ticker.C:
		}
	}
}

func taskToRequest(t *model.GenTask) *Request {
	return &Request{
		Mode: t.Mode, Prompt: t.Prompt, Lyrics: t.Lyrics,
		Style: t.Style, Mood: t.Mood, Lang: t.Lang, Title: t.Title,
	}
}

// finish 下载/整理音频文件并入库 My Music。
func (m *Manager) finish(task *model.GenTask, audioURL string) {
	path, err := m.fetchAudio(task, audioURL)
	if err != nil {
		m.fail(task, "下载生成结果失败: "+err.Error())
		return
	}

	so, err := m.lib.IndexFile(path)
	if err != nil || so == nil {
		so = &model.Song{
			Path:   path,
			Format: strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), "."),
		}
		if fi, err := os.Stat(path); err == nil {
			so.Size = fi.Size()
			so.ModTime = fi.ModTime().Unix()
		}
	}
	so.Title = task.Title
	so.Artist = "AI 音乐"
	so.Album = "My Music"
	so.Genre = task.Style
	so.Source = model.SourceGenerated
	id, err := m.st.UpsertSong(so)
	if err != nil {
		m.fail(task, "入库失败: "+err.Error())
		return
	}

	task.Status = model.GenDone
	task.AudioPath = path
	task.SongID = id
	task.FailReason = ""
	_ = m.st.UpdateGenTask(task)
	m.emit("gen:done", task)
}

// fetchAudio 把音频内容落到 data/generated/ 目录。
// audioURL 为本地路径（合成器）时直接返回；http(s) 时下载。
func (m *Manager) fetchAudio(task *model.GenTask, audioURL string) (string, error) {
	dir := filepath.Join(m.dataDir, "generated")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("%s-%d", sanitizeFileName(task.Title), time.Now().Unix())

	// 本地文件（合成器产物）
	if !strings.HasPrefix(audioURL, "http://") && !strings.HasPrefix(audioURL, "https://") {
		ext := filepath.Ext(audioURL)
		dst := filepath.Join(dir, name+ext)
		if err := copyFile(audioURL, dst); err != nil {
			return "", err
		}
		return dst, nil
	}

	// 远端下载
	resp, err := m.http.Get(audioURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ext := extFromContentType(resp.Header.Get("Content-Type"), audioURL)
	dst := filepath.Join(dir, name+ext)
	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 200<<20)); err != nil { // 上限 200MB
		return "", err
	}
	return dst, nil
}

func extFromContentType(ct, url string) string {
	ct = strings.ToLower(ct)
	switch {
	case strings.Contains(ct, "mpeg"), strings.Contains(ct, "mp3"):
		return ".mp3"
	case strings.Contains(ct, "wav"):
		return ".wav"
	case strings.Contains(ct, "flac"):
		return ".flac"
	case strings.Contains(ct, "ogg"):
		return ".ogg"
	case strings.Contains(ct, "mp4"), strings.Contains(ct, "m4a"), strings.Contains(ct, "aac"):
		return ".m4a"
	}
	if e := strings.ToLower(filepath.Ext(strings.SplitN(url, "?", 2)[0])); e != "" && len(e) <= 5 {
		return e
	}
	return ".mp3"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// Delete 删除任务：同时清理入库歌曲与音频文件。
func (m *Manager) Delete(taskID int64) error {
	task, err := m.st.GenTaskByID(taskID)
	if err != nil || task == nil {
		return err
	}
	if task.SongID > 0 {
		_ = m.st.DeleteSong(task.SongID)
	}
	if task.AudioPath != "" {
		_ = os.Remove(task.AudioPath)
	}
	return m.st.DeleteGenTask(taskID)
}

// Tasks 全部生成历史。
func (m *Manager) Tasks() ([]*model.GenTask, error) { return m.st.GenTasks() }

// Active 当前进行中的任务数。
func (m *Manager) Active() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.active)
}

var errNotReady = errors.New("生成服务未就绪")
