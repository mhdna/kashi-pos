package main

import (
	"context"
	"embed"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// appVersion is set at build time via -ldflags "-X main.appVersion=...".
// Left as "dev" for local `wails dev`/unflagged `go build` runs, where no
// release version applies.
var appVersion = "dev"

func main() {
	// The update helper (updater.go) is this same program run from a
	// temporary copy; it swaps the executable and exits, no window.
	if len(os.Args) == 3 && os.Args[1] == "--apply-update" {
		os.Exit(runUpdateHelper(os.Args[2], defaultUpdateProcesses()))
	}

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
		Title:            "GB Cloud",
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
			WebviewUserDataPath:  webviewUserDataPath(),
		},
		// A second launch (a double-click while the splash is still up, or the
		// Desktop and Start Menu shortcuts both used) would otherwise run a
		// second copy against the same database and cloud sync. It exits
		// instead and brings this window forward.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "gb-cloud-7f3c1d52-9a0e-4b6a-8c14-2e5d90b7a1f6",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) {
				// Nil until startup has run; the window isn't up to show yet.
				if ctx := app.ctx; ctx != nil {
					wailsruntime.WindowUnminimise(ctx)
					wailsruntime.WindowShow(ctx)
				}
			},
		},
		OnStartup: app.startup,
		OnDomReady: func(ctx context.Context) {
			wailsruntime.WindowShow(ctx)
			hideSplash()
			// Off the startup path: nothing on screen depends on these.
			go autoHideTaskbar()
			go registerAutostart()
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
	restoreTaskbar()
	if err != nil {
		println("Error:", err.Error())
	}
}
