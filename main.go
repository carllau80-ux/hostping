// DBPing — 多数据库巡检与拓扑工具（纯 Go 驱动，单可执行文件）。
package main

import (
	"embed"
	"log"

	"dbping/app"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	a := app.New()
	err := wails.Run(&options.App{
		Title:            "HostPing v" + app.AppVersion + " — 主机巡检与分析",
		Width:            1400,
		Height:           900,
		MinWidth:         1080,
		MinHeight:        680,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 243, G: 246, B: 250, A: 255},
		OnStartup:        a.Startup,
		OnShutdown:       a.Shutdown,
		Bind:             []interface{}{a},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
