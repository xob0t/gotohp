package backend

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

var compareFileMu sync.Mutex

func getCompareFileListPath() string {
	if ConfigPath == "" {
		determineConfigPath()
	}
	return filepath.Join(filepath.Dir(ConfigPath), "CompareFileList.csv")
}

// writeCompareFileList writes a list of Google Photos filenames to the local CSV cache
func writeCompareFileList(filepathStr string, filenames []string) error {
	compareFileMu.Lock()
	defer compareFileMu.Unlock()

	if err := os.MkdirAll(filepath.Dir(filepathStr), 0755); err != nil {
		return err
	}

	file, err := os.Create(filepathStr)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	if err := writer.Write([]string{"Filename"}); err != nil {
		return err
	}

	for _, name := range filenames {
		if err := writer.Write([]string{name}); err != nil {
			return err
		}
	}
	return nil
}

// readCompareFileList reads a list of Google Photos filenames from the local CSV cache
func readCompareFileList(filepathStr string) ([]string, error) {
	compareFileMu.Lock()
	defer compareFileMu.Unlock()

	file, err := os.Open(filepathStr)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewWriter(nil) // just mock using csv reader
	_ = reader

	csvReader := csv.NewReader(file)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, err
	}

	var filenames []string
	if len(records) > 1 {
		// Skip header row
		for _, record := range records[1:] {
			if len(record) > 0 {
				filenames = append(filenames, record[0])
			}
		}
	}
	return filenames, nil
}

// AppendToCompareFileList appends a newly uploaded filename to the local CSV cache
func AppendToCompareFileList(filename string) error {
	filePath := getCompareFileListPath()

	compareFileMu.Lock()
	defer compareFileMu.Unlock()

	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}

	// Check if file exists. If not, write header first.
	exists := true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exists = false
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	if !exists {
		if err := writer.Write([]string{"Filename"}); err != nil {
			return err
		}
	}

	if err := writer.Write([]string{filename}); err != nil {
		return err
	}
	return nil
}

// Protobuf payload builders

func encodeVarint(val uint64) []byte {
	var buf []byte
	for val >= 0x80 {
		buf = append(buf, byte(val|0x80))
		val >>= 7
	}
	buf = append(buf, byte(val))
	return buf
}

func makeGetLibStatePayload(syncToken string) []byte {
	rootBytes := make([]byte, len(libStateRootStatic))
	copy(rootBytes, libStateRootStatic)

	if syncToken != "" {
		rootBytes = append(rootBytes, 0x32) // tag 6, wire type 2
		rootBytes = append(rootBytes, encodeVarint(uint64(len(syncToken)))...)
		rootBytes = append(rootBytes, []byte(syncToken)...)
	}

	var payload []byte
	payload = append(payload, 0x0a) // tag 1, wire type 2
	payload = append(payload, encodeVarint(uint64(len(rootBytes)))...)
	payload = append(payload, rootBytes...)

	payload = append(payload, 0x12) // tag 2, wire type 2
	payload = append(payload, encodeVarint(uint64(len(libStateField2Static)))...)
	payload = append(payload, libStateField2Static...)

	return payload
}

func makeGetLibPageInitPayload(resumeToken string) []byte {
	rootBytes := make([]byte, len(libPageInitRootStatic))
	copy(rootBytes, libPageInitRootStatic)

	if resumeToken != "" {
		rootBytes = append(rootBytes, 0x22) // tag 4, wire type 2
		rootBytes = append(rootBytes, encodeVarint(uint64(len(resumeToken)))...)
		rootBytes = append(rootBytes, []byte(resumeToken)...)
	}

	var payload []byte
	payload = append(payload, 0x0a)
	payload = append(payload, encodeVarint(uint64(len(rootBytes)))...)
	payload = append(payload, rootBytes...)

	payload = append(payload, 0x12)
	payload = append(payload, encodeVarint(uint64(len(libPageInitField2Static)))...)
	payload = append(payload, libPageInitField2Static...)

	return payload
}

// Protobuf parsing helpers

func readVarint(data []byte) (uint64, int) {
	var val uint64
	var shift uint
	for i, b := range data {
		val |= uint64(b&0x7f) << shift
		if b&0x80 == 0 {
			return val, i + 1
		}
		shift += 7
		if shift >= 64 {
			return 0, -1
		}
	}
	return 0, -1
}

func parseProto(data []byte, onField func(num int, wireType int, val []byte)) error {
	for len(data) > 0 {
		key, n := readVarint(data)
		if n <= 0 {
			return fmt.Errorf("invalid varint key")
		}
		data = data[n:]
		fieldNum := int(key >> 3)
		wireType := int(key & 0x7)
		switch wireType {
		case 0: // Varint
			_, n2 := readVarint(data)
			if n2 <= 0 {
				return fmt.Errorf("invalid varint value")
			}
			onField(fieldNum, wireType, data[:n2])
			data = data[n2:]
		case 1: // 64-bit
			if len(data) < 8 {
				return fmt.Errorf("unexpected EOF for 64-bit field")
			}
			onField(fieldNum, wireType, data[:8])
			data = data[8:]
		case 2: // Length-delimited
			length, n2 := readVarint(data)
			if n2 <= 0 {
				return fmt.Errorf("invalid length varint")
			}
			data = data[n2:]
			if int64(len(data)) < int64(length) {
				return fmt.Errorf("unexpected EOF for length-delimited field")
			}
			onField(fieldNum, wireType, data[:length])
			data = data[length:]
		case 5: // 32-bit
			if len(data) < 4 {
				return fmt.Errorf("unexpected EOF for 32-bit field")
			}
			onField(fieldNum, wireType, data[:4])
			data = data[4:]
		default:
			return fmt.Errorf("unsupported wire type: %d", wireType)
		}
	}
	return nil
}

func extractLibraryFilenamesAndTokens(data []byte) (filenames []string, resumeToken string, syncToken string, err error) {
	err = parseProto(data, func(num int, wireType int, val []byte) {
		if num == 1 && wireType == 2 { // root
			_ = parseProto(val, func(num2 int, wireType2 int, val2 []byte) {
				switch num2 {
				case 1: // resume_token
					if wireType2 == 2 {
						resumeToken = string(val2)
					}
				case 6: // sync_token
					if wireType2 == 2 {
						syncToken = string(val2)
					}
				case 2: // media_items (repeated)
					if wireType2 == 2 {
						_ = parseProto(val2, func(num3 int, wireType3 int, val3 []byte) {
							if num3 == 2 && wireType3 == 2 { // metadata (d2)
								_ = parseProto(val3, func(num4 int, wireType4 int, val4 []byte) {
									if num4 == 4 && wireType4 == 2 { // file_name
										filenames = append(filenames, string(val4))
									}
								})
							}
						})
					}
				}
			})
		}
	})
	return
}

func (a *Api) doLibStateRequest(ctx context.Context, payload []byte) ([]byte, error) {
	bearerToken, err := a.BearerToken()
	if err != nil {
		return nil, fmt.Errorf("failed to get bearer token: %w", err)
	}

	headers := map[string]string{
		"Accept-Encoding":          "gzip",
		"Accept-Language":          a.language,
		"Content-Type":             "application/x-protobuf",
		"User-Agent":               a.userAgent,
		"Authorization":            "Bearer " + bearerToken,
		"x-goog-ext-173412678-bin": "CgcIAhClARgC",
		"x-goog-ext-174067345-bin": "CgIIAg==",
	}

	retryConfig := DefaultRetryConfig()
	var lastErr error

	for attempt := 0; attempt <= retryConfig.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt > 0 {
			delay := CalculateBackoff(attempt-1, retryConfig)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST",
			"https://photosdata-pa.googleapis.com/6439526531001121323/18047484249733410717",
			bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := a.client.Do(req)
		if !ShouldRetry(resp, err) {
			if err != nil {
				return nil, fmt.Errorf("request failed: %w", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode < 200 || resp.StatusCode >= 300 {
				body, _ := ReadResponseBody(resp)
				return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, string(body))
			}

			bodyBytes, err := ReadResponseBody(resp)
			if err != nil {
				return nil, fmt.Errorf("failed to read response body: %w", err)
			}
			return bodyBytes, nil
		}

		if err != nil {
			lastErr = err
		} else {
			body, _ := ReadResponseBody(resp)
			lastErr = fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
			resp.Body.Close()
		}
	}

	return nil, fmt.Errorf("request failed after %d attempts: %w", retryConfig.MaxRetries+1, lastErr)
}

// FetchAllFilenames fetches all filenames from Google Photos
func (a *Api) FetchAllFilenames(ctx context.Context, progressCallback func(fetchedCount int)) ([]string, error) {
	var filenames []string

	// Page 1
	payload := makeGetLibStatePayload("")
	body, err := a.doLibStateRequest(ctx, payload)
	if err != nil {
		return nil, err
	}

	pageNames, resumeToken, _, err := extractLibraryFilenamesAndTokens(body)
	if err != nil {
		return nil, err
	}
	filenames = append(filenames, pageNames...)
	if progressCallback != nil {
		progressCallback(len(filenames))
	}

	// Subsequent pages
	for resumeToken != "" {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		payload = makeGetLibPageInitPayload(resumeToken)
		body, err = a.doLibStateRequest(ctx, payload)
		if err != nil {
			return nil, err
		}
		pageNames, resumeToken, _, err = extractLibraryFilenamesAndTokens(body)
		if err != nil {
			return nil, err
		}
		filenames = append(filenames, pageNames...)
		if progressCallback != nil {
			progressCallback(len(filenames))
		}
	}

	return filenames, nil
}

type CompareProgress struct {
	Status string `json:"Status"` // "loading_cache", "fetching", "comparing"
	Count  int    `json:"Count"`
}

type CompareResult struct {
	TotalLocal        int    `json:"TotalLocal"`
	TotalGooglePhotos int    `json:"TotalGooglePhotos"`
	MissingCount      int    `json:"MissingCount"`
	MissingFilesPath  string `json:"MissingFilesPath"`
}

