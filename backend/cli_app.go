package backend

import (
	"io"
	"log"
	"log/slog"
	"os"
)

// CLIApp implements AppInterface for CLI usage
type CLIApp struct {
	eventCallback func(event string, data any)
	logger        *slog.Logger
}

func NewCLIApp(eventCallback func(event string, data any), logLevel slog.Level) *CLIApp {
	var logger *slog.Logger

	if logLevel <= slog.LevelInfo {
		// For info level and below, use io.Discard to hide logs
		logger = slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{
			Level: logLevel,
		}))
		// Disable HTTP client debug logs for info level
		SetHTTPClientLogger(log.New(io.Discard, "", 0))
	} else {
		// For debug level, log to stderr
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: logLevel,
		}))
		// Enable HTTP client debug logs for debug level
		SetHTTPClientLogger(nil) // nil will use default retryablehttp logger
	}

	return &CLIApp{
		eventCallback: eventCallback,
		logger:        logger,
	}
}

func NewCLIAppWithLogger(eventCallback func(event string, data any), logFile *os.File) *CLIApp {
	// Create a logger that writes to a file instead of stdout
	logger := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	return &CLIApp{
		eventCallback: eventCallback,
		logger:        logger,
	}
}

func (c *CLIApp) EmitEvent(event string, data any) {
	if c.eventCallback != nil {
		c.eventCallback(event, data)
	}
}

func (c *CLIApp) GetLogger() *slog.Logger {
	return c.logger
}
