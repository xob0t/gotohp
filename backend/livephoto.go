package backend

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

type UploadWorkKind string

const (
	UploadWorkSingle    UploadWorkKind = "single"
	UploadWorkLivePhoto UploadWorkKind = "live-photo"
)

type SingleMedia struct {
	Path string
}

type LivePhotoPair struct {
	ContentIdentifier string
	PhotoPath         string
	VideoPath         string
}

type UploadWorkItem struct {
	Kind      UploadWorkKind
	Single    *SingleMedia
	LivePhoto *LivePhotoPair
}

type PreflightWarning struct {
	Paths   []string `json:"Paths"`
	Code    string   `json:"Code"`
	Message string   `json:"Message"`
}

type LivePhotoClassificationOptions struct {
	Enabled             bool
	SkipIncomplete      bool
	IgnoreAppleMetadata bool
}

type LivePhotoMetadataReader interface {
	PhotoContentIdentifier(path string) (string, error)
	VideoLivePhotoMetadata(path string) (LivePhotoMetadata, error)
}

type fileLivePhotoMetadataReader struct{}

func (fileLivePhotoMetadataReader) PhotoContentIdentifier(path string) (string, error) {
	return ReadPhotoContentIdentifier(path)
}

func (fileLivePhotoMetadataReader) VideoLivePhotoMetadata(path string) (LivePhotoMetadata, error) {
	return ReadVideoLivePhotoMetadata(path)
}

type indexedLivePhotoMedia struct {
	path  string
	index int
}

type livePhotoCandidates struct {
	photos []indexedLivePhotoMedia
	videos []indexedLivePhotoMedia
}

// ClassifyUploadWork normally pairs files by their embedded Apple content
// identifier. The explicit CLI override uses case-insensitive filename stems;
// it remains one-to-one and still requires valid Live Photo video timing data.
func ClassifyUploadWork(paths []string, options LivePhotoClassificationOptions, reader LivePhotoMetadataReader) ([]UploadWorkItem, []PreflightWarning) {
	if !options.Enabled {
		return singleUploadWork(paths), nil
	}
	if reader == nil {
		reader = fileLivePhotoMetadataReader{}
	}

	candidatesByIdentifier := make(map[string]*livePhotoCandidates)
	warnings := make([]PreflightWarning, 0)
	for index, path := range paths {
		switch livePhotoCandidateType(path) {
		case "photo":
			if options.IgnoreAppleMetadata {
				candidates := candidatesForIdentifier(candidatesByIdentifier, livePhotoFilenameMatchKey(path))
				candidates.photos = append(candidates.photos, indexedLivePhotoMedia{path: path, index: index})
				continue
			}
			identifier, err := reader.PhotoContentIdentifier(path)
			if errors.Is(err, ErrContentIdentifierMissing) {
				continue
			}
			if err != nil {
				warnings = append(warnings, metadataReadWarning(path, err))
				continue
			}
			candidates := candidatesForIdentifier(candidatesByIdentifier, identifier)
			candidates.photos = append(candidates.photos, indexedLivePhotoMedia{path: path, index: index})
		case "video":
			metadata, err := reader.VideoLivePhotoMetadata(path)
			if errors.Is(err, ErrContentIdentifierMissing) && !options.IgnoreAppleMetadata {
				continue
			}
			if err != nil && !errors.Is(err, ErrContentIdentifierMissing) {
				warnings = append(warnings, metadataReadWarning(path, err))
				continue
			}
			if !metadata.HasStillImageTime {
				warnings = append(warnings, PreflightWarning{
					Paths:   []string{path},
					Code:    "missing-still-image-time",
					Message: "video has a Live Photo content identifier but no still-image-time marker",
				})
				continue
			}
			identifier := metadata.ContentIdentifier
			if options.IgnoreAppleMetadata {
				identifier = livePhotoFilenameMatchKey(path)
			}
			candidates := candidatesForIdentifier(candidatesByIdentifier, identifier)
			candidates.videos = append(candidates.videos, indexedLivePhotoMedia{path: path, index: index})
		}
	}

	pairsByIndex := make(map[int]LivePhotoPair)
	consumed := make(map[int]bool)
	identifiers := sortedLivePhotoIdentifiers(candidatesByIdentifier)
	for _, identifier := range identifiers {
		candidates := candidatesByIdentifier[identifier]
		if len(candidates.photos) == 1 && len(candidates.videos) == 1 {
			photo := candidates.photos[0]
			video := candidates.videos[0]
			pairIndex := min(photo.index, video.index)
			contentIdentifier := identifier
			if options.IgnoreAppleMetadata {
				contentIdentifier = ""
			}
			pairsByIndex[pairIndex] = LivePhotoPair{
				ContentIdentifier: contentIdentifier,
				PhotoPath:         photo.path,
				VideoPath:         video.path,
			}
			consumed[photo.index] = true
			consumed[video.index] = true
			continue
		}
		if len(candidates.photos) > 1 || len(candidates.videos) > 1 {
			ambiguousPaths := candidatePaths(candidates)
			code := "ambiguous-identifier"
			message := "multiple photo or video candidates share one Live Photo identifier"
			if options.IgnoreAppleMetadata {
				code = "ambiguous-filename-stem"
				message = "multiple photo or video candidates share one filename stem"
			}
			warnings = append(warnings, PreflightWarning{
				Paths:   ambiguousPaths,
				Code:    code,
				Message: message,
			})
			continue
		}
		if options.SkipIncomplete {
			orphanPaths := candidatePaths(candidates)
			for _, photo := range candidates.photos {
				consumed[photo.index] = true
			}
			for _, video := range candidates.videos {
				consumed[video.index] = true
			}
			warnings = append(warnings, PreflightWarning{
				Paths:   orphanPaths,
				Code:    "incomplete-live-photo-skipped",
				Message: "Skipped an incomplete Live Photo because its matching file is not in this queue. You can disable this check in Settings.",
			})
		}
	}

	work := make([]UploadWorkItem, 0, len(paths))
	for index, path := range paths {
		if pair, ok := pairsByIndex[index]; ok {
			pair := pair
			work = append(work, UploadWorkItem{Kind: UploadWorkLivePhoto, LivePhoto: &pair})
			continue
		}
		if consumed[index] {
			continue
		}
		work = append(work, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}})
	}
	return work, warnings
}

func sortedLivePhotoIdentifiers(candidatesByIdentifier map[string]*livePhotoCandidates) []string {
	identifiers := make([]string, 0, len(candidatesByIdentifier))
	for identifier := range candidatesByIdentifier {
		identifiers = append(identifiers, identifier)
	}
	sort.Slice(identifiers, func(left, right int) bool {
		return candidateFirstIndex(candidatesByIdentifier[identifiers[left]]) < candidateFirstIndex(candidatesByIdentifier[identifiers[right]])
	})
	return identifiers
}

func candidateFirstIndex(candidates *livePhotoCandidates) int {
	first := int(^uint(0) >> 1)
	for _, photo := range candidates.photos {
		first = min(first, photo.index)
	}
	for _, video := range candidates.videos {
		first = min(first, video.index)
	}
	return first
}

func singleUploadWork(paths []string) []UploadWorkItem {
	work := make([]UploadWorkItem, 0, len(paths))
	for _, path := range paths {
		work = append(work, UploadWorkItem{Kind: UploadWorkSingle, Single: &SingleMedia{Path: path}})
	}
	return work
}

func livePhotoCandidateType(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".heic", ".heif", ".jpg", ".jpeg":
		return "photo"
	case ".mov":
		return "video"
	default:
		return ""
	}
}

func livePhotoFilenameMatchKey(path string) string {
	cleanPath := filepath.Clean(path)
	return strings.ToLower(strings.TrimSuffix(cleanPath, filepath.Ext(cleanPath)))
}

func candidatesForIdentifier(candidatesByIdentifier map[string]*livePhotoCandidates, identifier string) *livePhotoCandidates {
	candidates := candidatesByIdentifier[identifier]
	if candidates == nil {
		candidates = &livePhotoCandidates{}
		candidatesByIdentifier[identifier] = candidates
	}
	return candidates
}

func metadataReadWarning(path string, err error) PreflightWarning {
	return PreflightWarning{
		Paths:   []string{path},
		Code:    "metadata-read-failed",
		Message: fmt.Sprintf("could not read Live Photo metadata: %v", err),
	}
}

func candidatePaths(candidates *livePhotoCandidates) []string {
	paths := make([]string, 0, len(candidates.photos)+len(candidates.videos))
	for _, photo := range candidates.photos {
		paths = append(paths, photo.path)
	}
	for _, video := range candidates.videos {
		paths = append(paths, video.path)
	}
	return paths
}
