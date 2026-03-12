//go:build !cli

package main

import (
	"app/backend"
	"embed"
	"fmt"
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
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:               title,
		Frameless:           false,
		Width:               400,
		Height:              600,
		EnableFileDrop:      true,
		DisableResize:       true,
		MaximiseButtonState: application.ButtonDisabled,
		BackgroundType:      application.BackgroundTypeTranslucent,
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

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		paths := event.Context().DroppedFiles()
		dropTarget := event.Context().DropTargetDetails()

		var dropZone string
		if dropTarget != nil {
			dropZone = dropTarget.Attributes["data-drop-zone"]
			wailsApp.Logger.Info("Drop target detected",
				"dropZone", dropZone,
				"elementID", dropTarget.ElementID)
		}

		// Emit event to frontend with drop details
		wailsApp.Event.Emit("files-dropped", backend.FilesDroppedEvent{
			Files:    paths,
			DropZone: dropZone,
		})
	})

	// Listen for upload request from frontend (after drop zone is determined)
	wailsApp.Event.On("startUpload", func(e *application.CustomEvent) {
		if data, ok := e.Data.(backend.StartUploadEvent); ok {
			wailsApp.Logger.Info("Starting upload", "fileCount", len(data.Files))
			uploadManager.Upload(app, data.Files)
		} else {
			wailsApp.Logger.Error("startUpload: unexpected data type", "type", fmt.Sprintf("%T", e.Data))
		}
	})

	err := wailsApp.Run()
	if err != nil {
		log.Fatal(err)
	}
}
