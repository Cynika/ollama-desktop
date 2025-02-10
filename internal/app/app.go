package app

import (
	"context"
	"github.com/hashicorp/go-version"
	"github.com/sqweek/dialog"
	"github.com/wailsapp/wails/v2/pkg/options"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"ollama-desktop/internal/config"
	"ollama-desktop/internal/job"
	"ollama-desktop/internal/log"
	"ollama-desktop/internal/util"
	"os"
	"runtime"
	"strings"
	"time"
)

var app = App{}

type App struct {
	ctx         context.Context
	lastVersion *util.Item
}

func (a *App) startup(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			dialog.Message("初始化应用失败(%+v)", r).Title("异常").Error()
			os.Exit(1)
		}
	}()
	log.Info().Ctx(ctx).Msg("Ollama Desktop startup...")
	a.ctx = ctx
	dao.startup(ctx)
	job.GetSchedule().AddFunc("0/10 * * * * ?", ollama.Heartbeat)
	go a.checkUpgrade()
}

func (a *App) checkUpgrade() {
	current, err := version.NewVersion(config.BuildVersion)
	if err != nil {
		log.Warn().Err(err).Msg("check app upgrade")
		return
	}
	for {
		select {
		case <-a.ctx.Done():
			return
		default:
			client := createHttpClient()
			var releases = []util.Release{&util.GithubRelease{Http: client}, &util.GiteeRelease{Http: client}}
			hasErr := false
			for _, release := range releases {
				item, err := release.Last("jianggujin", "ollama-desktop")
				if err != nil {
					log.Warn().Err(err).Str("channel", release.Channel()).Msg("check app upgrade")
					hasErr = true
					continue
				}
				if item == nil {
					continue
				}
				last, err := version.NewVersion(strings.ToLower(item.Name))
				if err != nil {
					log.Warn().Err(err).Str("channel", release.Channel()).Str("name", item.Name).Msg("check app upgrade")
					hasErr = true
					continue
				}
				log.Info().Str("channel", release.Channel()).Str("name", item.Name).Msg("check app upgrade")
				// 存在新版本，提示升级
				if last.GreaterThan(current) {
					a.lastVersion = item
					wailsruntime.EventsEmit(app.ctx, "appUpgrade", item)
				}
				return
			}
			// 未发生异常，表示正常检测完成
			if !hasErr {
				return
			}
		}
		// 如果请求失败，等待5秒后重试
		time.Sleep(5 * time.Minute)
	}
}

func (a *App) domReady(ctx context.Context) {
	log.Info().Ctx(ctx).Msg("Ollama Desktop domReady...")
	ollama.Heartbeat()
}

func (a *App) shutdown(ctx context.Context) {
	log.Info().Msg("Ollama Desktop shutdown...")
	dao.shutdown()
	job.GetSchedule().Stop()
}

func (a *App) onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	secondInstanceArgs := secondInstanceData.Args

	log.Debug().Str("Args", strings.Join(secondInstanceData.Args, ",")).Msg("user opened second instance")
	wailsruntime.WindowUnminimise(a.ctx)
	wailsruntime.Show(a.ctx)
	go wailsruntime.EventsEmit(a.ctx, "launchArgs", secondInstanceArgs)
}

func (a *App) AppInfo() map[string]string {
	shortHash := config.BuildHash
	if len(shortHash) > 7 {
		shortHash = shortHash[0:7]
	}
	lastVersionName := ""
	lastVersionBody := ""
	lastVersionUrl := ""
	if a.lastVersion != nil {
		lastVersionName = a.lastVersion.Name
		lastVersionBody = a.lastVersion.Body
		lastVersionUrl = a.lastVersion.Url
	}
	return map[string]string{
		"Version":        config.BuildVersion,
		"BuildHash":      config.BuildHash,
		"BuildShortHash": shortHash,
		"Platform":       runtime.GOOS,
		"Arch":           runtime.GOARCH,
		"LastName":       lastVersionName,
		"LastBody":       lastVersionBody,
		"LastUrl":        lastVersionUrl,
	}
}
