package backend

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestSessionUploadOptionsCapturePreferences(t *testing.T) {
	restoreConfigGlobals(t)
	if err := LoadConfig(filepath.Join(t.TempDir(), "gotohp.config")); err != nil {
		t.Fatal(err)
	}
	manager := &ConfigManager{}
	manager.SetProxy("http://proxy.example:8080")
	manager.SetSaver(true)
	manager.SetUseQuota(true)
	manager.SetUploadThreads(7)
	manager.SetPairLivePhotos(true)
	manager.SetSkipIncompleteLivePhotos(false)
	manager.SetUpdateExistingPhotosToLive(true)
	manager.SetAlbumName("Holiday")

	want := UploadOptions{
		Api:     ApiOptions{Proxy: "http://proxy.example:8080", Saver: true, UseQuota: true},
		Threads: 7, PairLivePhotos: true, UpdateExistingPhotosToLive: true,
		AlbumName: "Holiday",
	}
	captured := manager.SessionUploadOptions()
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("session options = %#v, want %#v", captured, want)
	}

	// Settings changed for the next run must not change an in-progress run.
	manager.SetProxy("")
	manager.SetSaver(false)
	manager.SetUseQuota(false)
	manager.SetUploadThreads(2)
	manager.SetAlbumName("")
	manager.SetAlbumAutoMode(true)
	if !reflect.DeepEqual(captured, want) {
		t.Fatalf("captured options changed after updating preferences: %#v", captured)
	}
	next := manager.SessionUploadOptions()
	if next.Api != (ApiOptions{}) || next.Threads != 2 || next.AlbumName != "" || !next.AlbumAutoMode {
		t.Fatalf("next run did not receive updated preferences: %#v", next)
	}
}
