package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"app/backend"
)

func TestUploadRunReturnsFailureWithJSON(t *testing.T) {
	previousConfig, previousPath := backend.AppConfig, backend.ConfigPath
	previousStdout := os.Stdout
	t.Cleanup(func() {
		backend.AppConfig, backend.ConfigPath = previousConfig, previousPath
		os.Stdout = previousStdout
	})
	dir := t.TempDir()
	configPath := filepath.Join(dir, "empty.config")
	if err := os.WriteFile(configPath, []byte("account: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(dir, "photo.jpg")
	// Credential validation fails before the image is decoded or a client is created.
	if err := os.WriteFile(photo, []byte("upload fixture"), 0o600); err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(dir, "stdout-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	os.Stdout = output
	code := Run([]string{"upload", photo, "--config", configPath, "--no-tui"}, Info{ExecutableName: "gotohp-cli"})
	os.Stdout = previousStdout
	if code == 0 {
		t.Error("upload returned success despite a failed file")
	}
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(output)
	var summary uploadSummary
	if err := decoder.Decode(&summary); err != nil {
		t.Fatalf("failure did not preserve JSON summary: %v", err)
	}
	if summary.Total != 1 || summary.Failed != 1 || summary.Succeeded != 0 || summary.Skipped != 0 || len(summary.Results) != 1 {
		t.Fatalf("unexpected failure summary: %+v", summary)
	}
	result := summary.Results[0]
	if result.Path != photo || result.Success || result.Error != "no account is selected" {
		t.Errorf("failed file details were lost: %+v", result)
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		t.Errorf("stdout contains data after the JSON summary: %v", err)
	}
}

func TestFinishUploadExitStatusPreservesJSON(t *testing.T) {
	for _, tc := range []struct {
		name string
		emit func(teaReporter)
		fail bool
		json string
	}{
		{
			name: "mixed success and failure", fail: true,
			emit: func(r teaReporter) {
				r.UploadStart(backend.UploadBatchStart{Total: 2})
				r.FileResult(backend.FileUploadResult{Path: "ok.jpg", MediaKey: "saved-key"})
				r.FileResult(backend.FileUploadResult{Path: "failed.jpg", IsError: true, Error: errors.New("upload refused")})
			},
			json: `{"total":2,"succeeded":1,"failed":1,"skipped":0,"results":[{"path":"ok.jpg","success":true,"mediaKey":"saved-key"},{"path":"failed.jpg","success":false,"error":"upload refused"}]}`,
		},
		{
			name: "album failure after successful upload", fail: true,
			emit: func(r teaReporter) {
				r.UploadStart(backend.UploadBatchStart{Total: 1})
				r.FileResult(backend.FileUploadResult{Path: "ok.jpg", MediaKey: "saved-key"})
				r.AlbumError(backend.AlbumError{AlbumName: "Trip", Error: "album access denied"})
			},
			json: `{"total":1,"succeeded":1,"failed":0,"skipped":0,"results":[{"path":"ok.jpg","success":true,"mediaKey":"saved-key"}],"album":{"name":"Trip","error":"album access denied"}}`,
		},
		{
			name: "success with skip and warning",
			emit: func(r teaReporter) {
				r.UploadStart(backend.UploadBatchStart{Total: 2})
				r.FileResult(backend.FileUploadResult{Path: "ok.jpg", MediaKey: "saved-key"})
				r.FileResult(backend.FileUploadResult{Path: "duplicate.jpg", Skipped: true, SkipCode: "remote-duplicate", SkipReason: "Already uploaded"})
				r.Warning(backend.PreflightWarning{Code: "metadata-unreadable", Message: "Could not read metadata", Paths: []string{"ok.jpg"}})
				r.AlbumComplete(backend.AlbumStatus{AlbumName: "Trip", ItemsAdded: 1, AlbumKeys: []string{"album-key"}, IsComplete: true})
			},
			json: `{"total":2,"succeeded":1,"failed":0,"skipped":1,"results":[{"path":"ok.jpg","success":true,"mediaKey":"saved-key"},{"path":"duplicate.jpg","success":false,"skipped":true,"skipCode":"remote-duplicate","skipReason":"Already uploaded"}],"warnings":[{"paths":["ok.jpg"],"code":"metadata-unreadable","message":"Could not read metadata"}],"album":{"name":"Trip","itemsAdded":1,"albumKeys":["album-key"]}}`,
		},
		{
			name: "empty batch",
			emit: func(r teaReporter) { r.UploadStart(backend.UploadBatchStart{}) },
			json: `{"total":0,"succeeded":0,"failed":0,"skipped":0,"results":[]}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := runReporterProgram(t, tc.emit)
			var output bytes.Buffer
			err := finishUpload(model, &output)
			if tc.fail {
				var exit exitError
				if !errors.As(err, &exit) || exit.code == 0 {
					t.Errorf("failure status = %v, want nonzero CLI exitError", err)
				}
			} else if err != nil {
				t.Errorf("successful run returned an error: %v", err)
			}
			var got, want any
			if err := json.Unmarshal(output.Bytes(), &got); err != nil {
				t.Fatalf("summary is not valid JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.json), &want); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("JSON = %s\nwant %s", output.String(), tc.json)
			}
		})
	}
}

func TestFinishUploadRetainsEarlierAlbumFailure(t *testing.T) {
	model := runReporterProgram(t, func(r teaReporter) {
		r.UploadStart(backend.UploadBatchStart{Total: 1})
		r.FileResult(backend.FileUploadResult{Path: "ok.jpg", MediaKey: "saved-key"})
		r.AlbumError(backend.AlbumError{AlbumName: "First folder", Error: "album access denied"})
		r.AlbumProgress(backend.AlbumStatus{AlbumName: "Second folder", ItemsAdded: 0, TotalItems: 1})
		r.AlbumComplete(backend.AlbumStatus{AlbumName: "Second folder", ItemsAdded: 1, AlbumKeys: []string{"second-album"}, IsComplete: true})
	})
	var output bytes.Buffer
	var exit exitError
	if err := finishUpload(model, &output); !errors.As(err, &exit) || exit.code == 0 {
		t.Errorf("later album success erased the failed exit status: %v", err)
	}
	var summary uploadSummary
	if err := json.Unmarshal(output.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Succeeded != 1 || summary.Failed != 0 || summary.Album == nil || summary.Album.Error != "album access denied" {
		t.Errorf("album failure or successful upload was lost from JSON: %+v", summary)
	}
}
