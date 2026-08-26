package backend

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadConfigMigratesLegacyFlatLayout(t *testing.T) {
	restoreConfigGlobals(t)

	path := filepath.Join(t.TempDir(), "gotohp.config")
	legacy := strings.Join([]string{
		"credentials:",
		"  - Email=person%40example.com&Token=secret",
		"selected: person@example.com",
		"proxy: http://proxy.example",
		"upload_threads: 7",
		"recursive: true",
		"skip_incomplete_live_photos: false",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	got := (&ConfigManager{}).GetConfig()
	if got.Account.Selected != "person@example.com" || len(got.Account.Credentials) != 1 {
		t.Fatalf("account not migrated: %+v", got.Account)
	}
	p := got.Preferences
	if p.Proxy != "http://proxy.example" || p.UploadThreads != 7 || !p.Recursive || p.SkipIncompleteLivePhotos {
		t.Fatalf("preferences not migrated: %+v", p)
	}

	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"account:", "preferences:", "upload_threads: 7"} {
		if !strings.Contains(string(saved), want) {
			t.Fatalf("migrated file missing %q:\n%s", want, saved)
		}
	}
	if strings.HasPrefix(string(saved), "credentials:") {
		t.Fatalf("file still in legacy layout:\n%s", saved)
	}

	// A second load must read the sectioned layout without re-migrating.
	if err := LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig (second): %v", err)
	}
	if again := (&ConfigManager{}).GetConfig(); !reflect.DeepEqual(again, got) {
		t.Fatalf("second load differs:\n%+v\n%+v", again, got)
	}
}

func TestLoadConfigMissingFileUsesDefaults(t *testing.T) {
	restoreConfigGlobals(t)

	path := filepath.Join(t.TempDir(), "gotohp.config")
	if err := LoadConfig(path); err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if got := (&ConfigManager{}).GetConfig(); !reflect.DeepEqual(got, DefaultConfig) {
		t.Fatalf("config = %+v, want defaults", got)
	}
}
