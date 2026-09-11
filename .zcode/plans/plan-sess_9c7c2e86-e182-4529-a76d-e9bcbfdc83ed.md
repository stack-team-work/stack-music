# AI 音乐生成功能实施计划（v1.1 "AI 音乐"）

## 决策（用户已确认）
1. **模型后端可配置**：支持官方模型与中转站 —— provider 类型（MiniMax 官方 / Suno 官方 / OpenAI 兼容中转 / 本地合成器）+ base_url + api_key + model 均可配置，默认给官方地址
2. **内置本地合成器兜底**：无 Key 也能走通 生成→保存→回听 全流程（和弦/鼓点/旋律的简单 BGM 级）
3. **全量参数**：三种模式（描述生成/歌词生成/纯音乐）+ 10 种风格 + 情绪 + 语言 + 标题

## 一、Go 后端

### internal/gen/（新包）
- `provider.go`：`Provider` 接口 `Submit(req) → remoteTaskID` / `Poll(remoteTaskID) → (status, audioURL, errMsg)`；`Registry` 按配置构造实例
- `minimax.go` / `suno.go`：官方 API 适配（文本/歌词任务提交、轮询、下载 MP3）；`base_url` 可覆盖为中转地址，`api_key`/`model` 可配
- `synthesizer.go`：本地合成器 —— 和弦进行（I–V–vi–IV 等变体）+ 鼓点（kick/snare/hihat）+ 五声音阶旋律游走，风格影响 BPM/音色；手写 16bit/44.1kHz 立体声 WAV 输出（`data/generated/`）
- `manager.go`：后台任务管理器 —— 提交入库（gen_tasks 表）→ goroutine 每 10s 轮询进行中任务 → 完成后下载到 `data/generated/` → 写入 songs 表（album=`My Music`、`source='generated'`）→ 发 `gen:done` 事件；失败发 `gen:failed`

### store
- `songs` 表：`ALTER TABLE ... ADD COLUMN source TEXT DEFAULT 'file'`（兼容旧库，无需迁移脚本）
- 新表 `gen_tasks(id, mode, prompt, lyrics, style, mood, lang, title, provider, model, remote_task_id, status, audio_path, song_id, created_at, updated_at)`
- CRUD：CreateGenTask / UpdateGenTask / GenTasks（历史）/ 删除（级联删歌曲）

### config
- `Gen` 配置段：`provider`、`base_url`、`api_key`、`model`（存 `data/config.json`，不落 git）；`SetGenConfig` 绑定方法运行时更新

### bindings（Wails 前端直调）
- `GetGenConfig` / `SetGenConfig`（Provider 选择 + 中转地址 + Key + 模型名）
- `GenSubmit(req)`（mode/prompt/lyrics/style/mood/lang/title）→ 立即返回任务
- `GenTasks()`（生成历史 + 实时状态）、`GenDeleteTask(id)`（删任务+删歌曲）

### server（浏览器远程模式同构）
- `GET/POST /api/gen/config`、`POST /api/gen/submit`、`GET /api/gen/tasks`、`DELETE /api/gen/tasks/:id`
- 复用现有 SSE/轮询通道推送 gen 事件

## 二、前端（frontend/）

- `store.ts`：`ViewName` 增加 `'gen'`；AppSide 加「AI 音乐」导航（SparklesOutline 图标）；App.vue 注册 GenView
- **`views/GenView.vue`**：
  - **设置区**：Provider 下拉（MiniMax / Suno / 中转 / 本地合成器）+ BaseURL + API Key + 模型名输入（可折叠）
  - **生成表单**：模式 Tab（描述文字 / 歌词 / 纯音乐）；textarea；风格 10 选（Pop/Hip Hop/Rock/Electronic/Acoustic/R&B/Jazz/Classical/Country/Lofi）；情绪（欢快/伤感/激昂/平静/浪漫/神秘）；语言（中文/英文/日文/韩文/粤语/无）；标题输入；生成按钮（提交中 loading）
  - **任务列表**：自动轮询（Wails 事件 + 2s 兜底），状态标签 排队中/生成中/已完成/失败，完成后 message 提示
  - **My Music 区**：生成历史（标题/风格/时间），单击回听（现有播放链路），「重新生成」回填表单改参数再生成，「删除」
  - 本地合成器模式：不用 Key 直接生成，供无 Key 演示
- 生成完成自动入库 → 曲库搜索"My Music"可见（现有搜索匹配 album ✓）

## 三、参数映射
- 风格/情绪/语言 → 各 Provider 字段映射表（minimax 的 music_style/mood/language 等），中转按所选官方兼容形态透传
- 本地合成器：风格→BPM/音色/和弦走向映射

## 四、实施顺序
1. store：songs.source 列 + gen_tasks 表 + CRUD + 测试
2. internal/gen：Provider 接口 + MiniMax/Suno 适配 + 本地合成器（合成器先完成，可离线验证全链路）
3. manager 后台任务轮询 + 入库 + 事件
4. bindings + server 的 gen API
5. 前端 GenView（设置/表单/任务列表/My Music）+ 导航
6. npm build + wails build
7. 回归：本地合成器端到端（生成→入库→播放/回听→重生成）；配置错误处理（无 Key 提示）；HTTP 模式同构验证；go vet/test 全绿
8. 文档同步（README/architecture/api/roadmap + 模型配置说明）

## 五、风险与说明
- **真模型需真实 Key**：无法自动化端到端验证官方 API；验证到"参数透传 + 错误处理清晰"（无 Key/地址不可达时给出明确提示），真 Key 验证由用户配置后进行
- **中转形态不统一**：适配"MiniMax 官方 / Suno 官方"两类任务式接口，兼容这两类的中转直接可用；其他私有形态需按文档微调（计划中明确此边界）
- **版权与费用**：生成内容版权与计费归所选平台，界面与文档注明
- 生成音乐默认为 MP3（官方）/WAV（本地合成器），播放链路均已支持