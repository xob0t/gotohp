package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	"app/backend"

	tea "github.com/charmbracelet/bubbletea"
)

// Exercise the reporter through Bubble Tea's event loop, including its quit
// command, without opening a terminal or starting an upload manager.
func runReporterProgram(t *testing.T, emit func(teaReporter)) uploadModel {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	program := tea.NewProgram(initialModel(), tea.WithContext(ctx), tea.WithInput(nil),
		tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
	defer program.Kill()
	sent := make(chan struct{})
	go func() {
		defer close(sent)
		reporter := teaReporter{p: program}
		emit(reporter)
		reporter.UploadStop()
	}()
	final, err := program.Run()
	cancel() // Release any blocked Send if the program exited before UploadStop.
	select {
	case <-sent:
	case <-time.After(time.Second):
		t.Fatal("reporter goroutine did not finish")
	}
	if err != nil {
		t.Fatalf("program did not finish normally after UploadStop: %v", err)
	}
	model, ok := final.(uploadModel)
	if !ok || !model.quitting {
		t.Fatalf("UploadStop did not return a completed upload model: %T", final)
	}
	return model
}

func assertReporterJSON(t *testing.T, model uploadModel, expected string) {
	t.Helper()
	encoded, err := json.Marshal(buildUploadSummary(model))
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(expected), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("summary = %s\nwant %s", encoded, expected)
	}
}

func TestTeaReporterPreservesFileResultsAndWarnings(t *testing.T) {
	model := runReporterProgram(t, func(r teaReporter) {
		r.UploadStart(backend.UploadBatchStart{}) // Preflight begins before the total is known.
		r.UploadStart(backend.UploadBatchStart{Total: 3, TotalBytes: 4096})
		r.ThreadStatus(backend.ThreadStatus{WorkerID: 2, Status: "uploading", FileName: "pair.heic"})
		r.FileResult(backend.FileUploadResult{
			Path: "photos/pair.heic", Paths: []string{"photos/pair.heic", "photos/pair.mov"},
			IsLivePhoto: true, MediaKey: "uploaded-pair-key",
		})
		r.FileResult(backend.FileUploadResult{
			Path: "photos/broken.jpg", Paths: []string{"photos/broken.jpg"},
			IsError: true, Error: errors.New("upload rejected"), ErrorMessage: "upload rejected",
		})
		r.FileResult(backend.FileUploadResult{
			Path: "photos/duplicate.jpg", Paths: []string{"photos/duplicate.jpg"},
			Skipped: true, MediaKey: "existing-key", SkipCode: "remote-duplicate",
			SkipReason: "Already in the library",
		})
		r.Warning(backend.PreflightWarning{
			Paths: []string{"photos/orphan.mov"}, Code: "metadata-unreadable", Message: "Could not read the content identifier",
		})
		// These warnings duplicate skip results and are intentionally omitted from JSON.
		for _, code := range []string{"incomplete-live-photo-skipped", "ambiguous-filename-stem"} {
			r.Warning(backend.PreflightWarning{Paths: []string{"photos/orphan.mov"}, Code: code, Message: "Skipped during preflight"})
		}
	})
	if model.currentFiles[2] != "pair.heic" || model.workers[2] != "[2] uploading: pair.heic" {
		t.Errorf("worker progress was lost: files=%v, workers=%v", model.currentFiles, model.workers)
	}
	if len(model.warnings) != 3 {
		t.Errorf("model retained %d warnings, want 3 before summary filtering", len(model.warnings))
	}
	assertReporterJSON(t, model, `{
		"total":3,"succeeded":1,"failed":1,"skipped":1,
		"results":[
			{"path":"photos/pair.heic","paths":["photos/pair.heic","photos/pair.mov"],"success":true,"mediaKey":"uploaded-pair-key"},
			{"path":"photos/broken.jpg","paths":["photos/broken.jpg"],"success":false,"error":"upload rejected"},
			{"path":"photos/duplicate.jpg","paths":["photos/duplicate.jpg"],"success":false,"skipped":true,"mediaKey":"existing-key","skipCode":"remote-duplicate","skipReason":"Already in the library"}
		],
		"warnings":[{"paths":["photos/orphan.mov"],"code":"metadata-unreadable","message":"Could not read the content identifier"}]
	}`)
}

func TestTeaReporterPreservesAlbumEvents(t *testing.T) {
	for _, tc := range []struct {
		name     string
		finish   func(teaReporter)
		complete bool
		added    int
		json     string
	}{
		{
			name: "progress", finish: func(teaReporter) {}, added: 2,
			json: `{"total":0,"succeeded":0,"failed":0,"skipped":0,"results":[],"album":{"name":"Trip","itemsAdded":2}}`,
		},
		{
			name: "complete", complete: true, added: 5,
			finish: func(r teaReporter) {
				r.AlbumComplete(backend.AlbumStatus{AlbumName: "Trip", ItemsAdded: 5, TotalItems: 5, AlbumKeys: []string{"album-one", "album-two"}, IsComplete: true})
			},
			json: `{"total":0,"succeeded":0,"failed":0,"skipped":0,"results":[],"album":{"name":"Trip","itemsAdded":5,"albumKeys":["album-one","album-two"]}}`,
		},
		{
			name: "error", added: 2,
			finish: func(r teaReporter) {
				r.AlbumError(backend.AlbumError{AlbumName: "Trip", Error: "album access denied"})
			},
			json: `{"total":0,"succeeded":0,"failed":0,"skipped":0,"results":[],"album":{"name":"Trip","itemsAdded":2,"error":"album access denied"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := runReporterProgram(t, func(r teaReporter) {
				r.UploadStart(backend.UploadBatchStart{})
				r.AlbumProgress(backend.AlbumStatus{AlbumName: "Trip", ItemsAdded: 2, TotalItems: 5})
				tc.finish(r)
			})
			if model.albumTotalItems != 5 || model.albumItemsAdded != tc.added || model.albumComplete != tc.complete {
				t.Errorf("album progress = %d/%d, complete=%t; want %d/5, complete=%t", model.albumItemsAdded, model.albumTotalItems, model.albumComplete, tc.added, tc.complete)
			}
			assertReporterJSON(t, model, tc.json)
		})
	}
}
