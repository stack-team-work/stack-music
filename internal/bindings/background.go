package bindings

import (
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"go-music/internal/background"
)

// Backgrounds 背景列表 + 当前选择（前端直接消费）。
type Backgrounds struct {
	Current string            `json:"current"`
	Items   []background.Item `json:"items"`
}

// GetBackgrounds 返回用户导入的背景列表与当前选择。
// Wails 模式下 URL 转为绝对地址（窗口跨 origin 直接引用图片）。
func (b *Backend) GetBackgrounds() *Backgrounds {
	if b.bgMgr == nil {
		return &Backgrounds{Current: background.BuiltinDefault}
	}
	cur := b.cfg.Background
	if cur == "" {
		cur = background.BuiltinDefault
	}
	items := b.bgMgr.List()
	for i := range items {
		items[i].URL = b.absoluteURL(items[i].URL)
	}
	return &Backgrounds{Current: cur, Items: items}
}

// absoluteURL 相对路径转本机服务绝对地址。
func (b *Backend) absoluteURL(url string) string {
	if url == "" {
		return ""
	}
	return fmt.Sprintf("http://127.0.0.1:%d%s", b.cfg.Port, url)
}

// SetBackground 切换背景（builtin:xxx 或已导入的背景文件名）。
func (b *Backend) SetBackground(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("背景 ID 不能为空")
	}
	if b.bgMgr != nil && !b.bgMgr.Exists(id) {
		return fmt.Errorf("背景不存在: %s", id)
	}
	b.cfg.Background = id
	return b.cfg.Save(b.dataDir)
}

// ImportBackground 打开系统文件选择框导入一张背景图。
func (b *Backend) ImportBackground() (*background.Item, error) {
	if b.ctx == nil {
		return nil, errNoContext
	}
	if b.bgMgr == nil {
		return nil, fmt.Errorf("背景服务未就绪")
	}
	path, err := runtime.OpenFileDialog(b.ctx, runtime.OpenDialogOptions{
		Title: "选择背景图片（jpg/png/webp/gif，最大 15MB）",
		Filters: []runtime.FileFilter{
			{DisplayName: "图片", Pattern: "*.jpg;*.jpeg;*.png;*.webp;*.gif"},
		},
	})
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil // 用户取消
	}
	item, err := b.bgMgr.Import(path)
	if err != nil {
		return nil, err
	}
	// 导入后立即切换为该背景
	b.cfg.Background = item.ID
	_ = b.cfg.Save(b.dataDir)
	item.URL = b.absoluteURL(item.URL)
	return &item, nil
}

// DeleteBackground 删除用户背景；若正在使用则回退默认渐变。
func (b *Backend) DeleteBackground(id string) error {
	if b.bgMgr == nil {
		return fmt.Errorf("背景服务未就绪")
	}
	if err := b.bgMgr.Delete(id); err != nil {
		return err
	}
	if b.cfg.Background == id {
		b.cfg.Background = background.BuiltinDefault
		return b.cfg.Save(b.dataDir)
	}
	return nil
}
