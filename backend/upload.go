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

type UploadManager struct {
	wg      sync.WaitGroup
	cancel  chan struct{}
	running bool
	app     *application.App
}

func NewUploadManager(app *application.App) *UploadManager {
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
	Total int
}

type FileUploadResult struct {
	MediaKey string
	IsError  bool
	Error    error
	Path     string
}

type ThreadStatus struct {
	WorkerID int
	Status   string // "idle", "hashing", "checking", "uploading", "finalizing", "completed", "error"
	FilePath string
	FileName string
	Message  string
}

func (m *UploadManager) Upload(app *application.App, paths []string) {
	if m.running {
		return
	}

	m.running = true
	m.cancel = make(chan struct{})

	targetPaths, err := filterGooglePhotosFiles(paths)
	if err != nil {
		app.Event.Emit("FileStatus", FileUploadResult{
			IsError: true,
			Error:   err,
		})
		return
	}

	if len(targetPaths) == 0 {
		return
	}

	app.Event.Emit("uploadStart", UploadBatchStart{
		Total: len(targetPaths),
	})

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
		app.Event.Emit("uploadStop")
		m.running = false
	}()

	// Process results
	go func() {
		for result := range results {
			app.Event.Emit("FileStatus", result)
			if result.IsError {
				s := fmt.Sprintf("upload error: %v", result.Error)
				app.Logger.Error(s)
			} else {
				s := fmt.Sprintf("upload success: %v", result.Path)
				app.Logger.Info(s)
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

func uploadFile(ctx context.Context, api *Api, filePath string, workerID int, app *application.App) (string, error) {
	fileName := filepath.Base(filePath)
	mediakey := ""

	// Stage 1: Hashing
	app.Event.Emit("ThreadStatus", ThreadStatus{
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
		app.Event.Emit("ThreadStatus", ThreadStatus{
			WorkerID: workerID,
			Status:   "checking",
			FilePath: filePath,
			FileName: fileName,
			Message:  "Checking if file exists in library...",
		})

		mediakey, err = api.FindRemoteMediaByHash(sha1_hash_bytes)
		if err != nil {
			fmt.Println("Error checking for remote matches:", err)
		}
		if len(mediakey) > 0 {
			app.Event.Emit("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "completed",
				FilePath: filePath,
				FileName: fileName,
				Message:  "Already in library",
			})
			if AppConfig.DeleteFromHost {
				err = os.Remove(filePath)
				if err != nil {
					fmt.Println("Error deleting file:", err)
				}
			}
			return mediakey, nil
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	fileInfo, err := file.Stat()
	file.Close()

	if err != nil {
		return "", fmt.Errorf("error getting file info: %w", err)
	}

	// Stage 3: Uploading
	app.Event.Emit("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "uploading",
		FilePath: filePath,
		FileName: fileName,
		Message:  "Uploading...",
	})

	token, err := api.GetUploadToken(sha1_hash_b64, fileInfo.Size())
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)
	}

	CommitToken, err := api.UploadFile(ctx, filePath, token)
	if err != nil {
		return "", fmt.Errorf("error uploading file: %w", err)

	}

	// Stage 4: Finalizing
	app.Event.Emit("ThreadStatus", ThreadStatus{
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
		os.Remove(filePath)
	}

	return mediaKey, nil

}

func startUploadWorker(workerID int, workChan <-chan string, results chan<- FileUploadResult, cancel <-chan struct{}, wg *sync.WaitGroup, app *application.App) {
	defer wg.Done()

	// Emit idle status initially
	app.Event.Emit("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Waiting for files...",
	})

	for path := range workChan {
		select {
		case <-cancel:
			app.Event.Emit("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Cancelled",
			})
			return // Stop if cancellation is requested
		default:
			ctx, cancelUpload := context.WithCancel(context.Background())
			go func() {
				<-cancel // If global cancel happens, cancel this upload
				cancelUpload()
			}()

			api, err := NewApi()
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
				app.Event.Emit("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("API error: %v", err),
				})
				continue
			}
			mediaKey, err := uploadFile(ctx, api, path, workerID, app)
			if err != nil {
				results <- FileUploadResult{IsError: true, Error: err, Path: path}
				app.Event.Emit("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "error",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  fmt.Sprintf("Error: %v", err),
				})
			} else {
				results <- FileUploadResult{IsError: false, Path: path, MediaKey: mediaKey}
				app.Event.Emit("ThreadStatus", ThreadStatus{
					WorkerID: workerID,
					Status:   "completed",
					FilePath: path,
					FileName: filepath.Base(path),
					Message:  "Completed",
				})
			}
			cancelUpload()

			// Mark as idle after completing file
			app.Event.Emit("ThreadStatus", ThreadStatus{
				WorkerID: workerID,
				Status:   "idle",
				Message:  "Waiting for next file...",
			})
		}
	}

	// Final idle status when no more work
	app.Event.Emit("ThreadStatus", ThreadStatus{
		WorkerID: workerID,
		Status:   "idle",
		Message:  "Finished",
	})
}
