package main

import (
	"app/backend"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// CLI flags and config
type cliConfig struct {
	recursive                     bool
	threads                       int
	forceUpload                   bool
	deleteFromHost                bool
	disableUnsupportedFilesFilter bool
	logLevel                      string
	configPath                    string
}

// Messages for bubbletea
type uploadStartMsg struct {
	total int
}

type fileProgressMsg struct {
	workerID int
	status   string
	fileName string
	message  string
}

type fileCompleteMsg struct {
	success  bool
	fileName string
	mediaKey string
	err      error
}

type uploadCompleteMsg struct{}

// Bubbletea model
type uploadModel struct {
	progress     progress.Model
	totalFiles   int
	completed    int
	failed       int
	currentFiles map[int]string // workerID -> current file
	workers      map[int]string // workerID -> status message
	results      []uploadResult // Track all upload results
	width        int
	quitting     bool
}

type uploadResult struct {
	Path     string `json:"path"`
	Success  bool   `json:"success"`
	MediaKey string `json:"mediaKey,omitempty"`
	Error    string `json:"error,omitempty"`
}

type uploadSummary struct {
	Total     int            `json:"total"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Results   []uploadResult `json:"results"`
}

func initialModel() uploadModel {
	return uploadModel{
		progress:     progress.New(progress.WithDefaultGradient()),
		currentFiles: make(map[int]string),
		workers:      make(map[int]string),
		results:      []uploadResult{},
		width:        80,
	}
}

func (m uploadModel) Init() tea.Cmd {
	return nil
}

func (m uploadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.progress.Width = msg.Width - 4
		return m, nil

	case uploadStartMsg:
		m.totalFiles = msg.total
		return m, nil

	case fileProgressMsg:
		m.workers[msg.workerID] = fmt.Sprintf("[%d] %s: %s", msg.workerID, msg.status, msg.fileName)
		if msg.fileName != "" {
			m.currentFiles[msg.workerID] = msg.fileName
		}
		return m, nil

	case fileCompleteMsg:
		result := uploadResult{
			Path:     msg.fileName,
			Success:  msg.success,
			MediaKey: msg.mediaKey,
		}
		if msg.success {
			m.completed++
		} else {
			m.failed++
			if msg.err != nil {
				result.Error = msg.err.Error()
			}
		}
		m.results = append(m.results, result)
		return m, nil

	case uploadCompleteMsg:
		m.quitting = true
		return m, tea.Quit

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m uploadModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	b.WriteString(titleStyle.Render("Uploading to Google Photos"))
	b.WriteString("\n\n")

	// Progress bar
	if m.totalFiles > 0 {
		percent := float64(m.completed+m.failed) / float64(m.totalFiles)
		b.WriteString(m.progress.ViewAs(percent))
		b.WriteString(fmt.Sprintf("\n%d/%d files", m.completed+m.failed, m.totalFiles))
		b.WriteString(fmt.Sprintf(" (✓ %d success, ✗ %d failed)\n\n", m.completed, m.failed))
	}

	// Worker status
	for i := 0; i < len(m.workers); i++ {
		if status, ok := m.workers[i]; ok {
			b.WriteString(status)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\nPress Ctrl+C to cancel\n")

	return b.String()
}

// parseLogLevel converts a string log level to slog.Level
func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		// Default to info for CLI
		return slog.LevelInfo
	}
}

// CLI upload implementation
func runCLIUpload(filePaths []string, config cliConfig) error {
	// Set custom config path if provided
	if config.configPath != "" {
		backend.ConfigPath = config.configPath
	}

	// Load backend config
	err := backend.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with CLI flags
	backend.AppConfig.Recursive = config.recursive
	backend.AppConfig.UploadThreads = config.threads
	backend.AppConfig.ForceUpload = config.forceUpload
	backend.AppConfig.DeleteFromHost = config.deleteFromHost
	backend.AppConfig.DisableUnsupportedFilesFilter = config.disableUnsupportedFilesFilter

	// Parse log level
	logLevel := parseLogLevel(config.logLevel)

	// Start bubbletea program
	model := initialModel()
	p := tea.NewProgram(model)

	// Create CLI app with event callback to bubbletea
	eventCallback := func(event string, data any) {
		switch event {
		case "uploadStart":
			if start, ok := data.(backend.UploadBatchStart); ok {
				p.Send(uploadStartMsg{total: start.Total})
			}
		case "ThreadStatus":
			if status, ok := data.(backend.ThreadStatus); ok {
				fileName := status.FileName
				// No truncation - show full filename
				p.Send(fileProgressMsg{
					workerID: status.WorkerID,
					status:   status.Status,
					fileName: fileName,
					message:  status.Message,
				})
			}
		case "FileStatus":
			if result, ok := data.(backend.FileUploadResult); ok {
				p.Send(fileCompleteMsg{
					success:  !result.IsError,
					fileName: result.Path,
					mediaKey: result.MediaKey,
					err:      result.Error,
				})
			}
		case "uploadStop":
			p.Send(uploadCompleteMsg{})
		}
	}

	cliApp := backend.NewCLIApp(eventCallback, logLevel)
	uploadManager := backend.NewUploadManager(cliApp)

	// Run upload in background
	go func() {
		uploadManager.Upload(cliApp, filePaths)
	}()

	// Run the TUI
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	// Print JSON summary after TUI completes
	if m, ok := finalModel.(uploadModel); ok {
		summary := uploadSummary{
			Total:     m.totalFiles,
			Succeeded: m.completed,
			Failed:    m.failed,
			Results:   m.results,
		}

		jsonOutput, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("error generating JSON: %w", err)
		}

		fmt.Println(string(jsonOutput))
	}

	return nil
}
