# Stack Music

> 当前版本 **v1.2.1**（Wails v2 + Vue 3 + Naive UI · 二次元樱粉主题 · AI 音乐生成）

一个使用 Go 语言开发的 Windows 桌面音乐播放器：**Wails** 提供桌面窗口（WebView2 渲染），**Vue 3 + Naive UI** 提供现代界面，**beep** 提供音频解码与播放，**Gin** 提供内嵌本地 API 服务，支持同一局域网内的浏览器/手机远程控制。

## 功能特性

- **本地曲库**：导入任意音乐文件夹，自动扫描 mp3 / flac / wav / ogg 文件，增量更新；启动或点击「刷新曲库」后**扫描完成自动重载列表**并提示新增数量
- **标签与封面**：读取 ID3v2 / FLAC / Vorbis 标签（标题、艺术家、专辑、年代、音轨），提取内嵌封面图并缓存
- **任意采样率**：播放前自动重采样到 44100Hz，48kHz / 96kHz / 192kHz 等无损音源（含 24bit WAV）正常速度、正常音调播放
- **播放控制**：播放 / 暂停 / 上一首 / 下一首、进度条拖动、音量调节、四种播放模式（顺序、列表循环、单曲循环、随机）、播完自动切歌
- **歌单管理**：创建 / 编辑 / 删除歌单，添加与移除歌曲
- **收藏与历史**：一键收藏歌曲；自动记录播放历史
- **歌词**：自动查找并同步显示同名 `.lrc` 歌词（高亮当前行、自动滚动、点击跳转）
- **曲库浏览**：完整列表滚动浏览（亮色滚动条 + 表格卡片边界），支持标题/艺术家/专辑搜索过滤
- **局域网远程控制**：exe 内嵌 Gin HTTP 服务，手机 / 电脑浏览器打开 `http://<电脑IP>:8080` 获得与桌面端**同一套界面**的远程控制
- **AI 音乐生成**：
  - 三种模式：文字描述生成歌曲 / 自己的歌词谱曲 / 纯音乐（BGM）
  - 全量参数：10 种风格（Pop/Hip Hop/Rock/Electronic/Acoustic/R&B/Jazz/Classical/Country/Lofi）、情绪、语言、标题
  - 模型后端可配置：MiniMax / Suno 官方接口或中转地址（base_url / API Key / 模型名均可改）
  - **内置本地合成器**：无 API Key 也能离线生成简单 BGM，走通 生成→保存→回听 全流程
  - 生成结果自动入库到 **My Music** 专辑，可回听、回填参数重新生成、一键删除
- **现代界面**：二次元樱粉 × 天蓝浅色主题、毛玻璃面板、渐变按钮与选中态；歌曲列表铺满内容区（标题列自适应），支持搜索过滤
- **可切换背景**：内置 4 套二次元渐变背景（樱花粉/晴空蓝/星空紫/薄荷绿）+ 从本地导入任意图片（jpg/png/webp/gif，≤15MB，如火影等动漫壁纸）；一键切换、重启保持；静态资源强制 no-store，浏览器始终加载最新版本

## 技术栈

| 组件 | 选型 | 说明 |
|---|---|---|
| 桌面窗口 | [Wails v2](https://wails.io) | 原生 WebView2 渲染，无需额外运行时（Win10/11 自带） |
| 前端框架 | Vue 3 + TypeScript + Vite | 主界面与远程控制共用同一套前端 |
| UI 组件库 | [Naive UI](https://www.naiveui.com) + @vicons/ionicons5 | 暗色主题、数据表格、滑块、弹窗 |
| 音频解码 / 播放 | [gopxl/beep](https://github.com/gopxl/beep) v1.4 | 纯 Go 解码 mp3 / flac / wav / ogg，基于 oto 输出 |
| 标签 / 封面 | [dhowden/tag](https://github.com/dhowden/tag) | ID3 / FLAC / Vorbis 元数据 |
| HTTP 服务 | [gin-gonic/gin](https://github.com/gin-gonic/gin) v1.12 | 本地 API + 前端静态资源 |
| 持久化 | [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | 纯 Go SQLite，无 CGO 依赖 |

## 快速开始

### 环境要求

- Go 1.25+
- Node.js 18+（前端构建）
- Wails CLI：
  ```bash
  go install github.com/wailsapp/wails/v2/cmd/wails@latest
  ```
- Windows 下 Wails 构建需要 C 编译器（WebView2Loader CGO）：
  ```bash
  winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT
  ```

### 运行（开发模式）

```bash
wails dev
```

首次启动后：

1. 点击顶部「导入文件夹」选择音乐目录（或直接修改 `data/config.json` 的 `libraries` 后重启）；
2. 程序自动扫描曲库；
3. 单击歌曲行即播放。

### 构建 exe

```bash
wails build
```

产物在 `build/bin/stack-music.exe`（约 30MB，含全部前端资源，单文件分发）。

### 远程控制

1. 启动播放器（默认端口 8080，可在 `data/config.json` 修改 `port`）；
2. 查看电脑局域网 IP：
   ```bash
   ipconfig
   ```
3. 在手机/其他电脑浏览器打开 `http://<电脑IP>:8080`。

### 界面背景

- 播放条右侧「背景」按钮打开选择面板；
- 内置 4 套渐变背景；点击「从本地添加」导入自己的图片（如动漫壁纸），导入后自动应用；
- 背景保存在 `data/backgrounds/`，选择记录在 `data/config.json`，重启后保持；
- **程序不内置任何受版权保护的图片**，请使用自己拥有授权的内容。

### AI 音乐生成

1. 侧栏进入「AI 音乐」；
2. 选择模式（文字生成 / 歌词生成 / 纯音乐），填写描述或歌词，选风格/情绪/语言/标题；
3. 生成后端默认 **本地合成器**（离线可用）；如需真实模型，在「生成设置」中选择 MiniMax / Suno 并填入接口地址与 API Key（官方或中转均可）；
4. 生成完成后作品自动保存到 **My Music**，在任务列表或曲库中直接回听。

> 使用远端模型时，生成内容的版权与计费归对应平台。

> ⚠️ 远程控制 API **无鉴权**，请仅在可信局域网内使用。

## 目录结构

```
stack-music/
├── main.go                  # Wails 入口：装配 config/store/library/player/server/bindings
├── wails.json               # Wails 配置（名称/产物/前端构建命令）
├── internal/
│   ├── bindings/            # Wails 暴露给前端的 Go 绑定层（播放/曲库/歌单/收藏/歌词/扫描）
│   ├── config/              # 配置加载与持久化（data/config.json）
│   ├── model/               # Song / Playlist / PlayMode 等数据结构
│   ├── store/               # SQLite 数据层（曲库/歌单/收藏/历史）
│   ├── library/             # 音乐文件夹扫描、标签/封面/时长提取
│   ├── player/              # beep 播放引擎封装（队列/模式/进度/音量/事件）
│   ├── gen/                 # AI 音乐生成（多后端 Provider + 本地合成器 + 任务管理器）
│   ├── background/          # 界面背景管理（导入/列表/删除，文件安全校验）
│   ├── server/              # Gin API + SSE + 前端静态资源（浏览器远程控制）
│   └── lyrics/              # LRC 歌词解析
├── frontend/                # Vue 3 + Naive UI 前端（桌面窗口与远程控制共用）
│   ├── src/                 # 页面/组件/双模式 API 层
│   ├── dist/                # 构建产物（go:embed 打进 exe）
│   └── wailsjs/             # wails build 生成的绑定声明
├── docs/                    # 项目文档
└── testdata/music/          # 演示用测试音频与歌词
```

## 数据存储

所有运行数据位于 **exe 同级目录下的 `data/`**（开发模式为项目根目录 `data/`）：

- `config.json` — 端口、音量、播放模式、音乐文件夹列表
- `music.db` — SQLite 数据库（曲库、歌单、收藏、历史）
- `covers/` — 从音频内嵌提取的封面图缓存

## API 摘要

完整文档见 [docs/api.md](docs/api.md)。

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/` | 远程控制页 |
| GET | `/api/status` | 播放器状态（轮询） |
| GET | `/api/events` | SSE 状态推送 |
| GET | `/api/songs?q=` | 曲库列表（支持搜索） |
| POST | `/api/play` | 播放（song_id / playlist_id / queue） |
| POST | `/api/pause` `/api/next` `/api/prev` `/api/seek` `/api/volume` `/api/mode` | 播放控制 |
| GET/POST/DELETE | `/api/playlists...` | 歌单管理 |
| POST | `/api/favorite` / GET `/api/favorites` | 收藏 |
| GET | `/api/history` | 播放历史 |
| GET | `/api/covers/:id` | 封面图 |
| GET/POST | `/api/backgrounds...` | 界面背景（列表/上传/切换/删除） |
| POST | `/api/scan` | 触发曲库扫描 |
| GET/POST | `/api/gen/config` | AI 生成配置（后端/中转地址/Key/模型） |
| POST | `/api/gen/submit` | 提交生成任务 |
| GET | `/api/gen/tasks` | 生成历史与状态 |
| DELETE | `/api/gen/tasks/:id` | 删除生成任务（含入库歌曲） |

## 架构与设计

模块职责、线程模型、播放状态机等详见 [docs/architecture.md](docs/architecture.md)。

## Roadmap

见 [docs/roadmap.md](docs/roadmap.md)。

## License

本项目仅用于学习与技术演示。