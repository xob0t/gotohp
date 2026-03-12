package backend

import (
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// AlbumError represents an album creation error
type AlbumError struct {
	AlbumName string `json:"AlbumName"`
	Error     string `json:"Error"`
}

func init() {
	application.RegisterEvent[AlbumStatus]("albumProgress")
	application.RegisterEvent[AlbumStatus]("albumComplete")
	application.RegisterEvent[AlbumError]("albumError")
}

const (
	// AlbumBatchSize is the maximum number of items per API call
	AlbumBatchSize = 500
	// AlbumLimit is the maximum number of items per album
	AlbumLimit = 20000
)

// AlbumStatus represents the status of album creation
type AlbumStatus struct {
	AlbumName  string   `json:"AlbumName"`
	ItemsAdded int      `json:"ItemsAdded"`
	TotalItems int      `json:"TotalItems"`
	AlbumKeys  []string `json:"AlbumKeys"`
	IsComplete bool     `json:"IsComplete"`
}

// AlbumManager handles album creation with batching
type AlbumManager struct {
	api    *Api
	app    AppInterface
	cancel <-chan struct{}
}

// NewAlbumManager creates a new AlbumManager
func NewAlbumManager(api *Api, app AppInterface, cancel <-chan struct{}) *AlbumManager {
	return &AlbumManager{
		api:    api,
		app:    app,
		cancel: cancel,
	}
}

// isCancelled checks if cancellation has been requested
func (m *AlbumManager) isCancelled() bool {
	if m.cancel == nil {
		return false
	}
	select {
	case <-m.cancel:
		return true
	default:
		return false
	}
}

// IsAlbumKey checks if the input looks like an album media key (starts with "AF1Qip")
func IsAlbumKey(input string) bool {
	return len(input) > 6 && input[:6] == "AF1Qip"
}

// AddToAlbum adds media items to an album with proper batching.
// - If albumNameOrKey is an album key (starts with AF1Qip), adds to existing album
// - Otherwise creates a new album with that name
// - If items exceed AlbumLimit (20,000), creates multiple numbered albums
// Returns a list of album media keys for all created/used albums.
func (m *AlbumManager) AddToAlbum(mediaKeys []string, albumNameOrKey string) ([]string, error) {
	if len(mediaKeys) == 0 {
		return nil, fmt.Errorf("no media keys provided")
	}

	albumNameOrKey = strings.TrimSpace(albumNameOrKey)
	if albumNameOrKey == "" {
		return nil, fmt.Errorf("album name or key cannot be empty")
	}

	// Check if we're adding to an existing album
	if IsAlbumKey(albumNameOrKey) {
		return m.addToExistingAlbum(mediaKeys, albumNameOrKey)
	}

	return m.createNewAlbum(mediaKeys, albumNameOrKey)
}

// addToExistingAlbum adds media to an existing album using the album media key
func (m *AlbumManager) addToExistingAlbum(mediaKeys []string, albumKey string) ([]string, error) {
	totalItems := len(mediaKeys)
	itemsAdded := 0
	displayName := fmt.Sprintf("Album (%s...)", albumKey[:10])

	// Process in API-sized batches (500 items per call)
	for i := 0; i < len(mediaKeys); i += AlbumBatchSize {
		// Check for cancellation
		if m.isCancelled() {
			return []string{albumKey}, fmt.Errorf("album creation cancelled (added %d/%d items)", itemsAdded, totalItems)
		}

		end := min(i+AlbumBatchSize, len(mediaKeys))
		batch := mediaKeys[i:end]

		err := m.addMediaWithRetry(albumKey, batch)
		if err != nil {
			return []string{albumKey}, fmt.Errorf("failed to add media to album (added %d/%d items): %w", itemsAdded, totalItems, err)
		}

		itemsAdded += len(batch)

		// Emit progress event
		m.app.EmitEvent("albumProgress", AlbumStatus{
			AlbumName:  displayName,
			ItemsAdded: itemsAdded,
			TotalItems: totalItems,
			AlbumKeys:  []string{albumKey},
			IsComplete: false,
		})
	}

	// Emit completion event
	m.app.EmitEvent("albumComplete", AlbumStatus{
		AlbumName:  displayName,
		ItemsAdded: itemsAdded,
		TotalItems: totalItems,
		AlbumKeys:  []string{albumKey},
		IsComplete: true,
	})

	return []string{albumKey}, nil
}

// addMediaWithRetry adds media to an album with retry logic
func (m *AlbumManager) addMediaWithRetry(albumKey string, mediaKeys []string) error {
	retryConfig := DefaultRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check for cancellation before retry delay
			if m.isCancelled() {
				return fmt.Errorf("cancelled during retry")
			}
			delay := CalculateBackoff(attempt-1, retryConfig)
			time.Sleep(delay)
		}

		err := m.api.AddMediaToAlbum(albumKey, mediaKeys)
		if err == nil {
			return nil
		}
		lastErr = err

		// Don't retry on 4xx errors (except 429)
		if !isRetryableAlbumError(err) {
			return err
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", retryConfig.MaxRetries+1, lastErr)
}

// isRetryableAlbumError checks if an error should be retried
func isRetryableAlbumError(err error) bool {
	errStr := err.Error()
	// Retry on 5xx errors or 429 (rate limit)
	if strings.Contains(errStr, "status 5") || strings.Contains(errStr, "status 429") {
		return true
	}
	// Retry on network errors
	if strings.Contains(errStr, "connection") || strings.Contains(errStr, "timeout") {
		return true
	}
	return false
}

// createNewAlbum creates a new album with the given name and adds media to it
func (m *AlbumManager) createNewAlbum(mediaKeys []string, albumName string) ([]string, error) {
	var albumKeys []string
	totalItems := len(mediaKeys)
	itemsAdded := 0
	albumCounter := 1

	// Log if we need multiple albums
	if len(mediaKeys) > AlbumLimit {
		m.app.GetLogger().Warn(fmt.Sprintf("%d items exceed the album limit of %d. They will be split into multiple albums.", len(mediaKeys), AlbumLimit))
	}

	// Process in album-sized chunks (up to 20,000 items per album)
	for i := 0; i < len(mediaKeys); i += AlbumLimit {
		// Check for cancellation
		if m.isCancelled() {
			return albumKeys, fmt.Errorf("album creation cancelled (added %d/%d items)", itemsAdded, totalItems)
		}

		end := min(i+AlbumLimit, len(mediaKeys))
		albumBatch := mediaKeys[i:end]

		// Determine album name (add suffix if multiple albums needed)
		currentAlbumName := albumName
		if len(mediaKeys) > AlbumLimit {
			currentAlbumName = fmt.Sprintf("%s (%d)", albumName, albumCounter)
		}

		var currentAlbumKey string

		// Process this album's items in API-sized batches (500 items per call)
		for j := 0; j < len(albumBatch); j += AlbumBatchSize {
			// Check for cancellation
			if m.isCancelled() {
				return albumKeys, fmt.Errorf("album creation cancelled (added %d/%d items)", itemsAdded, totalItems)
			}

			batchEnd := min(j+AlbumBatchSize, len(albumBatch))
			batch := albumBatch[j:batchEnd]

			var err error
			if currentAlbumKey == "" {
				// First batch: create the album with retry
				currentAlbumKey, err = m.createAlbumWithRetry(currentAlbumName, batch)
				if err != nil {
					return albumKeys, fmt.Errorf("failed to create album '%s' (added %d/%d items): %w", currentAlbumName, itemsAdded, totalItems, err)
				}
				albumKeys = append(albumKeys, currentAlbumKey)
			} else {
				// Subsequent batches: add to existing album with retry
				err = m.addMediaWithRetry(currentAlbumKey, batch)
				if err != nil {
					return albumKeys, fmt.Errorf("failed to add media to album '%s' (added %d/%d items): %w", currentAlbumName, itemsAdded, totalItems, err)
				}
			}

			itemsAdded += len(batch)

			// Emit progress event
			m.app.EmitEvent("albumProgress", AlbumStatus{
				AlbumName:  currentAlbumName,
				ItemsAdded: itemsAdded,
				TotalItems: totalItems,
				AlbumKeys:  albumKeys,
				IsComplete: false,
			})
		}

		albumCounter++
	}

	// Emit completion event
	m.app.EmitEvent("albumComplete", AlbumStatus{
		AlbumName:  albumName,
		ItemsAdded: itemsAdded,
		TotalItems: totalItems,
		AlbumKeys:  albumKeys,
		IsComplete: true,
	})

	return albumKeys, nil
}

// createAlbumWithRetry creates an album with retry logic
func (m *AlbumManager) createAlbumWithRetry(albumName string, mediaKeys []string) (string, error) {
	retryConfig := DefaultRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if attempt > 0 {
			// Check for cancellation before retry delay
			if m.isCancelled() {
				return "", fmt.Errorf("cancelled during retry")
			}
			delay := CalculateBackoff(attempt-1, retryConfig)
			time.Sleep(delay)
		}

		albumKey, err := m.api.CreateAlbum(albumName, mediaKeys)
		if err == nil {
			return albumKey, nil
		}
		lastErr = err

		// Don't retry on 4xx errors (except 429)
		if !isRetryableAlbumError(err) {
			return "", err
		}
	}

	return "", fmt.Errorf("failed after %d attempts: %w", retryConfig.MaxRetries+1, lastErr)
}
