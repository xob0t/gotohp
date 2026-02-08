package backend

import (
	"io"
	"sync/atomic"
	"time"
)

// ProgressReader wraps an io.Reader to track read progress without buffering.
// It streams data directly through, only counting bytes as they pass.
type ProgressReader struct {
	reader       io.Reader
	total        int64
	read         atomic.Int64
	lastEmit     time.Time
	emitInterval time.Duration
	onProgress   func(bytesRead, totalBytes int64)
}

// NewProgressReader creates a new ProgressReader that wraps the given reader.
// The onProgress callback is called periodically (throttled) with progress updates.
func NewProgressReader(reader io.Reader, total int64, onProgress func(bytesRead, totalBytes int64)) *ProgressReader {
	return &ProgressReader{
		reader:       reader,
		total:        total,
		emitInterval: 100 * time.Millisecond, // Throttle to max 10 updates/second
		onProgress:   onProgress,
	}
}

// Read implements io.Reader, passing through to the underlying reader
// while tracking progress.
func (pr *ProgressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	if n > 0 {
		currentRead := pr.read.Add(int64(n))

		// Throttle progress updates to avoid overwhelming the event system
		now := time.Now()
		if now.Sub(pr.lastEmit) >= pr.emitInterval || err == io.EOF {
			pr.lastEmit = now
			if pr.onProgress != nil {
				pr.onProgress(currentRead, pr.total)
			}
		}
	}
	return n, err
}

// BytesRead returns the total number of bytes read so far.
func (pr *ProgressReader) BytesRead() int64 {
	return pr.read.Load()
}
