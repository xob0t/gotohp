package cli

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"app/backend"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"
)

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
	success    bool
	skipped    bool
	fileName   string
	paths      []string
	mediaKey   string
	skipCode   string
	skipReason string
	err        error
}

type preflightWarningMsg struct {
	paths   []string
	code    string
	message string
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
	skipped      int
	currentFiles map[int]string // workerID -> current file
	workers      map[int]string // workerID -> status message
	results      []uploadResult // Track all upload results
	warnings     []uploadWarning
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
	Path       string   `json:"path"`
	Paths      []string `json:"paths,omitempty"`
	Success    bool     `json:"success"`
	Skipped    bool     `json:"skipped,omitempty"`
	MediaKey   string   `json:"mediaKey,omitempty"`
	SkipCode   string   `json:"skipCode,omitempty"`
	SkipReason string   `json:"skipReason,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type uploadWarning struct {
	Paths   []string `json:"paths,omitempty"`
	Code    string   `json:"code"`
	Message string   `json:"message"`
}

type albumSummary struct {
	Name       string   `json:"name,omitempty"`
	ItemsAdded int      `json:"itemsAdded,omitempty"`
	AlbumKeys  []string `json:"albumKeys,omitempty"`
	Error      string   `json:"error,omitempty"`
}

type uploadSummary struct {
	Total     int             `json:"total"`
	Succeeded int             `json:"succeeded"`
	Failed    int             `json:"failed"`
	Skipped   int             `json:"skipped"`
	Results   []uploadResult  `json:"results"`
	Warnings  []uploadWarning `json:"warnings,omitempty"`
	Album     *albumSummary   `json:"album,omitempty"`
}

func initialModel() uploadModel {
	return uploadModel{
		progress:     progress.New(progress.WithDefaultGradient()),
		currentFiles: make(map[int]string),
		workers:      make(map[int]string),
		results:      []uploadResult{},
		warnings:     []uploadWarning{},
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
			Path:       msg.fileName,
			Paths:      msg.paths,
			Success:    msg.success,
			Skipped:    msg.skipped,
			MediaKey:   msg.mediaKey,
			SkipCode:   msg.skipCode,
			SkipReason: msg.skipReason,
		}
		if msg.skipped {
			m.skipped++
		} else if msg.success {
			m.completed++
		} else {
			m.failed++
			if msg.err != nil {
				result.Error = msg.err.Error()
			}
		}
		m.results = append(m.results, result)
		return m, nil

	case preflightWarningMsg:
		m.warnings = append(m.warnings, uploadWarning{
			Paths:   msg.paths,
			Code:    msg.code,
			Message: msg.message,
		})
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
		percent := float64(m.completed+m.failed+m.skipped) / float64(m.totalFiles)
		b.WriteString(m.progress.ViewAs(percent))
		fmt.Fprintf(&b, "\n%d/%d items", m.completed+m.failed+m.skipped, m.totalFiles)
		fmt.Fprintf(&b, " (✓ %d success, ↷ %d skipped, ✗ %d failed)\n\n", m.completed, m.skipped, m.failed)
	}

	// Worker status
	for i := 0; i < len(m.workers); i++ {
		if status, ok := m.workers[i]; ok {
			b.WriteString(status)
			b.WriteString("\n")
		}
	}

	if len(m.warnings) > 0 {
		warningStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
		b.WriteString("\n")
		b.WriteString(warningStyle.Render("Upload warnings:"))
		b.WriteString("\n")
		for _, warning := range m.warnings {
			fmt.Fprintf(&b, "- [%s] %s", warning.Code, warning.Message)
			if len(warning.Paths) > 0 {
				fmt.Fprintf(&b, " (%s)", strings.Join(warning.Paths, ", "))
			}
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
			fmt.Fprintf(&b, " (%d items)\n", m.albumItemsAdded)
		} else if m.albumTotalItems > 0 {
			b.WriteString(albumStyle.Render("Adding to album: "))
			b.WriteString(m.albumName)
			fmt.Fprintf(&b, " (%d/%d items)\n", m.albumItemsAdded, m.albumTotalItems)
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

func shouldUseTUI(settings uploadRunSettings) bool {
	return !settings.noTUI &&
		term.IsTerminal(os.Stdin.Fd()) &&
		term.IsTerminal(os.Stdout.Fd())
}

// teaReporter renders backend progress by forwarding it to the bubbletea program.
type teaReporter struct {
	p *tea.Program
}

func (r teaReporter) UploadStart(start backend.UploadBatchStart) {
	r.p.Send(uploadStartMsg{total: start.Total})
}

func (r teaReporter) UploadStop() {
	r.p.Send(uploadCompleteMsg{})
}

func (r teaReporter) TotalBytes(int64)      {}
func (r teaReporter) TotalBytesDelta(int64) {}

func (r teaReporter) Warning(warning backend.PreflightWarning) {
	r.p.Send(preflightWarningMsg{
		paths:   warning.Paths,
		code:    warning.Code,
		message: warning.Message,
	})
}

func (r teaReporter) ThreadStatus(status backend.ThreadStatus) {
	r.p.Send(fileProgressMsg{
		workerID: status.WorkerID,
		status:   status.Status,
		fileName: status.FileName,
		message:  status.Message,
	})
}

func (r teaReporter) FileResult(result backend.FileUploadResult) {
	r.p.Send(fileCompleteMsg{
		success:    !result.IsError && !result.Skipped,
		skipped:    result.Skipped,
		fileName:   result.Path,
		paths:      result.Paths,
		mediaKey:   result.MediaKey,
		skipCode:   result.SkipCode,
		skipReason: result.SkipReason,
		err:        result.Error,
	})
}

func (r teaReporter) AlbumProgress(status backend.AlbumStatus) {
	r.p.Send(albumProgressMsg{
		albumName:  status.AlbumName,
		itemsAdded: status.ItemsAdded,
		totalItems: status.TotalItems,
	})
}

func (r teaReporter) AlbumComplete(status backend.AlbumStatus) {
	r.p.Send(albumCompleteMsg{
		albumName:  status.AlbumName,
		itemsAdded: status.ItemsAdded,
		albumKeys:  status.AlbumKeys,
	})
}

func (r teaReporter) AlbumError(albumErr backend.AlbumError) {
	r.p.Send(albumErrorMsg{
		albumName: albumErr.AlbumName,
		error:     albumErr.Error,
	})
}

func newLogger(level slog.Level) *slog.Logger {
	if level > slog.LevelDebug {
		// Anything above debug would only clutter the TUI; the JSON summary
		// carries the outcome.
		return slog.New(slog.DiscardHandler)
	}
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
}

// runUpload runs the upload with already-resolved options and prints a JSON
// summary when it completes.
func runUpload(paths []string, opts backend.UploadOptions, settings uploadRunSettings) error {
	model := initialModel()
	programOptions := []tea.ProgramOption{}
	if !shouldUseTUI(settings) {
		programOptions = append(
			programOptions,
			tea.WithInput(nil),
			tea.WithoutRenderer(),
		)
	}
	p := tea.NewProgram(model, programOptions...)

	uploadManager := backend.NewUploadManager(teaReporter{p: p}, newLogger(parseLogLevel(settings.logLevel)))
	go uploadManager.Upload(paths, opts)

	// Run until the upload manager reports UploadStop.
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running upload program: %w", err)
	}

	if m, ok := finalModel.(uploadModel); ok {
		summary := buildUploadSummary(m)
		jsonOutput, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return fmt.Errorf("error generating JSON: %w", err)
		}
		fmt.Println(string(jsonOutput))
	}

	return nil
}

func buildUploadSummary(model uploadModel) uploadSummary {
	warnings := make([]uploadWarning, 0, len(model.warnings))
	for _, warning := range model.warnings {
		if warning.Code == "incomplete-live-photo-skipped" || warning.Code == "ambiguous-filename-stem" {
			continue
		}
		warnings = append(warnings, warning)
	}
	summary := uploadSummary{
		Total:     model.totalFiles,
		Succeeded: model.completed,
		Failed:    model.failed,
		Skipped:   model.skipped,
		Results:   model.results,
		Warnings:  warnings,
	}
	if model.albumName != "" {
		summary.Album = &albumSummary{
			Name:       model.albumName,
			ItemsAdded: model.albumItemsAdded,
			AlbumKeys:  model.albumKeys,
			Error:      model.albumError,
		}
	}
	return summary
}
