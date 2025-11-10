//go:build !cli

package main

import (
	"app/backend"
	"embed"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

var title = "gotohp v" + getAppVersion()

func main() {
	// Check if running in CLI mode based on recognized commands
	// If unrecognized arguments are passed, default to GUI mode
	if len(os.Args) > 1 && isCLICommand(os.Args[1]) {
		runCLI()
		return
	}

	// Run GUI mode (default when no arguments or unrecognized arguments)
	runGUI()
}

func runGUI() {
	wailsApp := application.New(application.Options{
		Name:        "com.xob0t.gotohp",
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(&backend.ConfigManager{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:             title,
		Frameless:         false,
		Width:             400,
		Height:            600,
		MaxWidth:          400,
		MaxHeight:         600,
		EnableDragAndDrop: true,
		DisableResize:     true,
		BackgroundType:    application.BackgroundTypeTranslucent,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	// Wrap Wails app in AppInterface
	app := backend.NewWailsApp(wailsApp)
	uploadManager := backend.NewUploadManager(app)

	// Listen for upload cancel event
	wailsApp.Event.On("uploadCancel", func(e *application.CustomEvent) {
		uploadManager.Cancel()
	})

	window.OnWindowEvent(events.Common.WindowDropZoneFilesDropped, func(event *application.WindowEvent) {
		paths := event.Context().DroppedFiles()
		uploadManager.Upload(app, paths)
	})

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
