package main

import (
	"embed"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is shown in the window title so it's clear which build a till
// is running. Bump it with every build that goes to a till.
const appVersion = "2026.10.01-fix-prices"

func main() {
	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Kashi Product Importer " + appVersion,
		Width:  1100,
		Height: 800,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
