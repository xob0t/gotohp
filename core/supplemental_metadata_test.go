package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseTimestampFromSupplementalMetadata(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "Kaitlin_Stapleton_Spaghetti.jpg")
	metadataPath := mediaPath + supplementalMetadataSuffix
	if err := os.WriteFile(metadataPath, []byte(`{"photoTakenTime":{"timestamp":"1341774123"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, found, err := parseTimestampFromSupplementalMetadata(mediaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !found || got.Unix() != 1341774123 {
		t.Fatalf("supplemental timestamp = %v, found %v", got, found)
	}
}

func TestParseTimestampFromSupplementalMetadataAcceptsNumericTimestamp(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(mediaPath+supplementalMetadataSuffix, []byte(`{"photoTakenTime":{"timestamp":1341774123}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, found, err := parseTimestampFromSupplementalMetadata(mediaPath)
	if err != nil || !found || got.Unix() != 1341774123 {
		t.Fatalf("supplemental timestamp = %v, found %v, error %v", got, found, err)
	}
}

func TestResolveUploadTimeFallsBackWhenSupplementalMetadataIsMissing(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "photo.jpg")
	fallback := time.Unix(123, 0)

	got, err := resolveUploadTime(mediaPath, filepath.Base(mediaPath), fallback, false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(fallback) {
		t.Fatalf("upload time = %v, want fallback %v", got, fallback)
	}
}

func TestParseTimestampFromSupplementalMetadataRejectsInvalidSidecar(t *testing.T) {
	mediaPath := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(mediaPath+supplementalMetadataSuffix, []byte(`{"photoTakenTime":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, found, err := parseTimestampFromSupplementalMetadata(mediaPath); err == nil || found {
		t.Fatalf("invalid sidecar returned found %v, error %v", found, err)
	}
}
