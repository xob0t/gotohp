package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// FilesDroppedEvent is emitted when files are dropped on any drop zone
type FilesDroppedEvent struct {
	Files    []string `json:"files"`
	DropZone string   `json:"dropZone"`
}

// StartUploadEvent is received from frontend to start upload
type StartUploadEvent struct {
	Files []string `json:"files"`
}

func init() {
	application.RegisterEvent[UploadBatchStart]("uploadStart")
	application.RegisterEvent[application.Void]("uploadStop")
	application.RegisterEvent[FileUploadResult]("FileStatus")
	application.RegisterEvent[ThreadStatus]("ThreadStatus")
	application.RegisterEvent[application.Void]("uploadCancel")
	application.RegisterEvent[int64]("uploadTotalBytes")
	application.RegisterEvent[FilesDroppedEvent]("files-dropped")
	application.RegisterEvent[StartUploadEvent]("startUpload")
}

// ProgressCallback is a function type for upload progress updates
type ProgressCallback func(event string, data any)

type UploadManager struct {
	mu      sync.Mutex
	wg      sync.WaitGroup
	cancel  chan struct{}
	running bool
	app     AppInterface
}

func NewUploadManager(app AppInterface) *UploadManager {
	return &UploadManager{
		app: app,
	}
}

func (m *UploadManager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

func (m *UploadManager) Cancel() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cancel != nil {
		close(m.cancel)
		// Don't set to nil - readers still need to detect closure via select
	}
}

// isCancelled checks if cancellation has been requested
func (m *UploadManager) isCancelled() bool {
	m.mu.Lock()
	cancel := m.cancel
	m.mu.Unlock()
	if cancel == nil {
		return false
	}
	select {
	case <-cancel:
		return true
	default:
		return false
	}
}

// getCancelChan returns the cancel channel safely
func (m *UploadManager) getCancelChan() <-chan struct{} {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cancel
}

type UploadBatchStart struct {
	Total      int   `json:"Total"`
	TotalBytes int64 `json:"TotalBytes"`
}

type FileUploadResult struct {
	MediaKey string `json:"MediaKey"`
	IsError  bool   `json:"IsError"`
	Error    error  `json:"-"`
	Path     string `json:"Path"`
}

type ThreadStatus struct {
	WorkerID      int    `json:"WorkerID"`
	Status        string `json:"Status"` // "idle", "hashing", "checking", "uploading", "finalizing", "completed", "error"
	FilePath      string `json:"FilePath"`
	FileName      string `json:"FileName"`
	Message       string `json:"Message"`
	BytesUploaded int64  `json:"BytesUploaded"`
	BytesTotal    int64  `json:"BytesTotal"`
	Attempt       int    `json:"Attempt"` // Current attempt number (1-based), 0 if not applicable
}

func (m *UploadManager) Upload(app AppInterface, paths []string) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.cancel = make(chan struct{})
	m.mu.Unlock()

	targetPaths, err := filterGooglePhotosFiles(paths)
	if err != nil {
		app.EmitEvent("FileStatus", FileUploadResult{
			IsError: true,
			Error:   err,
		})
		return
	}

	if len(targetPaths) == 0 {
		return
	}

	// Emit uploadStart immediately with TotalBytes=0 for responsive UI
	app.EmitEvent("uploadStart", UploadBatchStart{
		Total:      len(targetPaths),
		TotalBytes: 0,
	})

	// Calculate total bytes asynchronously and emit update when complete
	go func() {
		var totalBytes int64
		for _, path := range targetPaths {
			if info, err := os.Stat(path); err == nil {
				totalBytes += info.Size()
			}
		}
		app.EmitEvent("uploadTotalBytes", totalBytes)
	}()

	if AppConfig.UploadThreads < 1 {
		AppConfig.UploadThreads = 1
	}

	// Don't start more threads than files to process
	numWorkers := min(AppConfig.UploadThreads, len(targetPaths))

	// Create a worker pool for concurrent uploads
	workChan := make(chan string, len(targetPaths))
	results := make(chan FileUploadResult, len(targetPaths))

	// Start workers
	for i := range numWorkers {
		m.wg.Add(1)
		go startUploadWorker(i, workChan, results, m.cancel, &m.wg, app)
	}

	// Send work to workers
	go func() {
	LOOP:
		for _, path := range targetPaths {
			select {
			case <-m.cancel:
				break LOOP
			case workChan <- path:
			}
		}
		close(workChan)
	}()

	// Handle results, wait for completion, and create album if configured
	go func() {
		// Collect successful uploads with path -> mediaKey mapping for AUTO mode
		successfulUploads := make(map[string]string) // path -> mediaKey

		// Wait for all workers to finish in a separate goroutine, then close results
		go func() {
			m.wg.Wait()
			close(results)
		}()

		// Process all results (this blocks until results channel is closed)
		for result := range results {
			app.EmitEvent("FileStatus", result)
			if result.IsError {
				s := fmt.Sprintf("upload error: %v", result.Error)
				app.GetLogger().Error(s)
			} else {
				s := fmt.Sprintf("upload success: %v", result.Path)
				app.GetLogger().Info(s)
				if result.MediaKey != "" {
					successfulUploads[result.Path] = result.MediaKey
				}
			}
		}

		// Handle album creation after all results are processed
		// Get album config atomically to avoid race conditions
		albumName, albumAutoMode := GetAlbumConfig()
		app.GetLogger().Info(fmt.Sprintf("Upload complete. Successful uploads: %d, AlbumName: '%s', AlbumAutoMode: %v",
			len(successfulUploads), albumName, albumAutoMode))

		if len(successfulUploads) > 0 {
			m.handleAlbumCreation(app, successfulUploads, albumName, albumAutoMode)
		}

		app.EmitEvent("uploadStop", nil)
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()
}

// handleAlbumCreation handles album creation based on config (manual name/key or AUTO mode)
func (m *UploadManager) handleAlbumCreation(app AppInterface, uploads map[string]string, albumName string, albumAutoMode bool) {
	// Check if cancelled before starting album creation
	if m.isCancelled() {
		app.GetLogger().Info("Upload cancelled, skipping album creation")
		return
	}

	app.GetLogger().Info(fmt.Sprintf("handleAlbumCreation called with %d uploads", len(uploads)))

	// Create API once for all album operations
	api, err := NewApi()
	if err != nil {
		app.GetLogger().Error(fmt.Sprintf("failed to create API for album creation: %v", err))
		app.EmitEvent("albumError", AlbumError{
			AlbumName: albumName,
			Error:     fmt.Sprintf("failed to initialize API: %v", err),
		})
		return
	}

	albumManager := NewAlbumManager(api, app, m.getCancelChan())

	// Check if AUTO mode is enabled
	if albumAutoMode {
		app.GetLogger().Info("AUTO mode enabled, creating albums from directories")
		m.createAlbumsFromDirectories(albumManager, app, uploads)
		return
	}

	// Manual mode: use AlbumName if set
	if albumName == "" {
		app.GetLogger().Info("No album name set and AUTO mode disabled, skipping album creation")
		return
	}

	app.GetLogger().Info(fmt.Sprintf("Creating album with name/key: '%s'", albumName))

	mediaKeys := make([]string, 0, len(uploads))
	for _, mediaKey := range uploads {
		mediaKeys = append(mediaKeys, mediaKey)
	}

	app.GetLogger().Info(fmt.Sprintf("Adding %d media keys to album '%s'", len(mediaKeys), albumName))

	albumKeys, err := albumManager.AddToAlbum(mediaKeys, albumName)
	if err != nil {
		app.GetLogger().Error(fmt.Sprintf("failed to create album '%s': %v", albumName, err))
		app.EmitEvent("albumError", AlbumError{
			AlbumName: albumName,
			Error:     err.Error(),
		})
		return
	}
	app.GetLogger().Info(fmt.Sprintf("created album '%s' with %d items, album keys: %v", albumName, len(mediaKeys), albumKeys))
}

// createAlbumsFromDirectories creates albums based on parent directory names (AUTO mode)
func (m *UploadManager) createAlbumsFromDirectories(albumManager *AlbumManager, app AppInterface, uploads map[string]string) {
	// Group media keys by parent directory
	mediaKeysByDir := make(map[string][]string)

	for filePath, mediaKey := range uploads {
		parentDir := filepath.Dir(filePath)
		mediaKeysByDir[parentDir] = append(mediaKeysByDir[parentDir], mediaKey)
	}

	// Create an album for each directory
	for dirPath, mediaKeys := range mediaKeysByDir {
		albumName := filepath.Base(dirPath)
		if albumName == "" || albumName == "." {
			albumName = "Uploads"
		}

		albumKeys, err := albumManager.AddToAlbum(mediaKeys, albumName)
		if err != nil {
			app.GetLogger().Error(fmt.Sprintf("failed to create album '%s': %v", albumName, err))
			app.EmitEvent("albumError", AlbumError{
				AlbumName: albumName,
				Error:     err.Error(),
			})
			continue
		}
		app.GetLogger().Info(fmt.Sprintf("created album '%s' with %d items, album keys: %v", albumName, len(mediaKeys), albumKeys))
	}
}

// supportedFormats is a map of file extensions supported by Google Photos (O(1) lookup)
var supportedFormats = map[string]bool{
	// Photo formats
	"avif": true, "bmp": true, "gif": true, "heic": true, "ico": true,
	"jpg": true, "jpeg": true, "png": true, "tiff": true, "webp": true,
	"cr2": true, "cr3": true, "nef": true, "arw": true, "orf": true,
	"raf": true, "rw2": true, "pef": true, "sr2": true, "dng": true,
	// Video formats
	"3gp": true, "3g2": true, "asf": true, "avi": true, "divx": true,
	"m2t": true, "m2ts": true, "m4v": true, "mkv": true, "mmv": true,
	"mod": true, "mov": true, "mp4": true, "mpg": true, "mpeg": true,
	"mts": true, "tod": true, "wmv": true, "ts": true,
}

// isSupportedByGooglePhotos checks if a file extension is supported by Google Photos
func isSupportedByGooglePhotos(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}
	// Remove the dot and check map
	return supportedFormats[ext[1:]]
}

func scanDirectoryForFiles(path string, recursive bool) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if recursive {
				subFiles, err := scanDirectoryForFiles(fullPath, recursive)
				if err != nil {
					// Log error and continue with other directories instead of failing
					// This handles permission errors, broken symlinks, etc.
					continue
				}
				files = append(files, subFiles...)
			}
		} else {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

// FilterGooglePhotosFiles returns a list of files that are supported by Google Photos (exported)
func FilterGooglePhotosFiles(paths []string) ([]string, error) {
	return filterGooglePhotosFiles(paths)
}

// filterGooglePhotosFiles returns a list of files that are supported by Google Photos
func filterGooglePhotosFiles(paths []string) ([]string, error) {
	var supportedFiles []string

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			files, err := scanDirectoryForFiles(path, AppConfig.Recursive)
			if err != nil {
				return nil, fmt.Errorf("error scanning directory %s: %v", path, err)
			}

			for _, file := range files {
				if AppConfig.DisableUnsupportedFilesFilter {
					supportedFiles = append(supportedFiles, file)
				} else {
					if isSupportedByGooglePhotos(file) {
						supportedFiles = append(supportedFiles, file)
					}
				}

			}
		} else {
			if AppConfig.DisableUnsupportedFilesFilter {
				supportedFiles = append(supportedFiles, path)
			} else {
				if isSupportedByGooglePhotos(path) {
					supportedFiles = append(supportedFiles, path)
				}
			}

		}
	}

	return supportedFiles, nil
}

// UploadFile is an exported version for CLI use with callback
func UploadFile(ctx context.Context, api *Api, filePath string, workerID int, callback ProgressCallback) (string, error) {
	return uploadFileWithCallback(ctx, api, filePath, workerID, callback)
}

func uploadFileWithCallback(ctx context.Context, api *Api, filePath string, workerID int, callback ProgressCallback) (string, error) {
	fileName := filepath.Base(filePath)
	mediakey := ""

	// Stage 1: Hashing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "hashing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Hashing...",
	})

	sha1_hash_bytes, err := CalculateSHA1(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("error calculating hash file: %w", err)
	}

	sha1_hash_b64 := base64.StdEncoding.EncodeToString([]byte(sha1_hash_bytes))

	// Stage 2: Checking if exists in library
	if !AppConfig.ForceUpload {
		callback("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "checking",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Checking if file exists in library...",
		})

		mediakey, err = api.FindRemoteMediaByHash(sha1_hash_bytes)
		if err != nil {
			// Non-fatal: log via callback and continue with upload
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "checking",
				FilePath: filePath,
				FileName: fileName,
				Message:  fmt.Sprintf("Hash check warning: %v, proceeding with upload", err),
			})
		}
		if len(mediakey) > 0 {
			callback("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "completed",
				FilePath: filePath,
				FileName: fileName,
				Message:  "Already in library",
			})
			if AppConfig.DeleteFromHost {
				if err := os.Remove(filePath); err != nil {
					return mediakey, fmt.Errorf("file exists in library but failed to delete local copy: %w", err)
				}
			}
			return mediakey, nil
		}
	}

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("error getting file info: %w", err)
	}

	// Stage 3: Uploading
	fileSize := fileInfo.Size()
	callback("ThreadStatus", ThreadStatus{
		WorkerID:      workerID,
		Status:        "uploading",
		FilePath:      filePath,
		FileName:      fileName,
		Message:       "Uploading...",
		BytesUploaded: 0,
		BytesTotal:    fileSize,
	})

	token, err := api.GetUploadToken(sha1_hash_b64, fileSize)
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)
	}

	// Create progress callback for upload
	progressCallback := func(bytesUploaded, bytesTotal int64, attempt int) {
		message := "Uploading..."
		if attempt > 1 {
			message = fmt.Sprintf("Retrying... (attempt %d)", attempt)
		}
		callback("ThreadStatus", ThreadStatus{
			WorkerID:      workerID,
			Status:        "uploading",
			FilePath:      filePath,
			FileName:      fileName,
			Message:       message,
			BytesUploaded: bytesUploaded,
			BytesTotal:    bytesTotal,
			Attempt:       attempt,
		})
	}

	CommitToken, err := api.UploadFileWithProgress(ctx, filePath, token, progressCallback)
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)

	}

	// Stage 4: Finalizing
	callback("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "finalizing",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Committing upload...",
	})

	mediaKey, err := api.CommitUpload(CommitToken, fileInfo.Name(), sha1_hash_bytes, fileInfo.ModTime().Unix())
	if err != nil {
		return "", fmt.Errorf("error committing file: %w", err)
	}

	if len(mediaKey) == 0 {
		return "", fmt.Errorf("media key not received")
	}

	if AppConfig.DeleteFromHost {
		if err := os.Remove(filePath); err != nil {
			return mediaKey, fmt.Errorf("uploaded successfully but failed to delete file: %w", err)
		}
	}

	return mediaKey, nil

}

func startUploadWorker(workerID int, workChan <-chan string, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup, app AppInterface) {
	defer wg.Done()

	// Emit idle status initially
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Waiting for files...",
	})

	// Create API client once per worker for connection reuse
	api, err := NewApi()
	if err != nil {
		app.EmitEvent("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "error",
			Message:  fmt.Sprintf("Failed to initialize API: %v", err),
		})
		return
	}

	// Create callback from app interface (reuse for all files)
	callback := func(event string, data any) {
		app.EmitEvent(event, data)
	}

	for path := range workChan {
		select {
		case <-cancel:
			app.EmitEvent("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Cancelled",
			})
			return // Stop if cancellation is requested
		default:
			ctx, cancelUpload := context.WithCancel(context.Background())
			go func() {
				select {
				case <-cancel:
					cancelUpload()
				case <-ctx.Done():
					// Upload completed or context cancelled
				}
			}()

			mediaKey, err := uploadFileWithCallback(ctx, api, path, workerID, callback)
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Error: %v", err),
				})
			} else {
				results <- FileUploadResult{IsError: false, Path: path, MediaKey: mediaKey}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  "Completed",
				})
			}
			cancelUpload()

			// Mark as idle after completing file
			app.EmitEvent("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Waiting for next file...",
			})
		}
	}

	// Final idle status when no more work
	app.EmitEvent("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Finished",
	})
}
