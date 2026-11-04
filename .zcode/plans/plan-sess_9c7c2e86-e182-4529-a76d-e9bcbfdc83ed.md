# 项目改名：go-music → stack-music（显示名 Stack Music）

## Context
项目当前所有标识（Go module、import 路径、Wails 配置、exe 产物名、窗口/页面/侧栏标题、文档）都使用 `go-music` / 「堆栈音乐播放器」。用户要求统一改名为 **stack-music**，应用显示名使用英文 **Stack Music**。运行数据（`data/`、曲库、歌单、背景、AI 生成记录）均与项目名无关，改名不会丢失或迁移任何数据。

## 改动清单（探索已确认全部位置）

### 1. Go 模块与 import
- `go.mod:1`：`module go-music` → `module stack-music`
- 全仓 **44 处** import 批量替换：`go-music/internal/...` → `stack-music/internal/...`
  - 分布：`main.go` 7、`internal/bindings` 12、`internal/server` 14、`internal/gen` 4、`internal/store` 3、`internal/library` 2、`internal/lyrics` 1、`internal/player` 1
- 执行 `go mod tidy`；`frontend/wailsjs` 生成代码无模块路径，无需手改

### 2. Wails 配置（`wails.json`）
- `name` / `outputfilename` / `author.name` / `info.copyright` → `stack-music`
- `author.email` → `stack-music@local`
- `info.productName` → `Stack Music`
- `productVersion` 保持 `1.2.1`（仅改名，不升功能版本）

### 3. 显示名称
| 文件 | 改动 |
|---|---|
| `main.go:67` | 窗口标题 `堆栈音乐播放器` → `Stack Music`；文件头注释同步 |
| `frontend/index.html:6` | `<title>Stack Music</title>` |
| `frontend/src/components/AppSide.vue:21-22` | 品牌主文字 `Stack Music`，副标题改为 `Music Player`（避免与主文字重复） |
| `frontend/src/style.css:1` | 注释项目名同步（可选） |
| `internal/server/routes_test.go:15,47-48` | 测试桩与断言 `堆栈音乐播放器` → `Stack Music`（**不同步会导致 go test 失败**） |

### 4. 文档与路径
- `README.md`：标题 `# Stack Music`；产物 `build/bin/stack-music.exe`；目录树根 `stack-music/`；版本行 `v1.2.0` 修正为 `v1.2.1`
- `docs/build.md:53,74,80`：3 处 `go-music.exe` → `stack-music.exe`；自检项窗口标题
- `docs/architecture.md:7`：架构图 `go-music.exe` → `stack-music.exe`
- `docs/roadmap.md`：历史条目保留原名并标注「（现更名 Stack Music）」，不改写历史

### 5. 其他
- `internal/gen/synth.go:22`：临时目录前缀 `gomusic-gen-*` → `stack-music-gen-*`
- 构建后清理旧产物：`build/bin/go-music.exe`、`build/go-music.exe`
- **磁盘目录保持 `E:\my_project\go-music` 不变**（当前会话工作目录依赖它；如需文件夹也改名，关闭会话后重命名即可，功能与数据不受影响）

## 验证
1. 残留检查：`grep -rn "go-music"`（排除 `build/`、`node_modules`、`dist`、`.git`）应为空；`堆栈音乐播放器` 仅保留 roadmap 历史标注
2. `go build ./... && go vet ./... && go test ./...` 全绿（含 routes_test 断言）
3. `vue-tsc --noEmit` + `npm run build` 通过
4. `wails build` → 产物 `build/bin/stack-music.exe`
5. 启动验证（8090 端口）：窗口标题 `Stack Music`、侧栏品牌、曲库 52 首、播放正常、AI 音乐/背景页正常；浏览器打开 8090 标题同步
6. 数据核对：曲库、歌单、收藏、生成历史与改名前一一致（data 目录未动）

## 风险
- module 改名遗漏 import 会导致编译失败 → grep 残留检查 + 全量 build 兜底
- 旧 exe 进程需先停止再构建，避免文件占用
- 纯命名性改动，不涉及业务逻辑，回归风险低