package bindings

import (
	"fmt"
	"strings"

	"go-music/internal/config"
	"go-music/internal/gen"
	"go-music/internal/model"
)

// --- AI 音乐生成 ---

// genConfigDTO 生成配置（含 Key，便于桌面端回显编辑）。
type genConfigDTO struct {
	Provider string `json:"provider"`
	BaseURL  string `json:"base_url"`
	APIKey   string `json:"api_key"`
	Model    string `json:"model"`
	// Defaults 各 Provider 的默认地址/模型（前端选择时自动填充）。
	Defaults map[string]genDefaults `json:"defaults"`
}

type genDefaults struct {
	BaseURL string `json:"base_url"`
	Model   string `json:"model"`
}

// GetGenConfig 读取生成配置。
func (b *Backend) GetGenConfig() *genConfigDTO {
	g := b.cfg.Gen
	if g.Provider == "" {
		g.Provider = gen.ProviderLocal
	}
	return &genConfigDTO{
		Provider: g.Provider,
		BaseURL:  g.BaseURL,
		APIKey:   g.APIKey,
		Model:    g.Model,
		Defaults: map[string]genDefaults{
			gen.ProviderMinimax: {BaseURL: gen.DefaultBaseURL(gen.ProviderMinimax), Model: gen.DefaultModel(gen.ProviderMinimax)},
			gen.ProviderSuno:    {BaseURL: gen.DefaultBaseURL(gen.ProviderSuno), Model: gen.DefaultModel(gen.ProviderSuno)},
			gen.ProviderLocal:   {},
		},
	}
}

// SetGenConfig 保存生成配置（Provider/中转地址/Key/模型）。
func (b *Backend) SetGenConfig(cfg genConfigDTO) error {
	provider := strings.TrimSpace(cfg.Provider)
	switch provider {
	case gen.ProviderMinimax, gen.ProviderSuno, gen.ProviderLocal:
	default:
		return fmt.Errorf("未知生成后端: %q", provider)
	}
	b.cfg.Gen = config.GenConfig{
		Provider: provider,
		BaseURL:  strings.TrimSpace(cfg.BaseURL),
		APIKey:   strings.TrimSpace(cfg.APIKey),
		Model:    strings.TrimSpace(cfg.Model),
	}
	return b.cfg.Save(b.dataDir)
}

// GenSubmit 提交生成任务。
func (b *Backend) GenSubmit(req gen.Request) (*model.GenTask, error) {
	if b.genMgr == nil {
		return nil, fmt.Errorf("生成服务未就绪")
	}
	return b.genMgr.Submit(&req)
}

// GenTasks 生成历史（新→旧）。
func (b *Backend) GenTasks() ([]*model.GenTask, error) {
	if b.genMgr == nil {
		return nil, fmt.Errorf("生成服务未就绪")
	}
	return b.genMgr.Tasks()
}

// GenDeleteTask 删除生成任务（含入库歌曲与音频文件）。
func (b *Backend) GenDeleteTask(id int64) error {
	if b.genMgr == nil {
		return fmt.Errorf("生成服务未就绪")
	}
	return b.genMgr.Delete(id)
}

// GenActive 进行中的生成任务数。
func (b *Backend) GenActive() int {
	if b.genMgr == nil {
		return 0
	}
	return b.genMgr.Active()
}
