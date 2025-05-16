package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type UploadBatchStart struct {
	Total int
}

type FileUploadResult struct {
	IsError bool
	Error   error
	Path    string
}

func FilterSupportedFiles(targetPaths []string) ([]string, error) {
	supportedFiles, err := filterGooglePhotosFiles(targetPaths)
	if err != nil {
		return []string{}, err
	}

	return supportedFiles, nil
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
				} else if isSupportedByGooglePhotos(file) {
					supportedFiles = append(supportedFiles, file)
				}
			}
		} else {
			if !AppConfig.DisableUnsupportedFilesFilter {
				supportedFiles = append(supportedFiles, path)
			} else if isSupportedByGooglePhotos(path) {
				supportedFiles = append(supportedFiles, path)
			}

		}
	}

	return supportedFiles, nil
}

func Upload(ctx context.Context, filePath string) error {
	api, _ := NewApi()

	mediakey := ""

	sha1_hash_bytes, err := CalculateSHA1(ctx, filePath)
	if err != nil {
		return fmt.Errorf("error calculating hash file: %w", err)
	}

	sha1_hash_b64 := base64.StdEncoding.EncodeToString([]byte(sha1_hash_bytes))

	if !AppConfig.ForceUpload {
		mediakey, err = api.FindRemoteMediaByHash(sha1_hash_bytes)
		if err != nil {
			fmt.Println("Error checking for remote matches:", err)
		}
		if len(mediakey) > 0 {
			if AppConfig.DeleteFromHost {
				err = os.Remove(filePath)
				if err != nil {
					fmt.Println("Error deleting file:", err)
				}
			}
			return nil
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	fileInfo, err := file.Stat()
	file.Close()

	if err != nil {
		return fmt.Errorf("error getting file info: %w", err)
	}

	token, err := api.GetUploadToken(sha1_hash_b64, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)
	}

	CommitToken, err := api.UploadFile(ctx, filePath, token)
	if err != nil {
		return fmt.Errorf("error uploading file: %w", err)

	}

	mediaKey, err := api.CommitUpload(CommitToken, fileInfo.Name(), sha1_hash_bytes, fileInfo.ModTime().Unix())
	if err != nil {
		return fmt.Errorf("error commiting file: %w", err)
	}

	if len(mediaKey) == 0 {
		return fmt.Errorf("media key not received")
	}

	if AppConfig.DeleteFromHost {
		os.Remove(filePath)
	}

	return nil

}
