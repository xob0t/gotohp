//go:build !cli

package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"strings"

	"app/core"
	"app/internal/cli"
	"app/internal/gui"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

var title = "gotohp v" + getAppVersion()

func main() {
	// Run the CLI when the arguments name one of its commands; otherwise start the GUI.
	if cli.IsCLIInvocation(os.Args[1:]) {
		os.Exit(cli.Run(os.Args[1:], cli.Info{
			ExecutableName: "gotohp",
			HasGUI:         true,
			Version:        getAppVersion(),
		}))
	}

	runGUI()
}

func runGUI() {
	normalizeFrontendDevServerURL()

	configManager := &core.ConfigManager{}
	wailsApp := application.New(application.Options{
		Name:        "com.xob0t.gotohp",
		Description: "Google Photos unofficial client",
		Services: []application.Service{
			application.NewService(configManager),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: false,
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

	uploadManager := core.NewUploadManager(gui.NewWailsReporter(wailsApp), wailsApp.Logger)

	// Create auto-backup manager; notify the frontend whenever state changes.
	autoBackupManager := core.NewAutoBackupManager(uploadManager, wailsApp.Logger, func(enabled bool, folders []string) {
		if folders == nil {
			folders = []string{}
		}
		wailsApp.Event.Emit("autoBackupStatus", gui.AutoBackupStatus{
			Enabled: enabled,
			Folders: folders,
		})
	})

	// Start the watcher now if auto-backup was already enabled at launch.
	if cfg := core.CurrentAutoBackupConfig(); cfg.Enabled && len(cfg.Folders) > 0 {
		if err := autoBackupManager.Start(); err != nil {
			wailsApp.Logger.Error("auto-backup: failed to start watcher at launch", "err", err)
		}
	}

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
		wailsApp.Event.Emit("files-dropped", core.FilesDroppedEvent{
			Files:    paths,
			DropZone: dropZone,
		})
	})

	// Listen for upload request from frontend (after drop zone is determined)
	wailsApp.Event.On("startUpload", func(e *application.CustomEvent) {
		if data, ok := e.Data.(core.StartUploadEvent); ok {
			wailsApp.Logger.Info("Starting upload", "fileCount", len(data.Files))
			uploadManager.Upload(data.Files, configManager.SessionUploadOptions())
		} else {
			wailsApp.Logger.Error("startUpload: unexpected data type", "type", fmt.Sprintf("%T", e.Data))
		}
	})

	// Listen for auto-backup toggle from frontend
	wailsApp.Event.On("setAutoBackupEnabled", func(e *application.CustomEvent) {
		enabled, _ := e.Data.(bool)
		configManager.SetAutoBackupEnabled(enabled)
		if enabled {
			if err := autoBackupManager.Restart(); err != nil {
				wailsApp.Logger.Error("auto-backup: failed to restart watcher", "err", err)
			}
		} else {
			autoBackupManager.Stop()
		}
	})

	// Listen for folder-list changes from frontend
	wailsApp.Event.On("restartAutoBackup", func(e *application.CustomEvent) {
		if err := autoBackupManager.Restart(); err != nil {
			wailsApp.Logger.Error("auto-backup: failed to restart watcher after folder change", "err", err)
		}
	})

	// Set up system tray for background running
	systemTray := wailsApp.SystemTray.New()
	if len(appIcon) > 0 {
		systemTray.SetIcon(appIcon)
	}
	systemTray.SetTooltip("gotohp - Google Photos Backup")

	trayMenu := application.NewMenu()
	trayMenu.Add("Open gotohp").OnClick(func(ctx *application.Context) {
		window.Show()
		window.Focus()
	})
	trayMenu.AddSeparator()
	var isQuitting bool
	trayMenu.Add("Quit").OnClick(func(ctx *application.Context) {
		isQuitting = true
		autoBackupManager.Stop()
		wailsApp.Quit()
	})
	systemTray.SetMenu(trayMenu)

	systemTray.OnClick(func() {
		window.Show()
		window.Focus()
	})

	// Intercept window close so it minimizes to system tray instead of exiting
	window.OnWindowEvent(events.Common.WindowClosing, func(event *application.WindowEvent) {
		if !isQuitting {
			event.Cancel()
			window.Hide()
		}
	})

	err := wailsApp.Run()
	autoBackupManager.Stop() // clean shutdown
	if err != nil {
		log.Fatal(err)
	}
}


func normalizeFrontendDevServerURL() {
	const envName = "FRONTEND_DEVSERVER_URL"

	value := os.Getenv(envName)
	value = strings.Replace(value, "http://localhost:", "http://127.0.0.1:", 1)
	value = strings.Replace(value, "https://localhost:", "https://127.0.0.1:", 1)
	if value != os.Getenv(envName) {
		_ = os.Setenv(envName, value)
	}
}
