// 堆栈音乐播放器主入口（Wails v2 + Vue 3）。
//
// 架构：
//   - Wails 桌面窗口：Go 绑定（internal/bindings）+ 前端（frontend/dist，go:embed）
//   - Go 后端模块：player（beep 播放引擎）/ library / store / config 全部原有逻辑
//   - Gin 局域网远程控制服务：与桌面窗口共享同一套前端（浏览器模式走 HTTP API）
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"go-music/internal/background"
	"go-music/internal/bindings"
	"go-music/internal/config"
	"go-music/internal/library"
	"go-music/internal/player"
	"go-music/internal/server"
	"go-music/internal/store"
)

//go:embed all:frontend/dist
var assets embed.FS

// distFS 嵌入的 frontend/dist 子树（Wails AssetServer 与 Gin 静态服务共用）。
var distFS, _ = fs.Sub(assets, "frontend/dist")

func main() {
	dataDir := config.DataDir()
	cfg := config.Load(dataDir)
	if err := cfg.Save(dataDir); err != nil {
		log.Printf("保存配置失败: %v", err)
	}

	st, err := store.Open(dataDir)
	if err != nil {
		log.Fatalf("打开数据库失败: %v", err)
	}
	defer st.Close()

	lib := library.New(st, dataDir)

	pl, err := player.New()
	if err != nil {
		log.Fatalf("初始化播放器失败: %v", err)
	}

	backend := bindings.New(cfg, st, lib, pl, dataDir)

	// 局域网远程控制服务（浏览器打开 http://IP:端口 复用同一前端）
	srv := server.New(cfg, st, lib, pl, distFS)
	srv.SetGenManager(backend.GenManager())
	srv.SetBackgroundManager(background.NewManager(dataDir))
	go func() {
		if err := srv.Start(); err != nil {
			log.Printf("Web 服务启动失败（端口 %d）: %v", cfg.Port, err)
		}
	}()

	err = wails.Run(&options.App{
		Title:     "堆栈音乐播放器",
		Width:     1120,
		Height:    760,
		MinWidth:  900,
		MinHeight: 600,
		AssetServer: &assetserver.Options{
			Assets: distFS,
		},
		BackgroundColour: &options.RGBA{R: 16, G: 17, B: 20, A: 1},
		OnStartup:        backend.OnStartup,
		OnShutdown: func(ctx context.Context) {
			// 退出前保存配置（音量/模式等）
			cfg.Volume = pl.Volume()
			cfg.Mode = int(pl.Mode())
			_ = cfg.Save(dataDir)
			pl.Close()
		},
		Bind: []interface{}{
			backend,
		},
	})
	if err != nil {
		log.Fatalf("应用启动失败: %v", err)
	}
}
