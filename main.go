package main

import (
	"context"
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
var title = "gotohp v" + GetVersion()

func main() {
	App = application.New(application.Options{
		Name:        title,
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(&ConfigManager{}),
		},
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
	})

	window := App.NewWebviewWindowWithOptions(application.WebviewWindowOptions{
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
		targetPaths, err := FilterSupportedFiles(paths)
		if err != nil {
			App.EmitEvent("FileStatus", FileUploadResult{
				IsError: true,
				Error:   err,
			})
			return
		}

		App.EmitEvent("uploadStart", UploadBatchStart{
			Total: len(targetPaths),
		})

		UploadRunning = true
		uploadCancel = make(chan struct{})
		uploadWG = sync.WaitGroup{}

		if AppConfig.UploadThreads < 1 {
			AppConfig.UploadThreads = 1
		}
		// Create a worker pool for concurrent uploads
		workChan := make(chan string, len(targetPaths))
		results := make(chan FileUploadResult, len(targetPaths))

		// Start workers
		for range AppConfig.UploadThreads {
			uploadWG.Add(1)
			go startUploadWorker(workChan, results, uploadCancel, &uploadWG)
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

func startUploadWorker(workChan <-chan string, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for path := range workChan {
		select {
		case <-cancel:
			return // Stop if cancellation is requested
		default:
			ctx, cancelUpload := context.WithCancel(context.Background())
			go func() {
				<-cancel // If global cancel happens, cancel this upload
				cancelUpload()
			}()

			err := Upload(ctx, path)
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
			} else {
				results <- FileUploadResult{IsError: false, Path: path}
			}
			cancelUpload()
		}
	}
}
