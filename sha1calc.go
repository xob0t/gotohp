package main

import (
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

// CalculateSHA1WithProgress calculates the SHA1 hash of a file and reports progress
func CalculateSHA1WithProgress(filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return []byte{}, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()
	hash := sha1.New()

	// Create a 1MB buffer (1024 * 1024 bytes)
	buffer := make([]byte, 1024*1024)

	// Use io.CopyBuffer with our custom buffer size
	if _, err := io.CopyBuffer(hash, file, buffer); err != nil {
		return []byte{}, fmt.Errorf("error calculating hash: %w", err)
	}

	// Convert the hash to a hex string
	hashInBytes := hash.Sum(nil)[:20]
	return hashInBytes, nil
}
