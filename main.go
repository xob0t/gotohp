package main

import (
	"embed"
	"log"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Wails uses Go's `embed` package to embed the frontend files into the binary.
// Any files in the frontend/dist folder will be embedded into the binary and
// made available to the frontend.
// See https://pkg.go.dev/embed for more information.

//go:embed all:frontend/dist
var assets embed.FS

var App *application.App

// main function serves as the application's entry point. It initializes the application, creates a window,
// and starts a goroutine that emits a time-based event every second. It subsequently runs the application and
// logs any error that might occur.
func main() {

	// Create a new Wails application by providing the necessary options.
	// Variables 'Name' and 'Description' are for application metadata.
	// 'Assets' configures the asset server with the 'FS' variable pointing to the frontend files.
	// 'Bind' is a list of Go struct instances. The frontend has access to the methods of these instances.
	// 'Mac' options tailor the application when running an macOS.
	App = application.New(application.Options{
		Name:        "gotohs",
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(&ConfigService{}),
			application.NewService(&UploadService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	// Create a new window with the necessary options.
	// 'Title' is the title of the window.
	// 'Mac' options tailor the window when running on macOS.
	// 'BackgroundColour' is the background colour of the window.
	// 'URL' is the URL that will be loaded into the webview.
	window := App.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:             "gotohs",
		Frameless:         false,
		Width:             1000,
		Height:            900,
		EnableDragAndDrop: true,
		Mac: application.MacWindow{
			InvisibleTitleBarHeight: 0,
			Backdrop:                application.MacBackdropTranslucent,
			TitleBar:                application.MacTitleBarHiddenInset,
		},
		BackgroundType: application.BackgroundTypeTranslucent,
		URL:            "/",
	})

	// Create a goroutine that emits an event containing the current time every second.
	// The frontend can listen to this event and update the UI accordingly.
	go func() {
		for {
			now := time.Now().Format(time.RFC1123)
			App.EmitEvent("time", now)
			time.Sleep(time.Second)
		}
	}()

	App.OnEvent("mergeSettingsChanged", func(e *application.CustomEvent) {
		App.Logger.Info("mergeSettingsChanged")
		App.Logger.Info("[Go] WailsEvent received", "name", e.Name, "data", e.Data, "sender", e.Sender, "cancelled", e.Cancelled)
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		if ActionRunning {
			return
		}
		ActionRunning = true
		paths := event.Context().DroppedFiles()
		targetPaths, err := Filter(paths)
		if err != nil {
			App.EmitEvent("uploadStatus", UploadStatus{
				IsError: true,
				Error:   err,
			})
		}
		for _, path := range targetPaths {
			err := Upload(path)
			if err != nil {
				App.EmitEvent("uploadStatus", UploadStatus{
					IsError: true,
					Error:   err,
				})
			} else {
				App.EmitEvent("uploadStatus", UploadStatus{
					IsError: false,
				})
			}
		}
		ActionRunning = false
	})

	// Run the application. This blocks until the application has been exited.
	err := App.Run()

	// If an error occurred while running the application, log it and exit.
	if err != nil {
		log.Fatal(err)
	}
}
