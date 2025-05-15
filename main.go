package main

import (
	"embed"
	"fmt"
	"log"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

var App *application.App
var uploadCancel chan struct{}
var uploadWG sync.WaitGroup

func main() {
	App = application.New(application.Options{
		Name:        "gotohp",
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(&ConfigService{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := App.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
		Title:             "gotohp",
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

	// Listen for upload cancel event
	App.OnEvent("uploadCancel", func(e *application.CustomEvent) {
		if uploadCancel != nil {
			close(uploadCancel)
			uploadCancel = nil
		}
	})

	window.OnWindowEvent(events.Common.WindowFilesDropped, func(event *application.WindowEvent) {
		if UploadRunning {
			return
		}

		paths := event.Context().DroppedFiles()
		targetPaths, err := GetFiles(paths)
		if err != nil {
			App.EmitEvent("FileStatus", UploadStatus{
				IsError: true,
				Error:   err,
			})
			return
		}

		App.EmitEvent("uploadStart", UploadStarted{
			Total: len(targetPaths),
		})

		UploadRunning = true
		uploadCancel = make(chan struct{})
		uploadWG = sync.WaitGroup{}

		if GlobalSettingsConfig.UploadThreads < 1 {
			GlobalSettingsConfig.UploadThreads = 1
		}
		// Create a worker pool for concurrent uploads
		workChan := make(chan string, len(targetPaths))
		results := make(chan UploadStatus, len(targetPaths))

		// Start workers
		for i := 0; i < GlobalSettingsConfig.UploadThreads; i++ {
			uploadWG.Add(1)
			go uploadWorker(workChan, results, uploadCancel, &uploadWG)
		}

		// Send work to workers
		go func() {
			for _, path := range targetPaths {
				select {
				case <-uploadCancel:
					break
				case workChan <- path:
				}
			}
			close(workChan)
		}()

		// Handle results and wait for completion
		go func() {
			uploadWG.Wait()
			close(results)
			App.EmitEvent("uploadStop")
			UploadRunning = false
		}()

		// Process results
		go func() {
			for result := range results {
				App.EmitEvent("FileStatus", result)
				if result.IsError {
					s := fmt.Sprintf("upload error: %v", result.Error)
					App.Logger.Error(s)
				} else {
					s := fmt.Sprintf("upload success: %v", result.Path)
					App.Logger.Info(s)
				}
			}
		}()

	})

	err := App.Run()
	if err != nil {
		log.Fatal(err)
	}
}

func uploadWorker(workChan <-chan string, results chan<- UploadStatus, cancel <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range workChan {
		select {
		case <-cancel:
			return
		default:
			err := Upload(path)
			if err != nil {
				results <- UploadStatus{
					IsError: true,
					Error:   err,
					Path:    path,
				}
			} else {
				results <- UploadStatus{
					IsError: false,
					Path:    path,
				}
			}
		}
	}
}
