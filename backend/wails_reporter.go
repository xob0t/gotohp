//go:build !cli

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

// WailsReporter forwards upload progress to the frontend as Wails events.
type WailsReporter struct {
	app *application.App
}

func NewWailsReporter(app *application.App) *WailsReporter {
	return &WailsReporter{app: app}
}

func (w *WailsReporter) UploadStart(s UploadBatchStart) { w.app.Event.Emit("uploadStart", s) }
func (w *WailsReporter) UploadStop()                    { w.app.Event.Emit("uploadStop", nil) }
func (w *WailsReporter) TotalBytes(n int64)             { w.app.Event.Emit("uploadTotalBytes", n) }
func (w *WailsReporter) TotalBytesDelta(n int64)        { w.app.Event.Emit("uploadTotalBytesDelta", n) }
func (w *WailsReporter) Warning(p PreflightWarning)     { w.app.Event.Emit("uploadWarning", p) }
func (w *WailsReporter) ThreadStatus(s ThreadStatus)    { w.app.Event.Emit("ThreadStatus", s) }
func (w *WailsReporter) FileResult(r FileUploadResult)  { w.app.Event.Emit("FileStatus", r) }
func (w *WailsReporter) AlbumProgress(s AlbumStatus)    { w.app.Event.Emit("albumProgress", s) }
func (w *WailsReporter) AlbumComplete(s AlbumStatus)    { w.app.Event.Emit("albumComplete", s) }
func (w *WailsReporter) AlbumError(e AlbumError)        { w.app.Event.Emit("albumError", e) }
