package backend

import "log/slog"

// AppInterface defines the interface that both GUI and CLI apps must implement
type AppInterface interface {
	// Event emitter
	EmitEvent(event string, data any)

	// Logger
	GetLogger() *slog.Logger
}
