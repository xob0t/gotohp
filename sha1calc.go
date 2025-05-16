package main

import (
	"context"
	"crypto/sha1"
	"fmt"
	"io"
	"os"
)

// Check context every 64MB to balance cancellation responsiveness and performance
const contextCheckInterval = 64 * 1024 * 1024 // 64MB

func CalculateSHA1(ctx context.Context, filePath string) ([]byte, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	hash := sha1.New()
	buffer := make([]byte, 1024*1024) // 1MB buffer
	bytesSinceLastCheck := 0

	for {
		// Check context every 64MB processed
		if bytesSinceLastCheck >= contextCheckInterval {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				bytesSinceLastCheck = 0
			}
		}

		n, err := file.Read(buffer)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("error reading file: %w", err)
		}

		if _, err := hash.Write(buffer[:n]); err != nil {
			return nil, fmt.Errorf("error calculating hash: %w", err)
		}

		bytesSinceLastCheck += n
	}

	return hash.Sum(nil)[:20], nil
}
