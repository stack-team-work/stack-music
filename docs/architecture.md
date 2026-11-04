# 架构设计

## 1. 总体架构

```
┌──────────────────────────────────────────────────────────────────┐
│                        stack-music.exe                              │
│                                                                  │
│  ┌─────────────────────────┐        ┌──────────────────────────┐ │
│  │  Wails 桌面窗口          │        │  Gin Server (8080)       │ │
│  │  (WebView2 渲染)        │        │  REST API + SSE          │ │
│  │  ┌───────────────────┐  │        │  + 前端静态资源           │ │
│  │  │ Vue3 + Naive UI   │  │ 绑定   │  (同一套 frontend/dist)  │ │
│  │  │  (frontend/dist)  │◄─┼───────┼──►                       │ │
│  │  └─────────┬─────────┘  │        └──────────┬───────────────┘ │
│  │            │ 双模式     │                   │                 │
│  │            │ (Wails绑定 │ HTTP API          │                 │
│  │            │  / HTTP)   │                   │                 │
│  └────────────┼────────────┘                   │                 │
│               ▼                                 ▼                 │
│  ┌─────────────────── Bindings 层（internal/bindings）────────┐  │
│  │  Status/Play/Seek/歌单/收藏/历史/歌词/扫描/封面/导入        │  │
│  └───────────────┬───────────────────────────────────────────┘  │
│                  ▼                                              │
│  ┌──────────┐   ┌────────────┐   ┌───────────────────────┐      │
│  │ Player   │   │ Library    │   │ Store (SQLite)        │      │
│  │ beep 引擎 │   │ 扫描/标签   │   │ 曲库/歌单/收藏/历史      │      │
│  └──────────┘   └────────────┘   └───────────────────────┘      │
│                                                                  │
│  data/  config.json · music.db · covers/                         │
└──────────────────────────────────────────────────────────────────┘
            ▲
   局域网浏览器 / 手机（http://<IP>:8080）→ 同一套 Vue 界面（HTTP 模式）
```

**双模式前端**是核心设计：`frontend/dist` 同时被
- Wails AssetServer 加载（桌面窗口，`window.go.bindings.Backend.*` 直调 Go 绑定，事件走 `window.runtime`）
- Gin 静态服务加载（浏览器，`fetch /api/*` + SSE）

前端启动时探测 `window.go` 是否存在自动切换模式，业务组件无感知。

进程内线程：WebView2 UI 线程（前端 JS）↔ Go 绑定调用 ↔ beep 播放协程 ↔ Gin 服务协程（goroutine） ↔ 扫描协程。共享状态由 `sync.Mutex` 保护（player 内部），跨线程只传数据不共享对象。

## 2. 模块职责

| 模块 | 职责 | 关键点 |
|---|---|---|
| `internal/config` | 配置的加载与保存 | 数据目录取 exe 同级 `data/`，开发模式退回当前目录 |
| `internal/model` | 数据结构 | `Song`（含时长/哈希/封面标记）、`Playlist`、`PlayMode` 枚举 |
| `internal/store` | SQLite 数据层 | 表：`songs`、`favorites`、`playlists`、`playlist_songs`、`history`；纯 Go 驱动 `modernc.org/sqlite` |
| `internal/library` | 音乐库扫描 | 增量策略（path + mod_time + size）；标签与封面提取（dhowden/tag）；时长探测；封面缓存到 `data/covers/{hash}.jpg` |
| `internal/player` | 播放引擎 | 队列、四种模式、进度、音量、事件通道 |
| `internal/bindings` | Wails 绑定层 | 前端直调的 Go 方法（播放/曲库/歌单/收藏/历史/歌词/封面/扫描/导入）；播放器事件转 `runtime.EventsEmit` 推给前端；扫描进度推送 |
| `internal/server` | HTTP 服务 | REST API、SSE、前端静态资源（`fs.FS` 注入，与 Wails 共用 `frontend/dist`） |
| `internal/lyrics` | LRC 解析 | `[mm:ss.xx]` 时间戳、offset 元数据、按时间排序 |
| `internal/gen` | AI 音乐生成 | Provider 抽象（MiniMax/Suno 官方或中转，base_url/key/model 可配）+ 本地合成器（离线兜底）+ Manager（提交/轮询/下载/入库/事件），失败重试与 30 分钟超时 |
| `internal/background` | 界面背景 | 用户图片导入/列表/删除（白名单扩展名、≤15MB、UUID 命名防路径穿越）；内置渐变由前端 CSS 提供 |
| `frontend/` | Vue3 + Naive UI | 双模式 API 层（`src/api/`）；页面：曲库/歌单/收藏/历史；组件：歌曲表格/播放条/歌词面板 |

## 3. 播放器设计（internal/player）

### 3.1 状态机

```
             Toggle/Play               自然播完（顺序模式播完最后一首）
  Stopped ─────────────►  Playing  ────────────────────────────► Stopped（seek 回 0）
     ▲                        │  │
     │         Pause/Toggle   ▼  │ Play/Toggle
     └────────────────────  Paused
```

### 3.2 播放队列与事件

- `PlayQueue(songs, start)`：整体替换播放队列并播放；UI 单击列表行会以**整个当前列表**为队列播放，保证上一首/下一首行为符合直觉；
- 队列索引、当前歌曲等状态由 `sync.Mutex` 保护；
- 播放器通过 `Events() <-chan Event` 推送 `EvSong`（切歌）、`EvState`（播放状态）、`EvError`（解码失败）；UI 与 SSE 各自消费；
- 每首歌一次播放会话带**代际号（gen）**，切歌时旧流的播完回调携带旧代际，被自动忽略，避免"切歌误触自动下一首"；
- 自动切歌逻辑由 `finishLoop` 协程响应 `doneCh` 完成，四种模式：
  - 顺序：播放完列表末尾即停止；
  - 列表循环：末尾回到开头；
  - 单曲循环：重播当前；
  - 随机：随机取一首（避免立即重复）。

### 3.3 流生命周期

```
openStream(文件) → interruptible(可取消包装) → [beep.Resample(统一采样率)]
      → gainStream(原子音量) → beep.Ctrl(暂停控制)
      → speaker.Play(Seq(ctrl, Callback(done)))
```

- **采样率统一**：speaker 固定以 44100Hz 输出（`speaker.Init(44100, ...)`），凡源采样率 ≠ 44100（如 48kHz mp3、96kHz FLAC/WAV）必须先经 `beep.Resample(4, src, 44100, ...)` 重采样。若缺失，音频会**变速变调**（96kHz 音频将慢 2.18 倍、音调降低一个多八度）——这是 v1.0 第一个用户报告的解码类缺陷，已在 v1.0.1 修复；
- **切歌**：先解除旧流暂停再关闭其 cancel 通道，旧流在下一次拉取时返回 EOF，`Seq` 结束自动从混音器移除并关闭文件句柄，无音频重叠也无句柄泄漏；
- **seek**：直接 seek 原始 `StreamSeekCloser`（不经重采样器）；重采样器无 Reset API，seek 后内部缓冲残留 <1ms 旧样本，人耳不可闻；
- **进度**：`Position()` 在 `speaker.Lock` 下读取采样位置换算时间；
- **音量**：`gainStream` 内部为 `atomic.Uint64`（Float64bits），speaker 协程与设置协程无锁竞争。

## 4. 曲库扫描（internal/library）

1. `filepath.WalkDir` 收集目录下 `mp3/flac/wav/ogg` 文件（跳过隐藏目录）；
2. 与数据库记录的 `(path, mod_time)` 比对，未变化的文件直接跳过（增量）；
3. 新文件/变更文件：提取标签 → 内嵌封面写缓存 → 探测时长 → upsert；
4. 删除"库目录内文件已消失"的旧记录（避免越权清理库外路径，`withinRoots` 校验）。

### 时长探测策略

| 格式 | 策略 |
|---|---|
| mp3 | 优先解析首帧 **Xing/Info 头**（快速）；缺失则全量解码 |
| flac | 解析 **STREAMINFO** 元数据块（总采样数 / 采样率，快速） |
| wav | 解析 `fmt` 块字节率 + `data` 块大小（快速） |
| ogg | 全量解码探测（vorbis.Decode + Len） |

## 5. HTTP 服务（internal/server）

- 路由：`/api/*` 为 JSON API；其余路径由 `NoRoute` 交给 `http.FileServer` 服务内嵌静态页；
- `/api/status` 为无状态轮询快照；`/api/events` 为 SSE，连接后立即推送一次，之后在播放事件或 20s 心跳时推送；
- 服务端通过 `Controller` 接口（`server.go` 定义）使用播放器能力子集，不依赖具体实现；
- **安全说明**：API 无鉴权，绑定 `0.0.0.0`，仅建议在可信局域网使用。

## 6. 前端架构（frontend/）

- **双模式 API 层**（`src/api/index.ts`）：启动时探测 `window.go.bindings.Backend` 存在与否：
  - Wails 模式：`wailsCall` 直调绑定 + `window.runtime.EventsOn` 订阅 `player:event` / `scan:progress` / `scan:done`
  - HTTP 模式：`fetch /api/*` + EventSource SSE；封面返回相对 URL
  - 两模式接口签名一致，页面组件无感知
- **状态**：模块级 `reactive` 全局 store（播放状态、当前浏览列表、视图、scanVersion 扫描版本号）；播放状态 500ms 轮询 + 事件即时刷新
- **列表浏览**：歌曲表格使用 `flex-height`（自适应容器高度）+ 表格内部滚动浏览全部歌曲；`scanVersion` 变化（扫描完成）时各视图自动重载；滚动条全局加亮 + 表格卡片化（边框/底色/圆角），滚动区域边界明确（v1.0.2 视觉反馈设计）
- **扫描闭环**：扫描进度事件 → 状态栏指示；`scan:done`（含新增/更新/移除计数）→ 弹提示 + 视图重载
- **进度条**：拖动中本地值优先，松手后 `seek`；程序刷新与用户拖动用局部 ref 隔离
- **播放即队列**：单击任一行以"当前浏览列表"为队列播放，上一首/下一首行为与桌面语义一致
- 图标全部来自 @vicons/ionicons5（内联 SVG），无外部网络依赖，离线可用

## 7. 数据模型

```sql
songs(id, path UNIQUE, title, artist, album, genre, year, track,
      duration, format, size, mod_time, hash, has_cover)
favorites(song_id PK → songs.id ON DELETE CASCADE)
playlists(id, name, created_at)
playlist_songs(playlist_id, song_id, position, PK(playlist_id, position))
history(id, song_id, played_at)   -- 保留最近 200 条
```

- 歌曲删除级联清理收藏/歌单/历史引用（外键约束开启）；
- 封面以 `hash = sha1(path)[:16]` 命名缓存，防止路径含特殊字符。

## 8. 关键工程决策记录

| 决策 | 原因 |
|---|---|
| 用 `gopxl/beep` 而非 `faiface/beep` | faiface 已归档，gopxl 为维护分支；v1.4 中 `ogg` 包更名为 `vorbis` |
| 播放前统一重采样到 44100 | speaker 采样率固定，48k/96k 音源不重采样会变速变调（用户实测反馈的缺陷） |
| 自写 `gainStream`（atomic）而非 beep/volume | beep v1.4 的 volume 移入 effects，字段非并发安全，原子包装更简单 |
| 切歌用 cancel 通道 + gen 代际号 | 解决"旧流仍在混音器导致重叠播放/句柄泄漏/误触发自动下一首"三连问题 |
| UI 从 Fyne 迁移到 Wails v2 + Vue3 + Naive UI | Fyne 界面表现力差、依赖 gcc/OpenGL、体积 80MB；Wails 用系统 WebView2（无分发负担），前端自由度高，exe 30MB |
| 桌面与远程控制共用同一套前端（双模式） | 一套代码两处生效：Wails 内调绑定、浏览器调 HTTP；体验一致且维护量最小 |
| 绑定命名空间手写调用 `window.go.bindings.Backend.*` | 与 wails build 生成的 wailsjs 声明一致，无需在源码里依赖生成文件 |
| 绑定层返回错误用 error 返回值 | Wails v2 会把 error 转成前端 Promise reject，调用方可统一 catch |
| 歌曲列表用整表滚动而非分页 | 播放器场景下滚动浏览更符合直觉（v1.0.2 迭代结论）；数据量极大时再引入虚拟滚动 |
| 扫描完成事件驱动前端重载 + 提示 | 曾经用户"刷新后看不到新歌"——扫描完成仅刷状态不刷列表；现以 scanVersion 触发各视图 reload |
| 服务端静态资源强制 no-store | 曾出现浏览器缓存旧前端导致界面与后端不一致（排查两轮）；禁缓存一劳永逸 |
| 索引版本指纹（侧栏 v1.0.x） | 让新旧版本肉眼可辨，配合 no-store 快速确认加载的就是最新版 |
| 表格卡片化 + 滚动条加亮 | 深色主题下界面元素边界模糊曾误导用户以为数据缺失；视觉反馈优先于功能改动 |
| AI 生成后端做成可配置 Provider | 官方 API 与第三方中转的接口形态各异且会变化，base_url/key/model 全部开放配置，字段映射集中在 `rest.go` 三个函数 |
| 内置本地合成器作为兜底 | 没有 Key 也能端到端跑通生成→入库→回听（可自动化验证）；远端模型只需换 Provider，不阻塞主流程 |
| 生成任务全部落库（gen_tasks 表） | 支持历史回看、参数回填重生成、失败原因追踪；删除任务级联清理入库歌曲与音频文件 |
| 生成歌曲 source='generated' 标记 + My Music 专辑 | 与导入文件区分（重扫描不受影响），曲库搜索可按专辑/艺术家过滤 |
| 背景系统不内置版权图片，只做"内置渐变 + 用户导入" | 动漫壁纸（火影等）属版权作品，不能随程序分发；用户导入自己拥有授权的图片合法且灵活 |
| 背景列表在 Wails 模式返回绝对 URL | 桌面窗口 origin 与 127.0.0.1 服务不同源；img/background-image 加载无需 CORS，但需要可解析的绝对地址 |
| 背景层上方叠加白色遮罩 | 图片背景对比度不可控，浅色主题下用 72% 白遮罩 + 轻模糊保证文字可读性 |
| 歌曲表格标题列不设宽度 | Naive UI DataTable 无宽度列自动分配剩余空间，实现列宽总和始终等于容器宽度（解决宽屏右侧留白） |
| 布局容器用显式 `<aside>/<main>/<footer>` 包裹多根组件 | Vue 3 多根组件不会透传父级传入的 class，直接 `<AppSide class="app-side" />` 会导致 grid 定位失效、布局错乱（v1.2.1 修复的根因） |
| SongTable 内部自带 wrapper 撑满 | 不依赖调用方传 class 或 Naive UI 的 class 透传，组件自身保证任意页面下铺满 |
| `r.GET("/")` 直接读嵌入 HTML 而非 FileFromFS | Go 1.25 的 FileServer 将 `/index.html` 规范化为 301，FileFromFS 内部把路径设为 `index.html` 必触发该重定向 |
| 静态页走 `NoRoute` 而非 `StaticFS("/")` | `/*filepath` 通配与 `/api/*` 路由冲突导致 panic |
| embed 后 `fs.Sub("frontend/dist")` 再传 Wails/Gin | `all:frontend/dist` 的 embed 根是 `frontend/`，直接按 `index.html` 查找会 miss |
| SQLite join 查询列名带表别名 | 多表 join 时裸列名 `id` 歧义报错 |