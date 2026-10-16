package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// BackupFileRecord stores file metadata at the time of a successful upload.
type BackupFileRecord struct {
	Size    int64 `json:"size"`
	ModTime int64 `json:"modTime"`
}

// AutoBackupHistory maintains a persistent record of uploaded files to prevent
// duplicate uploads across application restarts.
type AutoBackupHistory struct {
	mu       sync.RWMutex
	filePath string
	records  map[string]BackupFileRecord
	dirty    bool
}

func normalizeHistoryKey(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	clean := filepath.Clean(abs)
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}

func defaultHistoryFilePath() string {
	cfgDir := filepath.Dir(ConfigPath)
	if cfgDir == "" || cfgDir == "." {
		userConfigDir := filepath.Join(getUserConfigDir(), "gotohp")
		return filepath.Join(userConfigDir, "autobackup_history.json")
	}
	return filepath.Join(cfgDir, "autobackup_history.json")
}

// NewAutoBackupHistory loads or initializes the auto-backup history.
func NewAutoBackupHistory(filePath string) *AutoBackupHistory {
	if filePath == "" {
		filePath = defaultHistoryFilePath()
	}
	h := &AutoBackupHistory{
		filePath: filePath,
		records:  make(map[string]BackupFileRecord),
	}
	h.Load()
	return h
}

// Load reads the history file from disk.
func (h *AutoBackupHistory) Load() {
	h.mu.Lock()
	defer h.mu.Unlock()

	data, err := os.ReadFile(h.filePath)
	if err != nil {
		return
	}
	var loaded map[string]BackupFileRecord
	if err := json.Unmarshal(data, &loaded); err == nil && loaded != nil {
		h.records = loaded
	}
}

// Save writes the records to disk if changes were made.
func (h *AutoBackupHistory) Save() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.dirty {
		return nil
	}

	data, err := json.MarshalIndent(h.records, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	tmpFile := h.filePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		return err
	}

	_ = os.Remove(h.filePath) // Windows overwrite safety
	if err := os.Rename(tmpFile, h.filePath); err != nil {
		return os.WriteFile(h.filePath, data, 0o644)
	}

	h.dirty = false
	return nil
}

// IsUploaded reports whether a file with identical size and modification time
// has already been recorded as uploaded.
func (h *AutoBackupHistory) IsUploaded(path string, size int64, modTime int64) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	key := normalizeHistoryKey(path)
	rec, ok := h.records[key]
	if !ok {
		return false
	}
	return rec.Size == size && rec.ModTime == modTime
}

// MarkUploaded records that the file was successfully uploaded or confirmed remote.
func (h *AutoBackupHistory) MarkUploaded(path string, size int64, modTime int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	key := normalizeHistoryKey(path)
	h.records[key] = BackupFileRecord{
		Size:    size,
		ModTime: modTime,
	}
	h.dirty = true
}

// Count returns the number of recorded uploaded files.
func (h *AutoBackupHistory) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.records)
}
