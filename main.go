package main

import (
	"embed"
	"fmt"
	"github.com/sqweek/dialog"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"ollama-desktop/internal/app"
	"ollama-desktop/internal/config"
	"ollama-desktop/internal/log"
	"os"
	"runtime"
	"time"
)

//go:embed all:frontend/dist
var assets embed.FS

// 初始化时区
func init() {
	timeZone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Printf("Load time zone error: %s\n", err.Error())
		return
	}
	time.Local = timeZone
}

func main() {
	log.Info().Str("BuildHash", config.BuildHash).Str("BuildVersion", config.BuildVersion).
		Str("Arch", runtime.GOARCH).Str("OS", runtime.GOOS).Str("GoVersion", runtime.Version()).
		Msg("Ollama Desktop")

	err := app.StartApp(&assetserver.Options{
		Assets: assets,
	})

	if err != nil {
		dialog.Message("启动应用失败(%+v)", err).Title("异常").Error()
		log.Error().Err(err).Msg("Run Ollama Desktop Error")
		os.Exit(1)
	}
}
