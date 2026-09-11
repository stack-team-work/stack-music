# REST API 文档

服务监听 `0.0.0.0`，默认端口 **8080**（`data/config.json` 的 `port` 可改）。

- Base URL：`http://<电脑IP>:8080`
- 所有接口均返回 JSON；错误统一为 `{"error": "..."}`
- **无鉴权**，仅供可信局域网使用

---

## 状态与事件

### GET /api/status

播放器完整状态快照（远程控制页 1s 轮询）。

```json
{
  "state": "playing",          // playing | paused | stopped
  "position": 12.5,            // 当前播放位置（秒）
  "duration": 210.0,           // 当前歌曲时长（秒）
  "volume": 80,                // 0-100
  "mode": 1,                   // 0 顺序 1 列表循环 2 单曲循环 3 随机
  "mode_text": "列表循环",
  "current": { ...songResp },  // null 表示未播放
  "queue_pos": 2,
  "queue_len": 10,
  "scanning": false,
  "song_count": 123,
  "library_roots": ["E:/music"],
  "covers_url": "/api/covers/5"
}
```

`songResp` 结构：

```json
{
  "id": 5, "title": "晴天", "artist": "周杰伦", "album": "叶惠美",
  "genre": "Pop", "year": 2003, "duration": 269.2, "format": "mp3",
  "favorited": false, "cover_url": "/api/covers/5"
}
```

### GET /api/events （SSE）

Server-Sent Events。连接后立即推送一次完整状态，之后在播放事件（切歌/暂停/播放）或每 20s 心跳推送。

```
data: {"state":"playing",...}

```

---

## 曲库

### GET /api/songs?q=关键词

曲库列表，`q` 可选（匹配标题/艺术家/专辑）。

```json
{ "songs": [ { ...songResp } ], "total": 123 }
```

### DELETE /api/songs/:id

从曲库移除歌曲（级联清理歌单/收藏/历史）。

### POST /api/scan

后台触发曲库扫描（扫描 `library_roots` 中的目录）。

```json
{ "ok": true, "scanning": true }
```

### GET /api/library

```json
{ "roots": ["E:/music"], "song_count": 123, "scanning": false }
```

---

## 播放控制

### POST /api/play

三种用法：

| 请求体 | 行为 |
|---|---|
| `{"song_id": 5}` | 播放单曲 |
| `{"playlist_id": 2, "song_id": 5}` | 以整个歌单为队列播放，从 song_id 处开始 |
| `{"queue": [3,2,1], "start": 1}` | 以指定 ID 列表为队列播放 |

响应为最新 status。

### POST /api/pause · /api/resume · /api/toggle

暂停 / 继续 / 切换。

### POST /api/next · /api/prev

下一首 / 上一首（超过 3 秒则回开头）。

### POST /api/seek

```json
{ "position": 100.5 }   // 秒
```

### POST /api/volume

```json
{ "volume": 60 }        // 0-100
```

### POST /api/mode

```json
{ "mode": 1 }           // 0 顺序 1 列表循环 2 单曲循环 3 随机
```

### GET /api/queue

```json
{ "pos": 1, "songs": [ { ...songResp } ] }
```

---

## 收藏与历史

### POST /api/favorite

```json
{ "song_id": 5, "favorite": true }    // favorite 省略时为收藏
```

### GET /api/favorites

```json
{ "songs": [ { ...songResp } ] }
```

### GET /api/history

最近播放（去重，最多 50 条）。

```json
{ "songs": [ { ...songResp } ] }
```

---

## 歌单

### GET /api/playlists

```json
{ "playlists": [ { "id": 1, "name": "我的最爱", "created_at": 1725000000, "song_count": 20 } ] }
```

### POST /api/playlists

```json
{ "name": "新歌单" }   →  { "ok": true, "id": 3 }
```

### DELETE /api/playlists/:id

删除歌单（歌曲本身不受影响）。

### GET /api/playlists/:id/songs

```json
{ "songs": [ { ...songResp } ] }
```

### POST /api/playlists/:id/songs

```json
{ "song_id": 5 }    // 追加到歌单末尾，重复添加被忽略
```

### DELETE /api/playlists/:id/songs/:songId

从歌单移除并自动重排顺序。

---

## 歌词

### GET /api/lyrics/:id

返回歌曲 LRC 歌词行（无歌词返回空数组）。

```json
{ "lines": [ { "time": 1.0, "text": "第一句" }, { "time": 3.0, "text": "第二句" } ] }
```

## AI 音乐生成

### GET /api/gen/config

读取生成配置（含各后端默认地址/模型）。

```json
{
  "provider": "local",            // local / minimax / suno
  "base_url": "",
  "api_key": "",
  "model": "",
  "defaults": {
    "minimax": { "base_url": "https://api.minimax.chat/v1/music_generation", "model": "music-01" },
    "suno": { "base_url": "https://api.sunoapi.org/api/v1/generate", "model": "chirp-v3-5" }
  }
}
```

### POST /api/gen/config

保存生成配置（支持官方或中转地址）：

```json
{ "provider": "minimax", "base_url": "https://…", "api_key": "sk-…", "model": "music-01" }
```

### POST /api/gen/submit

提交生成任务，立即返回任务对象（status=pending，后台异步生成）。

```json
{
  "mode": "text",              // text / lyrics / instrumental
  "prompt": "夏日晚风的海边",    // text/instrumental 模式必填
  "lyrics": "",                // lyrics 模式必填（≤3000 字）
  "style": "Lofi",             // Pop/Hip Hop/Rock/Electronic/Acoustic/R&B/Jazz/Classical/Country/Lofi
  "mood": "平静",               // 欢快/伤感/激昂/平静/浪漫/神秘
  "lang": "中文",               // 中文/英文/日文/韩文/粤语/纯音乐
  "title": "夏夜的栈道"          // 可空，自动命名
}
```

未配置 API Key 且使用远端后端时返回 `400 {"error":"未配置 API Key：…"}`。

### GET /api/gen/tasks

生成历史（新→旧），status ∈ pending / queued / generating / done / failed：

```json
{ "tasks": [ { "id": 1, "title": "夏夜的栈道", "status": "done", "song_id": 51, "audio_path": "…", "fail_reason": "" } ] }
```

### DELETE /api/gen/tasks/:id

删除生成任务：级联删除入库歌曲与本地音频文件。

## 封面

### GET /api/covers/:id

返回歌曲封面图（`image/jpeg` 或 `image/png`，无封面返回 404）。

---

## 固定页面

### GET /

远程控制页（自包含单文件 HTML+JS，已嵌入 exe，无需联网资源）。