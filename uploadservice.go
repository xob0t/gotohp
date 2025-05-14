package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type UploadService struct{}

type UploadStatus struct {
	IsError bool
	Error   error
}

func (g *UploadService) Upload(targetPaths []string) []string {
	supportedFiles, _ := filterGooglePhotosFiles(targetPaths, false)
	for _, path := range supportedFiles {
		App.Logger.Info(path)
	}
	return supportedFiles
}

func Filter(targetPaths []string) ([]string, error) {
	supportedFiles, err := filterGooglePhotosFiles(targetPaths, false)
	if err != nil {
		return []string{}, err
	}

	return supportedFiles, nil
}

// isGooglePhotosSupported checks if a file extension is supported by Google Photos
func isGooglePhotosSupported(filename string) bool {
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
	}

	// Supported video formats
	videoFormats := []string{
		"3gp", "3g2", "asf", "avi", "divx",
		"m2t", "m2ts", "m4v", "mkv", "mmv",
		"mod", "mov", "mp4", "mpg", "mpeg",
		"mts", "tod", "wmv",
	}

	// Check if extension is in either supported format
	return slices.Contains(photoFormats, ext) || slices.Contains(videoFormats, ext)
}

func scanDirectory(path string, recursive bool) ([]string, error) {
	var files []string

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if recursive {
				subFiles, err := scanDirectory(fullPath, recursive)
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
func filterGooglePhotosFiles(paths []string, recursive bool) ([]string, error) {
	var supportedFiles []string

	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			return nil, fmt.Errorf("error accessing path %s: %v", path, err)
		}

		if fileInfo.IsDir() {
			files, err := scanDirectory(path, recursive)
			if err != nil {
				return nil, fmt.Errorf("error scanning directory %s: %v", path, err)
			}

			for _, file := range files {
				if isGooglePhotosSupported(file) {
					supportedFiles = append(supportedFiles, file)
				}
			}
		} else {
			if isGooglePhotosSupported(fileInfo.Name()) {
				supportedFiles = append(supportedFiles, path)
			}
		}
	}

	return supportedFiles, nil
}

func Upload(filePath string) error {
	testAuthData := os.Getenv("GP_AUTH_DATA")

	sha1_hash_bytes, err := CalculateSHA1WithProgress(filePath)
	if err != nil {
		fmt.Println("Error calculating hash file:", err)
	}

	api, _ := NewApi(testAuthData, GlobalSettingsConfig.Proxy, "en-US")
	sha1_hash_b64 := base64.StdEncoding.EncodeToString([]byte(sha1_hash_bytes))

	if err != nil {
		panic(err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		fmt.Println("Error opening file:", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Println("Error getting file info:", err)
	}

	token, err := api.GetUploadToken(sha1_hash_b64, fileInfo.Size())
	if err != nil {
		fmt.Println("Error uploading file:", err)
	}
	App.Logger.Info("token" + token)

	CommitToken, err := api.UploadFile(filePath, token)
	if err != nil {
		fmt.Println("Error uploading file:", err)
	}

	mediakey, err := api.CommitUpload(CommitToken, fileInfo.Name(), sha1_hash_bytes, "original", "Google", "Pixel XL", fileInfo.ModTime().Unix())
	if err != nil {
		fmt.Println("Error commiting file:", err)
	}

	App.Logger.Info("mediakey" + mediakey)
	return nil

}
