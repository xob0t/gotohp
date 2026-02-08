package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func init() {
	application.RegisterEvent[UploadBatchStart]("uploadStart")
	application.RegisterEvent[application.Void]("uploadStop")
	application.RegisterEvent[FileUploadResult]("FileStatus")
	application.RegisterEvent[ThreadStatus]("ThreadStatus")
	application.RegisterEvent[application.Void]("uploadCancel")
	application.RegisterEvent[int64]("uploadTotalBytes")
}

// ProgressCallback is a function type for upload progress updates
type ProgressCallback func(event string, data any)

type UploadManager struct {
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
	return m.running
}

func (m *UploadManager) Cancel() {
	if m.cancel != nil {
		close(m.cancel)
		m.cancel = nil
	}
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
	if m.running {
		return
	}

	m.running = true
	m.cancel = make(chan struct{})

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

	// Handle results and wait for completion
	go func() {
		m.wg.Wait()
		close(results)
		app.EmitEvent("uploadStop", nil)
		m.running = false
	}()

	// Process results
	go func() {
		for result := range results {
			app.EmitEvent("FileStatus", result)
			if result.IsError {
				s := fmt.Sprintf("upload error: %v", result.Error)
				app.GetLogger().Error(s)
			} else {
				s := fmt.Sprintf("upload success: %v", result.Path)
				app.GetLogger().Info(s)
			}
		}
	}()
}

// isSupportedByGooglePhotos checks if a file extension is supported by Google Photos
func isSupportedByGooglePhotos(filename string) bool {
	// Convert to lowercase for case-insensitive comparison
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return false
	}

	// Remove the dot from the extension
	ext = ext[1:]

	// Supported photo formats
	photoFormats := []string{
		"avif", "bmp", "gif", "heic", "ico",
		"jpg", "jpeg", "png", "tiff", "webp",
		"cr2", "cr3", "nef", "arw", "orf",
		"raf", "rw2", "pef", "sr2", "dng",
	}

	// Supported video formats
	videoFormats := []string{
		"3gp", "3g2", "asf", "avi", "divx",
		"m2t", "m2ts", "m4v", "mkv", "mmv",
		"mod", "mov", "mp4", "mpg", "mpeg",
		"mts", "tod", "wmv", "ts",
	}

	// Check if extension is in either supported format
	return slices.Contains(photoFormats, ext) || slices.Contains(videoFormats, ext)
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
					return nil, err
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
		return "", fmt.Errorf("error commiting file: %w", err)
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

			api, err := NewApi()
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
				app.EmitEvent("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("API error: %v", err),
				})
				continue
			}

			// Create callback from app interface
			callback := func(event string, data any) {
				app.EmitEvent(event, data)
			}
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
