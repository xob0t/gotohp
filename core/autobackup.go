//go:build !cli

package core

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const autoBackupDebounce = 5 * time.Second

// AutoBackupManager watches the configured backup folders and triggers an
// upload whenever files are created or written.  It shares the global config
// (AutoBackupEnabled / AutoBackupFolders) that is persisted via ConfigManager.
type AutoBackupManager struct {
	mu            sync.Mutex
	uploadManager *UploadManager
	logger        *slog.Logger
	watcher       *fsnotify.Watcher

	// pending collects paths that have been seen but whose debounce timer has
	// not yet fired.
	pending   map[string]struct{}
	debounce  *time.Timer
	stopCh    chan struct{}
	stopped   bool

	// onStatusChange is called whenever the watcher starts or stops, so the
	// Wails layer can emit a frontend event.  May be nil.
	onStatusChange func(enabled bool, folders []string)
}

// NewAutoBackupManager creates a manager but does not start any watcher yet.
// Call Start() / Stop() explicitly, or toggle via SetEnabled().
func NewAutoBackupManager(uploadManager *UploadManager, logger *slog.Logger, onStatusChange func(bool, []string)) *AutoBackupManager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &AutoBackupManager{
		uploadManager:  uploadManager,
		logger:         logger,
		pending:        make(map[string]struct{}),
		onStatusChange: onStatusChange,
	}
}

// Start begins watching the currently configured backup folders.  It is
// idempotent: calling it when the watcher is already running does nothing.
func (m *AutoBackupManager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.watcher != nil {
		return nil // already running
	}

	folders := currentConfig().AutoBackup.Folders
	if len(folders) == 0 {
		m.logger.Info("auto-backup: no folders configured, watcher not started")
		return nil
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	added := 0
	for _, folder := range folders {
		if err := w.Add(folder); err != nil {
			m.logger.Warn("auto-backup: cannot watch folder", "path", folder, "err", err)
			continue
		}
		added++
	}

	if added == 0 {
		_ = w.Close()
		m.logger.Warn("auto-backup: no folders could be watched")
		return nil
	}

	m.watcher = w
	m.stopCh = make(chan struct{})
	m.stopped = false

	go m.loop()

	m.logger.Info("auto-backup: watcher started", "folders", folders)
	if m.onStatusChange != nil {
		m.onStatusChange(true, folders)
	}
	return nil
}

// Stop shuts down the watcher.  Idempotent.
func (m *AutoBackupManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.watcher == nil {
		return
	}

	m.stopped = true
	close(m.stopCh)
	_ = m.watcher.Close()
	m.watcher = nil

	if m.debounce != nil {
		m.debounce.Stop()
		m.debounce = nil
	}
	m.pending = make(map[string]struct{})

	m.logger.Info("auto-backup: watcher stopped")
	if m.onStatusChange != nil {
		m.onStatusChange(false, currentConfig().AutoBackup.Folders)
	}
}

// IsRunning reports whether the watcher is currently active.
func (m *AutoBackupManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.watcher != nil
}

// Restart stops any running watcher and starts a fresh one with the current
// folder list.  Used after adding/removing folders or toggling the feature.
func (m *AutoBackupManager) Restart() error {
	m.Stop()

	cfg := currentConfig().AutoBackup
	if !cfg.Enabled || len(cfg.Folders) == 0 {
		return nil
	}
	return m.Start()
}

// loop is the main event-dispatch goroutine.
func (m *AutoBackupManager) loop() {
	m.mu.Lock()
	w := m.watcher
	stopCh := m.stopCh
	m.mu.Unlock()

	if w == nil {
		return
	}

	for {
		select {
		case <-stopCh:
			return

		case event, ok := <-w.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
				m.schedule(event.Name)
			}

		case err, ok := <-w.Errors:
			if !ok {
				return
			}
			m.logger.Warn("auto-backup: watcher error", "err", err)
		}
	}
}

// schedule adds a path to the pending set and arms (or resets) the debounce
// timer so we batch many rapid filesystem events into a single upload.
func (m *AutoBackupManager) schedule(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	m.pending[abs] = struct{}{}

	if m.debounce != nil {
		m.debounce.Reset(autoBackupDebounce)
		return
	}
	m.debounce = time.AfterFunc(autoBackupDebounce, m.flush)
}

// isTemporaryDownloadFile detects transient download/temp files from browsers or apps.
func isTemporaryDownloadFile(filename string) bool {
	base := filepath.Base(filename)
	if strings.HasPrefix(base, ".") || strings.HasPrefix(base, "~$") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".crdownload", ".part", ".download", ".tmp", ".temp", ".aria2", ".partial":
		return true
	}
	return false
}

// flush drains the pending set and starts an upload run.
func (m *AutoBackupManager) flush() {
	m.mu.Lock()

	if m.stopped {
		m.mu.Unlock()
		return
	}

	pendingCopy := make([]string, 0, len(m.pending))
	for p := range m.pending {
		pendingCopy = append(pendingCopy, p)
	}
	m.pending = make(map[string]struct{})
	m.debounce = nil

	m.mu.Unlock()

	if len(pendingCopy) == 0 {
		return
	}

	opts := currentConfig().Preferences.UploadOptions()
	opts.Recursive = true // always scan recursively for backup folders

	var readyPaths []string
	var retryPaths []string

	for _, p := range pendingCopy {
		if isTemporaryDownloadFile(p) {
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			// File disappeared (e.g. temporary file deleted or renamed)
			continue
		}

		if info.IsDir() {
			readyPaths = append(readyPaths, p)
			continue
		}

		// Skip 0-byte placeholders
		if info.Size() == 0 {
			continue
		}

		// Skip non-media files unless unsupported filter is disabled
		if !opts.DisableUnsupportedFilesFilter && !isSupportedByGooglePhotos(p) {
			continue
		}

		// Check if file is currently locked by a downloader or writer
		f, err := os.Open(p)
		if err != nil {
			// File is busy; re-queue to try again after next debounce cycle
			retryPaths = append(retryPaths, p)
			continue
		}
		_ = f.Close()

		readyPaths = append(readyPaths, p)
	}

	// Re-queue any files that were locked/still writing
	if len(retryPaths) > 0 {
		m.mu.Lock()
		for _, rp := range retryPaths {
			m.pending[rp] = struct{}{}
		}
		if m.debounce == nil && !m.stopped {
			m.debounce = time.AfterFunc(autoBackupDebounce, m.flush)
		}
		m.mu.Unlock()
	}

	if len(readyPaths) == 0 {
		return
	}

	m.logger.Info("auto-backup: triggering upload", "files", len(readyPaths))
	m.uploadManager.Upload(readyPaths, opts)
}

