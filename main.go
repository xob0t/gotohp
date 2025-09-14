package main

import (
	"app/backend"
	"embed"
	"log"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/info.json
var infoJson embed.FS

var title = "gotohp v" + backend.GetVersion(infoJson)

func main() {
	app := application.New(application.Options{
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

	window := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:             title,
		Frameless:         false,
		Width:             400,
		Height:            600,
		EnableDragAndDrop: true,
		DisableResize:     true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		URL: "/",
	})

	uploadManager := backend.NewUploadManager(app)

	// Listen for upload cancel event
	app.Event.On("uploadCancel", func(e *application.CustomEvent) {
		uploadManager.Cancel()
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		paths := event.Context().DroppedFiles()
		uploadManager.Upload(app, paths)
	})

	err := app.Run()
	if err != nil {
		log.Fatal(err)
	}
}
