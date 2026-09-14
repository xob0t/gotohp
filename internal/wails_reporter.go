//go:build !cli

package internal

import (
	"app/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type WailsReporter struct{ app *application.App }

func NewWailsReporter(app *application.App) *WailsReporter   { return &WailsReporter{app: app} }
func (w *WailsReporter) UploadStart(s core.UploadBatchStart) { w.app.Event.Emit("uploadStart", s) }
func (w *WailsReporter) UploadStop()                         { w.app.Event.Emit("uploadStop", nil) }
func (w *WailsReporter) TotalBytes(n int64)                  { w.app.Event.Emit("uploadTotalBytes", n) }
func (w *WailsReporter) TotalBytesDelta(n int64)             { w.app.Event.Emit("uploadTotalBytesDelta", n) }
func (w *WailsReporter) Warning(p core.PreflightWarning)     { w.app.Event.Emit("uploadWarning", p) }
func (w *WailsReporter) ThreadStatus(s core.ThreadStatus)    { w.app.Event.Emit("ThreadStatus", s) }
func (w *WailsReporter) FileResult(r core.FileUploadResult)  { w.app.Event.Emit("FileStatus", r) }
func (w *WailsReporter) AlbumProgress(s core.AlbumStatus)    { w.app.Event.Emit("albumProgress", s) }
func (w *WailsReporter) AlbumComplete(s core.AlbumStatus)    { w.app.Event.Emit("albumComplete", s) }
func (w *WailsReporter) AlbumError(e core.AlbumError)        { w.app.Event.Emit("albumError", e) }
