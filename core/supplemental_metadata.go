package core

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const supplementalMetadataSuffix = ".supplemental-metadata.json"

type supplementalMetadata struct {
	PhotoTakenTime struct {
		Timestamp json.RawMessage `json:"timestamp"`
	} `json:"photoTakenTime"`
}

func parseTimestampFromSupplementalMetadata(mediaPath string) (time.Time, bool, error) {
	metadataPath := mediaPath + supplementalMetadataSuffix
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		if os.IsNotExist(err) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("read %q: %w", metadataPath, err)
	}

	var metadata supplementalMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return time.Time{}, false, fmt.Errorf("parse %q: %w", metadataPath, err)
	}
	if len(metadata.PhotoTakenTime.Timestamp) == 0 {
		return time.Time{}, false, fmt.Errorf("parse %q: photoTakenTime.timestamp is missing", metadataPath)
	}

	var timestamp string
	if metadata.PhotoTakenTime.Timestamp[0] == '"' {
		if err := json.Unmarshal(metadata.PhotoTakenTime.Timestamp, &timestamp); err != nil {
			return time.Time{}, false, fmt.Errorf("parse %q photoTakenTime.timestamp: %w", metadataPath, err)
		}
	} else {
		timestamp = string(metadata.PhotoTakenTime.Timestamp)
	}
	timestamp = strings.TrimSpace(timestamp)
	seconds, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return time.Time{}, false, fmt.Errorf("parse %q photoTakenTime.timestamp %q: %w", metadataPath, timestamp, err)
	}
	return time.Unix(seconds, 0), true, nil
}

func resolveUploadTime(mediaPath, filename string, fallback time.Time, fromFilename, fromSupplementalMetadata bool) (time.Time, error) {
	if fromSupplementalMetadata {
		parsed, found, err := parseTimestampFromSupplementalMetadata(mediaPath)
		if err != nil {
			return time.Time{}, err
		}
		if found {
			return parsed, nil
		}
	}
	if fromFilename {
		if parsed, ok := parseTimestampFromFilename(filename); ok {
			return parsed, nil
		}
	}
	return fallback, nil
}
