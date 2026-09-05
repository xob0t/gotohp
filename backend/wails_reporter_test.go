//go:build !cli

package backend

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

func TestWailsReporterFrontendEvents(t *testing.T) {
	// Wails owns process-wide application state and signal handlers. Isolate
	// initialization from the other backend tests; never run a desktop or window.
	const childEnvironment = "GOTOHP_TEST_WAILS_REPORTER"
	if os.Getenv(childEnvironment) != "1" {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWailsReporterFrontendEvents$", "-test.count=1")
		command.Env = append(os.Environ(), childEnvironment+"=1")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("Wails event test: %v\n%s", err, output)
		}
		return
	}

	app := application.New(application.Options{
		Name:   "gotohp reporter test",
		Logger: slog.New(slog.DiscardHandler),
	})
	reporter := NewWailsReporter(app)
	file := FileUploadResult{
		IsError: true, IsLivePhoto: true, Path: "photo.heic",
		Paths: []string{"photo.heic", "photo.mov"},
		Error: errors.New("upload failed"), ErrorMessage: "upload failed",
	}
	progress := AlbumStatus{AlbumName: "Holiday", ItemsAdded: 1, TotalItems: 2, AlbumKeys: []string{"album-key"}}
	complete := AlbumStatus{AlbumName: "Holiday", ItemsAdded: 2, TotalItems: 2, AlbumKeys: []string{"album-key"}, IsComplete: true}
	albumError := AlbumError{AlbumName: "Holiday", Error: "album unavailable"}

	for _, test := range []struct {
		name       string
		want       any
		jsonFields string
		emit       func()
	}{
		{"FileStatus", file, `{"IsError":true,"IsLivePhoto":true,"Path":"photo.heic","Paths":["photo.heic","photo.mov"],"ErrorMessage":"upload failed"}`, func() { reporter.FileResult(file) }},
		{"albumProgress", progress, `{"AlbumName":"Holiday","ItemsAdded":1,"TotalItems":2,"AlbumKeys":["album-key"],"IsComplete":false}`, func() { reporter.AlbumProgress(progress) }},
		{"albumComplete", complete, `{"AlbumName":"Holiday","ItemsAdded":2,"TotalItems":2,"AlbumKeys":["album-key"],"IsComplete":true}`, func() { reporter.AlbumComplete(complete) }},
		{"albumError", albumError, `{"AlbumName":"Holiday","Error":"album unavailable"}`, func() { reporter.AlbumError(albumError) }},
		{"uploadStop", nil, `null`, reporter.UploadStop},
	} {
		t.Run(test.name, func(t *testing.T) {
			received := make(chan *application.CustomEvent, 1)
			off := app.Event.On(test.name, func(event *application.CustomEvent) { received <- event })
			defer off()
			test.emit()
			select {
			case event := <-received:
				if event.Name != test.name || !reflect.DeepEqual(event.Data, test.want) {
					t.Fatalf("event = %#v, want %s with %#v", event, test.name, test.want)
				}
				encoded, err := json.Marshal(event.Data)
				if err != nil {
					t.Fatal(err)
				}
				var actual, expected map[string]any
				if err := json.Unmarshal(encoded, &actual); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(test.jsonFields), &expected); err != nil {
					t.Fatal(err)
				}
				for field, value := range expected {
					if !reflect.DeepEqual(actual[field], value) {
						t.Errorf("frontend field %s = %#v, want %#v", field, actual[field], value)
					}
				}
				if test.name == "FileStatus" {
					if _, exists := actual["Error"]; exists {
						t.Error("FileStatus exposed the internal Go error instead of only ErrorMessage")
					}
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("frontend event %s was not emitted", test.name)
			}
		})
	}
}
