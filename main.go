package main

import (
	"context"
	"embed"
	"fmt"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	"log"
	"res-downloader/core"
	"runtime"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var icon []byte

//go:embed build/windows/icon.ico
var iconIco []byte

//go:embed wails.json
var wailsJson string

func main() {
	// Create an instance of the app structure
	app := core.GetApp(assets, wailsJson)
	bind := core.NewBind()
	isMac := runtime.GOOS == "darwin"
	// menu
	appMenu := menu.NewMenu()
	if isMac {
		appMenu.Append(menu.AppMenu())
		appMenu.Append(menu.EditMenu())
		appMenu.Append(menu.WindowMenu())
	}

	var appCtx context.Context

	// Create application with options
	err := wails.Run(&options.App{
		Title:                    app.AppName,
		Width:                    1280,
		MinWidth:                 960,
		Height:                   800,
		MinHeight:                600,
		Frameless:                !isMac,
		HideWindowOnClose:        true,
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "res-downloader-6b9e782a-9f5b-48c6-a51f-d890b0e5ef58",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				if appCtx != nil {
					wailsRuntime.WindowShow(appCtx)
					wailsRuntime.WindowUnminimise(appCtx)
					wailsRuntime.WindowSetAlwaysOnTop(appCtx, true)
					wailsRuntime.WindowSetAlwaysOnTop(appCtx, false)
					wailsRuntime.EventsEmit(appCtx, "second-instance-launched", secondInstanceData)
				}
			},
		},
		Menu:                     appMenu,
		EnableDefaultContextMenu: true,
		AssetServer: &assetserver.Options{
			Assets:     assets,
			Middleware: core.Middleware,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup: func(ctx context.Context) {
			appCtx = ctx
			logo := `
	 _ __    ___   ___            __| |   ___   __      __  _ __   | |   ___     __ _     __| |   ___   _ __
	| '__|  / _ \ / __|  _____   / _· |  / _ \  \ \ /\ / / | '_ \  | |  / _ \   / _· |   / _· |  / _ \ | ·__|
	| |    |  __/ \__ \ |_____| | (_| | | (_) |  \ V  V /  | | | | | | | (_) | | (_| |  | (_| | |  __/ | |
	|_|     \___| |___/          \__,_|  \___/    \_/\_/   |_| |_| |_|  \___/   \__ ,_|  \__,_|  \___| |_|`

			log.Println(logo)
			fmt.Println("version:", app.Version)
			fmt.Println("lockfile:", app.LockFile)
			app.Startup(ctx)

			go func() {
				systray.Run(func() {
					if runtime.GOOS == "windows" {
						systray.SetIcon(iconIco)
					} else {
						systray.SetIcon(icon)
					}
					systray.SetTooltip(app.AppName)

					systray.SetOnClick(func(menu systray.IMenu) {
						wailsRuntime.WindowShow(ctx)
						wailsRuntime.WindowUnminimise(ctx)
					})
					systray.SetOnDClick(func(menu systray.IMenu) {
						wailsRuntime.WindowShow(ctx)
						wailsRuntime.WindowUnminimise(ctx)
					})

					mShow := systray.AddMenuItem("显示主窗口", "显示主窗口")
					mShow.Click(func() {
						wailsRuntime.WindowShow(ctx)
						wailsRuntime.WindowUnminimise(ctx)
					})

					systray.AddSeparator()

					mQuit := systray.AddMenuItem("退出程序", "退出程序")
					mQuit.Click(func() {
						systray.Quit()
						wailsRuntime.Quit(ctx)
					})
				}, func() {
					// tray onExit
				})
			}()
		},
		OnShutdown: func(ctx context.Context) {
			systray.Quit()
			app.OnExit()
		},
		Bind: []interface{}{
			bind,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   fmt.Sprintf("%s %s", app.AppName, app.Version),
				Message: app.Description + app.Copyright,
				Icon:    icon,
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
		Windows: &windows.Options{
			WebviewIsTransparent:              false,
			WindowIsTranslucent:               false,
			DisableFramelessWindowDecorations: false,
		},
		Linux: &linux.Options{
			ProgramName:         app.AppName,
			Icon:                icon,
			WebviewGpuPolicy:    linux.WebviewGpuPolicyOnDemand,
			WindowIsTranslucent: true,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
