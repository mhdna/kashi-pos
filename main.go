package main

import (
	"context"
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Paint a native splash immediately, before the WebView2 runtime boots.
	// hideSplash also runs on OnDomReady; the defer covers early-exit paths.
	showSplash()
	defer hideSplash()

	loadDotEnv()

	if err := configureWebView(); err != nil {
		println("Error configuring WebView:", err.Error())
	}

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options. In development (`wails dev`, which sets
	// the `dev` build tag) run windowed and resizable; production stays
	// fullscreen with a fixed window.
	appOptions := &options.App{
		Title:            "Kashi Cloud",
		Fullscreen:       !isDevMode,
		WindowStartState: options.Normal,
		DisableResize:    !isDevMode,
		// Keep the real window hidden until the frontend can paint, so the
		// only thing on screen during WebView2 boot is the native splash.
		StartHidden:      true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		Windows: &windows.Options{
			WebviewGpuIsDisabled: true,
		},
		OnStartup: app.startup,
		OnDomReady: func(ctx context.Context) {
			wailsruntime.WindowShow(ctx)
			hideSplash()
		},
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Debug: options.Debug{
			OpenInspectorOnStartup: envBool("KASHI_DEBUG", false),
		},
	}
	if isDevMode {
		appOptions.Width = appWindowWidth
		appOptions.Height = appWindowHeight
	}

	err := wails.Run(appOptions)
	if err != nil {
		println("Error:", err.Error())
	}
}
