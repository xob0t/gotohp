package main

import (
	"fmt"
	"os"
	"strings"
)

func parseUploadArgs(args []string) ([]string, cliConfig, error) {
	config := cliConfig{
		threads:  3,
		logLevel: "info",
	}
	paths := make([]string, 0, len(args))

	for index := 0; index < len(args); index++ {
		argument := args[index]
		nextValue := func() (string, error) {
			if index+1 >= len(args) {
				return "", fmt.Errorf("flag %s requires a value", argument)
			}
			index++
			return args[index], nil
		}

		switch argument {
		case "--recursive", "-r":
			config.recursive = true
		case "--force", "-f":
			config.forceUpload = true
		case "--delete", "-d":
			config.deleteFromHost = true
		case "--disable-filter", "-df":
			config.disableUnsupportedFilesFilter = true
		case "--date-from-filename":
			config.setDateFromFilename = true
		case "--no-tui":
			config.noTUI = true
		case "--pair-live-photos":
			config.pairLivePhotos = true
			if !config.skipIncompleteLivePhotosSet {
				config.skipIncompleteLivePhotos = true
				config.skipIncompleteLivePhotosSet = true
			}
		case "--skip-incomplete-live-photos":
			config.skipIncompleteLivePhotos = true
			config.skipIncompleteLivePhotosSet = true
		case "--upload-incomplete-live-photos":
			config.skipIncompleteLivePhotos = false
			config.skipIncompleteLivePhotosSet = true
		case "--update-existing-photos-to-live":
			config.updateExistingPhotosToLive = true
		case "--ignore-apple-metadata":
			config.ignoreAppleMetadata = true
		case "--exclude", "-e":
			value, err := nextValue()
			if err != nil {
				return nil, cliConfig{}, err
			}
			config.excludePattern = value
		case "--threads", "-t":
			value, err := nextValue()
			if err != nil {
				return nil, cliConfig{}, err
			}
			if _, err := fmt.Sscanf(value, "%d", &config.threads); err != nil || config.threads < 1 {
				return nil, cliConfig{}, fmt.Errorf("threads must be a positive integer, got %q", value)
			}
		case "--log-level", "-l":
			value, err := nextValue()
			if err != nil {
				return nil, cliConfig{}, err
			}
			config.logLevel = strings.ToLower(value)
		case "--config", "-c":
			value, err := nextValue()
			if err != nil {
				return nil, cliConfig{}, err
			}
			config.configPath = value
		case "--album", "-a":
			value, err := nextValue()
			if err != nil {
				return nil, cliConfig{}, err
			}
			config.albumName = value
		default:
			if strings.HasPrefix(argument, "-") {
				return nil, cliConfig{}, fmt.Errorf("unknown upload flag %q", argument)
			}
			paths = append(paths, argument)
		}
	}

	if config.updateExistingPhotosToLive && !config.pairLivePhotos {
		return nil, cliConfig{}, fmt.Errorf("--update-existing-photos-to-live requires --pair-live-photos")
	}
	if config.ignoreAppleMetadata && !config.pairLivePhotos {
		return nil, cliConfig{}, fmt.Errorf("--ignore-apple-metadata requires --pair-live-photos")
	}
	if len(paths) == 0 {
		return nil, cliConfig{}, fmt.Errorf("at least one file or directory path is required")
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return nil, cliConfig{}, fmt.Errorf("invalid upload path %q: %w", path, err)
		}
	}
	return paths, config, nil
}
