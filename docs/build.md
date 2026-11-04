# Windows 构建指南

## 1. 环境准备

### 1.1 Go

要求 Go **1.25+**：

```bash
go version
```

### 1.2 Node.js

前端（Vue 3 + Vite）构建需要 Node.js **18+** 与 npm：

```bash
node --version
npm --version
```

### 1.3 Wails CLI

```bash
go install github.com/wailsapp/wails/v2/cmd/wails@latest
wails version   # v2.15+
```

### 1.4 C 编译器

Wails v2 的 Windows WebView2 加载器（CGO）需要 gcc：

```bash
winget install -e --id BrechtSanders.WinLibs.POSIX.UCRT --accept-source-agreements --accept-package-agreements
gcc --version
```

> Git Bash 用户若新终端找不到 gcc，把 WinGet 安装目录下的 `mingw64/bin` 加入 `~/.bashrc` 的 PATH。

### 1.5 WebView2 Runtime

Windows 10/11 系统自带，无需安装。

## 2. 构建 exe

```bash
wails build
```

流程：`wails build` 会自动执行 `npm install` → `npm run build`（vue-tsc 类型检查 + vite 产物到 `frontend/dist`）→ 生成绑定声明（`frontend/wailsjs/`）→ CGO 编译并嵌入前端 → 产出：

```
build/bin/stack-music.exe   # 约 30MB，单文件，含全部前端资源
```

### 2.1 开发热重载

```bash
wails dev
```

前端改动即时生效（Vite dev server），Go 绑定改动自动重编译。

### 2.2 只构建前端

```bash
cd frontend && npm install && npm run build
```

### 2.3 压缩体积（可选）

```bash
winget install -e --id UPX.UPX
upx --best build/bin/stack-music.exe   # 可压到约 10MB
```

## 3. 运行与验证

```bash
./build/bin/stack-music.exe
```

自检清单：

1. 窗口标题「Stack Music」出现，WebView2 渲染无白屏；
2. 侧栏四个页面可切换，曲库表格有数据；
3. `curl http://127.0.0.1:8080/api/status` 返回 JSON；
4. 浏览器打开 `http://127.0.0.1:8080/` 显示与桌面端一致的界面，可控制播放。

## 4. 常见问题

| 现象 | 原因与解决 |
|---|---|
| `cgo: C compiler "gcc" not found` / `math: SquareRoot of negative` 类 CGO 报错 | 未安装 gcc 或不在 PATH，见 1.4；执行 `go env -w CGO_ENABLED=1` |
| WebView2 窗口白屏 | 前端构建产物缺失：先 `cd frontend && npm run build` 再 `wails build` |
| 端口被占用：`bind: Only one usage...` | 修改 `data/config.json` 的 `port`，或结束占用进程 |
| 远程页面打不开 | 确认防火墙放行、同一网段、访问电脑 IP 而非 127.0.0.1 |
| 杀软误报 | 未签名的大体积 Go exe 常见误报；可用 `signtool` 签名 |
| npm 安装慢/失败 | 配置国内镜像：`npm config set registry https://registry.npmmirror.com` |
| `wails build` 报 asset 相关错误 | 确认 `frontend/dist` 存在且含 `index.html`（build 失败可手动 `npm run build` 后重试） |

## 5. 测试

```bash
go test ./...
go vet ./...
cd frontend && npx vue-tsc --noEmit
```