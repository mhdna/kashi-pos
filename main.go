package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	loadDotEnv()

	if err := configureWebView(); err != nil {
		println("Error configuring WebView:", err.Error())
	}

	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title: "kashi-pos",
		// Width:      appWindowWidth,
		// Height:     appWindowHeight,
		// MinWidth:   appMinWindowWidth,
		// MinHeight:  appMinWindowHeight,
		Fullscreen:       false,
		WindowStartState: options.Maximised,
		// DisableResize:    true,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 255},
		Windows: &windows.Options{
			WebviewGpuIsDisabled: true,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		// Debug: options.Debug{
		// 	OpenInspectorOnStartup: true,
		// },
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
