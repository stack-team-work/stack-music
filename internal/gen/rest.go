package gen

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go-music/internal/model"
)

// restProvider 通用任务式 REST 后端：
// 适配 MiniMax / Suno 官方接口形态，base_url 可替换为中转地址，model 可自定义。
//
// 请求与响应的字段映射集中在 buildSubmitBody / parseSubmitResp / parsePollResp，
// 若中转站字段略有差异，只需调整这三个函数（或把差异反映在 base_url 上）。
type restProvider struct {
	cfg  Config
	http *http.Client
}

func newRESTProvider(cfg Config) *restProvider {
	return &restProvider{
		cfg:  cfg,
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

func (p *restProvider) Name() string { return p.cfg.Provider }

// Submit 提交生成任务，返回平台任务 ID。
func (p *restProvider) Submit(ctx context.Context, req *Request) (string, string, error) {
	body := p.buildSubmitBody(req)
	raw, _ := json.Marshal(body)

	endpoint := p.submitURL()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return "", "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	p.setAuth(httpReq)

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return "", "", fmt.Errorf("请求生成接口失败: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", fmt.Errorf("生成接口返回 %d: %s", resp.StatusCode, truncateRunes(string(data), 200))
	}

	remoteID, audioURL, err := p.parseSubmitResp(data)
	if err != nil {
		return "", "", err
	}
	if remoteID == "" && audioURL == "" {
		return "", "", fmt.Errorf("接口未返回任务 ID 或音频地址（响应: %s）", truncateRunes(string(data), 200))
	}
	return remoteID, audioURL, nil
}

// Poll 查询任务状态。
func (p *restProvider) Poll(ctx context.Context, remoteID string) (string, string, string, error) {
	endpoint := strings.TrimRight(p.cfg.BaseURL, "/") + "/" + remoteID
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", "", err
	}
	p.setAuth(httpReq)

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return model.GenGenerating, "", "", nil // 网络抖动视为仍在生成，下轮重试
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.GenFailed, "", fmt.Sprintf("查询接口返回 %d", resp.StatusCode), nil
	}
	return p.parsePollResp(data)
}

func (p *restProvider) submitURL() string {
	u := strings.TrimSpace(p.cfg.BaseURL)
	if u == "" {
		u = DefaultBaseURL(p.cfg.Provider)
	}
	return u
}

func (p *restProvider) setAuth(r *http.Request) {
	if p.cfg.APIKey != "" {
		r.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
}

// buildSubmitBody 按平台构造请求体（字段映射集中于此）。
func (p *restProvider) buildSubmitBody(req *Request) map[string]any {
	tags := strings.TrimSpace(req.Style)
	if req.Mood != "" {
		tags = strings.Join(nonEmpty(tags, req.Mood), ", ")
	}
	prompt := req.Prompt
	if req.Mode == ModeLyrics {
		prompt = "" // 歌词模式由 lyrics 字段承载
	}
	body := map[string]any{
		"model":             p.cfg.Model,
		"title":             req.TitleOrDefault(),
		"prompt":            prompt,
		"tags":              tags,
		"style":             tags,
		"lyrics":            req.Lyrics,
		"language":          req.Lang,
		"mood":              req.Mood,
		"custom_mode":       req.Mode == ModeLyrics,
		"instrumental":      req.Mode == ModeInstrumental,
		"make_instrumental": req.Mode == ModeInstrumental,
	}
	if req.Mode == ModeInstrumental && prompt == "" {
		body["prompt"] = tags
	}
	return body
}

func nonEmpty(ss ...string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// parseSubmitResp 从提交响应中提取任务 ID 或直接音频地址。
// 兼容形态：{id} / {task_id} / {data:{task_id}} / {data:{id}} / {data:[{id}]} / {data:{audio}}
func (p *restProvider) parseSubmitResp(data []byte) (remoteID, audioURL string, err error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %w", err)
	}
	pick := func(v map[string]any) (string, string) {
		var id, url string
		for _, k := range []string{"task_id", "taskId", "id", "clip_id"} {
			if s, ok := v[k].(string); ok && s != "" {
				id = s
				break
			}
		}
		for _, k := range []string{"audio_url", "audio", "url", "audioUrl", "stream_audio_url"} {
			if s, ok := v[k].(string); ok && s != "" {
				url = s
				break
			}
		}
		return id, url
	}

	if id, url := pick(m); id != "" || url != "" {
		return id, url, nil
	}
	switch d := m["data"].(type) {
	case map[string]any:
		id, url := pick(d)
		if id != "" || url != "" {
			return id, url, nil
		}
	case []any:
		for _, item := range d {
			if mm, ok := item.(map[string]any); ok {
				id, url := pick(mm)
				if id != "" || url != "" {
					return id, url, nil
				}
			}
		}
	}
	return "", "", nil
}

// parsePollResp 解析轮询响应：状态、音频地址、失败原因。
func (p *restProvider) parsePollResp(data []byte) (status, audioURL, errMsg string, err error) {
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return "", "", "", err
	}
	// 深入 data（对象或数组取第一个）
	target := m
	if d, ok := m["data"].(map[string]any); ok {
		target = d
	} else if arr, ok := m["data"].([]any); ok && len(arr) > 0 {
		if mm, ok := arr[0].(map[string]any); ok {
			target = mm
		}
	}

	str := func(keys ...string) string {
		for _, k := range keys {
			if s, ok := target[k].(string); ok && s != "" {
				return s
			}
		}
		return ""
	}

	audioURL = str("audio_url", "audioUrl", "audio", "url", "stream_audio_url")
	rawStatus := strings.ToLower(str("status", "state", "task_status"))
	failMsg := str("error_message", "fail_reason", "msg", "message")

	switch rawStatus {
	case "complete", "completed", "succeeded", "success", "done", "finished":
		if audioURL == "" {
			return model.GenGenerating, "", "", nil // 已完成但地址未就绪，再等一轮
		}
		return model.GenDone, audioURL, "", nil
	case "failed", "error", "fail":
		if failMsg == "" {
			failMsg = "平台返回失败"
		}
		return model.GenFailed, "", failMsg, nil
	case "queued", "pending", "submitted", "waiting":
		return model.GenQueued, "", "", nil
	default:
		if audioURL != "" {
			return model.GenDone, audioURL, "", nil
		}
		return model.GenGenerating, "", "", nil
	}
}
