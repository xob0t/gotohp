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
	albumName                     string
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

// Album messages
type albumProgressMsg struct {
	albumName  string
	itemsAdded int
	totalItems int
}

type albumCompleteMsg struct {
	albumName  string
	itemsAdded int
	albumKeys  []string
}

type albumErrorMsg struct {
	albumName string
	error     string
}

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
	// Album state
	albumName       string
	albumItemsAdded int
	albumTotalItems int
	albumComplete   bool
	albumError      string
	albumKeys       []string
}

type uploadResult struct {
	Path     string `json:"path"`
	Success  bool   `json:"success"`
	MediaKey string `json:"mediaKey,omitempty"`
	Error    string `json:"error,omitempty"`
}

type albumSummary struct {
	Name       string   `json:"name,omitempty"`
	ItemsAdded int      `json:"itemsAdded,omitempty"`
	AlbumKeys  []string `json:"albumKeys,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type uploadSummary struct {
	Total     int            `json:"total"`
	Succeeded int            `json:"succeeded"`
	Failed    int            `json:"failed"`
	Results   []uploadResult `json:"results"`
	Album     *albumSummary  `json:"album,omitempty"`
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

	case albumProgressMsg:
		m.albumName = msg.albumName
		m.albumItemsAdded = msg.itemsAdded
		m.albumTotalItems = msg.totalItems
		m.albumComplete = false
		return m, nil

	case albumCompleteMsg:
		m.albumName = msg.albumName
		m.albumItemsAdded = msg.itemsAdded
		m.albumComplete = true
		m.albumKeys = msg.albumKeys
		return m, nil

	case albumErrorMsg:
		m.albumName = msg.albumName
		m.albumError = msg.error
		return m, nil

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

	// Album progress
	if m.albumName != "" {
		b.WriteString("\n")
		albumStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
		if m.albumError != "" {
			errorStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("196"))
			b.WriteString(errorStyle.Render("✗ Album error: "))
			b.WriteString(m.albumError)
			b.WriteString("\n")
		} else if m.albumComplete {
			b.WriteString(albumStyle.Render("✓ Added to album: "))
			b.WriteString(m.albumName)
			b.WriteString(fmt.Sprintf(" (%d items)\n", m.albumItemsAdded))
		} else if m.albumTotalItems > 0 {
			b.WriteString(albumStyle.Render("Adding to album: "))
			b.WriteString(m.albumName)
			b.WriteString(fmt.Sprintf(" (%d/%d items)\n", m.albumItemsAdded, m.albumTotalItems))
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

	// Handle album option - check for AUTO mode
	if strings.ToUpper(config.albumName) == "AUTO" {
		backend.AppConfig.AlbumAutoMode = true
		backend.AppConfig.AlbumName = ""
	} else {
		backend.AppConfig.AlbumAutoMode = false
		backend.AppConfig.AlbumName = config.albumName
	}

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
		case "albumProgress":
			if status, ok := data.(backend.AlbumStatus); ok {
				p.Send(albumProgressMsg{
					albumName:  status.AlbumName,
					itemsAdded: status.ItemsAdded,
					totalItems: status.TotalItems,
				})
			}
		case "albumComplete":
			if status, ok := data.(backend.AlbumStatus); ok {
				p.Send(albumCompleteMsg{
					albumName:  status.AlbumName,
					itemsAdded: status.ItemsAdded,
					albumKeys:  status.AlbumKeys,
				})
			}
		case "albumError":
			if albumErr, ok := data.(backend.AlbumError); ok {
				p.Send(albumErrorMsg{
					albumName: albumErr.AlbumName,
					error:     albumErr.Error,
				})
			}
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

		// Add album info if present
		if m.albumName != "" {
			summary.Album = &albumSummary{
				Name:       m.albumName,
				ItemsAdded: m.albumItemsAdded,
				AlbumKeys:  m.albumKeys,
				Error:      m.albumError,
			}
		}

		jsonOutput, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("error generating JSON: %w", err)
		}

		fmt.Println(string(jsonOutput))
	}

	return nil
}
