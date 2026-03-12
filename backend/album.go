package backend

import (
	"fmt"

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
	api *Api
	app AppInterface
}

// NewAlbumManager creates a new AlbumManager
func NewAlbumManager(api *Api, app AppInterface) *AlbumManager {
	return &AlbumManager{
		api: api,
		app: app,
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

	// Process in API-sized batches (500 items per call)
	for i := 0; i < len(mediaKeys); i += AlbumBatchSize {
		end := min(i+AlbumBatchSize, len(mediaKeys))
		batch := mediaKeys[i:end]

		err := m.api.AddMediaToAlbum(albumKey, batch)
		if err != nil {
			return []string{albumKey}, fmt.Errorf("failed to add media to album: %w", err)
		}

		itemsAdded += len(batch)

		// Emit progress event
		m.app.EmitEvent("albumProgress", AlbumStatus{
			AlbumName:  "Existing Album",
			ItemsAdded: itemsAdded,
			TotalItems: totalItems,
			AlbumKeys:  []string{albumKey},
			IsComplete: false,
		})
	}

	// Emit completion event
	m.app.EmitEvent("albumComplete", AlbumStatus{
		AlbumName:  "Existing Album",
		ItemsAdded: itemsAdded,
		TotalItems: totalItems,
		AlbumKeys:  []string{albumKey},
		IsComplete: true,
	})

	return []string{albumKey}, nil
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
			batchEnd := min(j+AlbumBatchSize, len(albumBatch))
			batch := albumBatch[j:batchEnd]

			var err error
			if currentAlbumKey == "" {
				// First batch: create the album
				currentAlbumKey, err = m.api.CreateAlbum(currentAlbumName, batch)
				if err != nil {
					return albumKeys, fmt.Errorf("failed to create album '%s': %w", currentAlbumName, err)
				}
				albumKeys = append(albumKeys, currentAlbumKey)
			} else {
				// Subsequent batches: add to existing album
				err = m.api.AddMediaToAlbum(currentAlbumKey, batch)
				if err != nil {
					return albumKeys, fmt.Errorf("failed to add media to album '%s': %w", currentAlbumName, err)
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
