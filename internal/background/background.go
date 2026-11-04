// Package background 管理可切换的界面背景图：
// 用户导入的图片保存在 data/backgrounds/，内置渐变背景由前端 CSS 提供。
package background

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 允许的图片格式与大小上限。
var allowedExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true, ".gif": true,
}

// MaxSize 单张背景图大小上限（15MB）。
const MaxSize = 15 << 20

// BuiltinDefault 默认内置背景 ID（前端 CSS 渐变）。
const BuiltinDefault = "builtin:sakura"

// Item 背景项。
type Item struct {
	ID      string `json:"id"`      // builtin:xxx 或 <hex>.<ext>
	Name    string `json:"name"`    // 显示名
	URL     string `json:"url"`     // 图片访问地址（内置为空）
	Builtin bool   `json:"builtin"` // 是否内置
}

// Manager 背景文件管理器。
type Manager struct {
	dir string
}

// NewManager 创建管理器并确保目录存在。
func NewManager(dataDir string) *Manager {
	dir := filepath.Join(dataDir, "backgrounds")
	_ = os.MkdirAll(dir, 0o755)
	return &Manager{dir: dir}
}

// Dir 背景目录。
func (m *Manager) Dir() string { return m.dir }

// List 列出用户导入的背景（按文件名排序）。
func (m *Manager) List() []Item {
	entries, err := os.ReadDir(m.dir)
	if err != nil {
		return nil
	}
	var out []Item
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if !allowedExts[ext] {
			continue
		}
		fi, err := e.Info()
		if err != nil || fi.Size() == 0 {
			continue
		}
		id := e.Name()
		short := strings.TrimSuffix(e.Name(), ext)
		if len(short) > 8 {
			short = short[:8]
		}
		out = append(out, Item{
			ID:   id,
			Name: "自定义背景 " + short,
			URL:  "/api/backgrounds/" + id,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Import 复制本地图片文件到背景目录，返回背景项。
func (m *Manager) Import(srcPath string) (Item, error) {
	ext := strings.ToLower(filepath.Ext(srcPath))
	if !allowedExts[ext] {
		return Item{}, fmt.Errorf("不支持的图片格式（支持 jpg/png/webp/gif）")
	}
	fi, err := os.Stat(srcPath)
	if err != nil {
		return Item{}, err
	}
	if fi.IsDir() {
		return Item{}, fmt.Errorf("请选择图片文件")
	}
	if fi.Size() > MaxSize {
		return Item{}, fmt.Errorf("图片过大（最大 15MB）")
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return Item{}, err
	}
	defer f.Close()
	return m.importReader(f, ext)
}

// ImportReader 从上传流保存背景（文件名仅用于取扩展名）。
func (m *Manager) ImportReader(r io.Reader, filename string) (Item, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if !allowedExts[ext] {
		return Item{}, fmt.Errorf("不支持的图片格式（支持 jpg/png/webp/gif）")
	}
	return m.importReader(r, ext)
}

func (m *Manager) importReader(r io.Reader, ext string) (Item, error) {
	id, err := randomID()
	if err != nil {
		return Item{}, err
	}
	name := id + ext
	dst := filepath.Join(m.dir, name)
	out, err := os.Create(dst)
	if err != nil {
		return Item{}, err
	}
	defer out.Close()
	// 限制读取上限，防止超大上传
	n, err := io.Copy(out, io.LimitReader(r, MaxSize+1))
	if err != nil {
		_ = os.Remove(dst)
		return Item{}, err
	}
	if n > MaxSize {
		_ = os.Remove(dst)
		return Item{}, fmt.Errorf("图片过大（最大 15MB）")
	}
	if n == 0 {
		_ = os.Remove(dst)
		return Item{}, fmt.Errorf("图片内容为空")
	}
	return Item{ID: name, Name: strings.TrimSuffix(name, ext), URL: "/api/backgrounds/" + name}, nil
}

// Delete 删除用户背景；内置背景与非法 ID 返回错误。
func (m *Manager) Delete(id string) error {
	p, ok := m.FilePath(id)
	if !ok {
		return fmt.Errorf("背景不存在或不可删除")
	}
	return os.Remove(p)
}

// FilePath 返回用户背景的绝对路径（仅允许背景目录内、白名单扩展名）。
func (m *Manager) FilePath(id string) (string, bool) {
	if strings.ContainsAny(id, "/\\") || strings.Contains(id, "..") {
		return "", false
	}
	ext := strings.ToLower(filepath.Ext(id))
	if !allowedExts[ext] {
		return "", false
	}
	p := filepath.Join(m.dir, id)
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return "", false
	}
	return p, true
}

// Exists 背景是否可用（内置 ID 或已存在的用户文件）。
func (m *Manager) Exists(id string) bool {
	if strings.HasPrefix(id, "builtin:") {
		return true
	}
	_, ok := m.FilePath(id)
	return ok
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
