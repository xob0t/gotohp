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
// upload whenever files are created or written. It keeps track of previously
// uploaded files across restarts so that existing and new files are handled
// cleanly without repeatedly re-uploading duplicates.
type AutoBackupManager struct {
	mu            sync.Mutex
	uploadManager *UploadManager
	logger        *slog.Logger
	watcher       *fsnotify.Watcher
	history       *AutoBackupHistory

	// pending collects paths that have been seen but whose debounce timer has
	// not yet fired.
	pending  map[string]struct{}
	debounce *time.Timer
	stopCh   chan struct{}
	stopped  bool

	// onStatusChange is called whenever the watcher starts or stops, so the
	// Wails layer can emit a frontend event. May be nil.
	onStatusChange func(enabled bool, folders []string)
}

// NewAutoBackupManager creates a manager with persistent upload tracking.
func NewAutoBackupManager(uploadManager *UploadManager, logger *slog.Logger, onStatusChange func(bool, []string)) *AutoBackupManager {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &AutoBackupManager{
		uploadManager:  uploadManager,
		logger:         logger,
		history:        NewAutoBackupHistory(""),
		pending:        make(map[string]struct{}),
		onStatusChange: onStatusChange,
	}
}

// Start begins watching the currently configured backup folders and triggers
// an initial scan of all old and existing files. Idempotent.
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

	m.history.Load()

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	added := 0
	for _, folder := range folders {
		if err := m.addWatchRecursive(w, folder); err != nil {
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

	// Initial scan: check old/existing files in all watched folders
	go m.scanInitial(folders)

	m.logger.Info("auto-backup: watcher started", "folders", folders)
	if m.onStatusChange != nil {
		m.onStatusChange(true, folders)
	}
	return nil
}

// Stop shuts down the watcher and persists the upload history. Idempotent.
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

	_ = m.history.Save()

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
// folder list. Used after adding/removing folders or toggling the feature.
func (m *AutoBackupManager) Restart() error {
	m.Stop()

	cfg := currentConfig().AutoBackup
	if !cfg.Enabled || len(cfg.Folders) == 0 {
		return nil
	}
	return m.Start()
}

// addWatchRecursive walks a folder tree and registers all directories with the watcher.
func (m *AutoBackupManager) addWatchRecursive(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if err := w.Add(path); err != nil {
				m.logger.Warn("auto-backup: cannot watch subfolder", "path", path, "err", err)
			}
		}
		return nil
	})
}

// scanInitial recursively scans the given folders for existing files that haven't
// been uploaded yet and schedules them for backup.
func (m *AutoBackupManager) scanInitial(folders []string) {
	m.logger.Info("auto-backup: scanning existing files", "folders", folders)
	queued := 0

	opts := currentConfig().Preferences.UploadOptions()

	for _, folder := range folders {
		_ = filepath.WalkDir(folder, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			m.mu.Lock()
			stopped := m.stopped
			m.mu.Unlock()
			if stopped {
				return filepath.SkipAll
			}

			if d.IsDir() {
				return nil
			}

			if isTemporaryDownloadFile(path) {
				return nil
			}

			info, err := d.Info()
			if err != nil || info.Size() == 0 {
				return nil
			}

			if !opts.DisableUnsupportedFilesFilter && !isSupportedByGooglePhotos(path) {
				return nil
			}

			// If already uploaded and unmodified, skip it
			if m.history.IsUploaded(path, info.Size(), info.ModTime().Unix()) {
				return nil
			}

			m.schedule(path)
			queued++
			return nil
		})
	}

	m.logger.Info("auto-backup: initial scan completed", "newOrModifiedFiles", queued)
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

			// If a new directory was created, watch it recursively and scan it
			if event.Has(fsnotify.Create) {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					_ = m.addWatchRecursive(w, event.Name)
					go m.scanInitial([]string{event.Name})
					continue
				}
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

// schedule adds a path to the pending set and arms (or resets) the debounce timer.
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

	// If another upload run is currently active, don't drop pending files;
	// retry after a short delay once the current batch completes.
	if m.uploadManager.IsRunning() {
		if m.debounce != nil {
			m.debounce.Stop()
		}
		m.debounce = time.AfterFunc(3*time.Second, m.flush)
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
	opts.Recursive = true
	opts.OnFileUploaded = func(path string, size int64, modTime int64) {
		m.history.MarkUploaded(path, size, modTime)
	}

	var readyPaths []string
	var retryPaths []string

	for _, p := range pendingCopy {
		if isTemporaryDownloadFile(p) {
			continue
		}

		info, err := os.Stat(p)
		if err != nil {
			// File disappeared
			continue
		}

		if info.IsDir() {
			continue
		}

		if info.Size() == 0 {
			continue
		}

		if !opts.DisableUnsupportedFilesFilter && !isSupportedByGooglePhotos(p) {
			continue
		}

		// Skip if already recorded in history with identical size and modtime
		if m.history.IsUploaded(p, info.Size(), info.ModTime().Unix()) {
			continue
		}

		// Check if file is currently locked by a downloader or writer
		f, err := os.Open(p)
		if err != nil {
			retryPaths = append(retryPaths, p)
			continue
		}
		_ = f.Close()

		readyPaths = append(readyPaths, p)
	}

	// Re-queue any files that were locked
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
		_ = m.history.Save()
		return
	}

	m.logger.Info("auto-backup: triggering upload", "files", len(readyPaths))
	m.uploadManager.Upload(readyPaths, opts)

	// Persist history after starting upload
	go func() {
		time.Sleep(3 * time.Second)
		_ = m.history.Save()
	}()
}
