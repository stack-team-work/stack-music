# UI 迁移方案：Fyne → Wails v2 + Vue 3（已确认三个决策）

## 目标
- 主界面整体替换为 **Wails v2**（Windows WebView2 渲染 HTML/CSS，系统自带运行时）
- 前端用 **Vue 3 + TypeScript + Vite**
- **主窗口与局域网远程控制复用同一套前端**：Wails 环境内调 Go 绑定；浏览器打开 `http://IP:8080` 时前端自动切到 HTTP API 模式
- 音频播放引擎（beep）、曲库、SQLite 存储、Gin 服务全部保留不动

## 一、环境准备
1. 安装 Wails CLI：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
2. 确认 node/npm（已装）、gcc（已装，Wails v2 Windows 构建需要 CGO）
3. WebView2：Win11 26100 系统自带，无需安装

## 二、Go 侧改造（小改动）
1. **main.go** 改为 Wails 入口：
   - wails.Run(Assets: 嵌入 frontend/dist, Bind: bindings.Backend, 窗口尺寸设置)
   - 启动时后台协程拉起现有 Gin server（远程控制，端口可配）
2. **internal/bindings/backend.go**（新增，聚合层）：
   - 播放：Status / Play(queue|song|playlist) / Pause / Resume / Next / Prev / Seek / SetVolume / SetMode
   - 曲库：GetSongs(q) / Scan / ImportFolder（打开系统目录对话框）/ 扫描进度事件
   - 歌单：GetPlaylists / CreatePlaylist / DeletePlaylist / PlaylistSongs / AddSong / RemoveSong
   - 收藏/历史：GetFavorites / ToggleFavorite / GetHistory
   - 歌词：GetLyrics(songID)
   - 事件：播放器事件转 Wails Events.Emit，前端实时刷新（替代原 fyne.Do 链路）
3. **internal/server**：静态资源从"内嵌单页"改为服务 `frontend/dist`（embed 构建产物），API 不变；SSE 保留供浏览器模式
4. **删除 internal/ui/**（全部 Fyne 代码），go mod tidy 清理 fyne 依赖；全库保留的音视频/存储逻辑原样

## 三、前端工程（frontend/，工作量主体）
```
frontend/
├── wails.json / package.json / vite.config.ts / tsconfig.json
├── index.html
└── src/
    ├── api/            # 双模式适配层（核心）
    │   ├── wailsApi.ts   # window.go 绑定直调 + 事件监听
    │   ├── httpApi.ts    # fetch /api/* + SSE
    │   └── index.ts      # 运行时探测自动选择（同一接口签名）
    ├── views/          # 曲库 / 歌单 / 收藏 / 历史
    ├── components/     # SongTable・PlayerBar(封面/进度/音量/播放模式/收藏/歌词)
    │                   # LyricsPanel(逐行高亮+滚动)・Cover・PlaylistEditor
    ├── store/          # Pinia：播放状态、曲库、当前列表
    └── style.css       # 暗色主题 + CSS 变量 + 自定义滚动条（内联 SVG 图标，无外部依赖）
```

页面功能对齐现有桌面版：曲库（搜索+单击播放+收藏/加歌单菜单）、歌单（列表/详情/添加/移除歌曲）、收藏、历史、歌词窗口（同步高亮+点击跳转）、导入文件夹、后台扫描进度。

## 四、构建与验证
1. `npm install && npm run build`（VDist 产物 → embed 进 exe）
2. `wails build` → `build/bin/go-music.exe`（预计 15~25MB，比 Fyne 80MB 显著缩小）
3. 回归清单（复用原有测试音频与已修复链路验证）：
   - 96kHz/24bit WAV 重采样播放、进度、seek、自动切歌、四种模式
   - 歌单/收藏/历史 CRUD
   - 歌词同步显示
   - 远程控制：浏览器打开 http://IP:8080 显示同一套 UI 并可控制
   - go vet + go test 全绿
4. 文档更新：README（技术栈/截图说明）、docs/architecture.md（新架构图）、docs/build.md（Wails 构建）

## 五、风险与对策
- 前端工作量大 → 组件按需实现，先跑通骨架再美化；样式单文件集中管理
- Wails v2 需 CGO/gcc → 本机已装（winlibs）
- WebView2 分发 → Win10/11 系统组件，无需随包安装
- 远程控制不播放网页音频（维持"浏览器控制桌面播放"形态，同现状）
- 老 exe 与数据库兼容：data/music.db 结构不变，旧数据直接可用