package backend

import (
	"log/slog"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// WailsApp wraps a Wails application to implement AppInterface
type WailsApp struct {
	app *application.App
}

func NewWailsApp(app *application.App) *WailsApp {
	return &WailsApp{app: app}
}

func (w *WailsApp) EmitEvent(event string, data any) {
	w.app.Event.Emit(event, data)
}

func (w *WailsApp) GetLogger() *slog.Logger {
	return w.app.Logger
}
