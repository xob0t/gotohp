package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"app/backend"

	"github.com/spf13/cobra"
)

func TestUploadRunIgnoresGUIPreferences(t *testing.T) {
	previousConfig, previousPath := backend.AppConfig, backend.ConfigPath
	previousStdout := os.Stdout
	t.Cleanup(func() {
		backend.AppConfig, backend.ConfigPath = previousConfig, previousPath
		os.Stdout = previousStdout
	})
	dir := t.TempDir()
	configPath := filepath.Join(dir, "gui.config")
	config := []byte("account: {}\npreferences:\n  recursive: true\n  disable_unsupported_files_filter: true\n")
	if err := os.WriteFile(configPath, config, 0o600); err != nil {
		t.Fatal(err)
	}
	mediaDir := filepath.Join(dir, "media")
	if err := os.MkdirAll(filepath.Join(mediaDir, "nested"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"notes.txt", "nested/photo.jpg"} {
		if err := os.WriteFile(filepath.Join(mediaDir, filepath.FromSlash(name)), []byte("fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	output, err := os.CreateTemp(dir, "stdout-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	os.Stdout = output
	// Run the actual command, including config loading and the upload manager.
	// Defaults select neither file. Even if GUI settings leak, the empty
	// credential store makes NewApi fail before any network request.
	code := Run([]string{"upload", mediaDir, "--config", configPath, "--no-tui"}, Info{ExecutableName: "gotohp-cli"})
	os.Stdout = previousStdout
	if code != 0 {
		t.Fatalf("upload exit code = %d, want 0", code)
	}
	if _, err := output.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	var summary uploadSummary
	if err := json.NewDecoder(output).Decode(&summary); err != nil {
		t.Fatalf("decode upload summary: %v", err)
	}
	if summary.Total != 0 || len(summary.Results) != 0 || summary.Failed != 0 {
		t.Fatalf("GUI preferences changed CLI file selection: %+v", summary)
	}
	if !backend.AppConfig.Preferences.Recursive || !backend.AppConfig.Preferences.DisableUnsupportedFilesFilter {
		t.Fatal("actual upload command did not load the GUI fixture")
	}
	after, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, config) {
		t.Fatal("CLI upload rewrote the GUI config")
	}
}

// Parse through the real command tree, stopping before the upload starts so
// these tests never contact Google or delete files.
func parsedUploadOptions(t *testing.T, args ...string) (backend.UploadOptions, []string) {
	t.Helper()
	root := newRootCommand(Info{ExecutableName: "gotohp-cli"})
	upload, _, err := root.Find([]string{"upload"})
	if err != nil {
		t.Fatal(err)
	}
	var opts backend.UploadOptions
	var paths []string
	called := false
	upload.RunE = func(cmd *cobra.Command, args []string) error {
		called = true
		opts = uploadOptionsFromFlags(cmd.Flags())
		paths = args
		return validateUploadOptions(cmd.Flags(), opts)
	}
	root.SetArgs(normalizeLegacyArgs(root, append([]string{"upload"}, args...)))
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("upload command did not execute")
	}
	return opts, paths
}

func TestUploadDefaultsIgnoreLoadedGUIPreferences(t *testing.T) {
	// Config is global. Keep this test serial and restore the original state
	// without loading or writing the user's real config file.
	previousConfig, previousPath := backend.AppConfig, backend.ConfigPath
	t.Cleanup(func() {
		backend.AppConfig, backend.ConfigPath = previousConfig, previousPath
	})
	configPath := filepath.Join(t.TempDir(), "gui.config")
	config := `account:
  selected: gui@example.test
preferences:
  proxy: http://gui.invalid:9000
  use_quota: true
  saver: true
  recursive: true
  force_upload: true
  pair_live_photos: true
  skip_incomplete_live_photos: false
  update_existing_photos_to_live: true
  upload_threads: 17
  delete_from_host: true
  disable_unsupported_files_filter: true
  set_date_from_filename: true
  exclude_pattern: nested
`
	if err := os.WriteFile(configPath, []byte(config), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := backend.LoadConfig(configPath); err != nil {
		t.Fatal(err)
	}
	loaded := backend.AppConfig
	if !loaded.Preferences.Recursive || !loaded.Preferences.DeleteFromHost || loaded.Preferences.UploadThreads != 17 || loaded.Account.Selected != "gui@example.test" {
		t.Fatalf("GUI fixture was not loaded: %+v", loaded)
	}
	dir := uploadFilterFixture(t)
	opts, paths := parsedUploadOptions(t, dir)
	want := backend.UploadOptions{Threads: 3, SkipIncompleteLivePhotos: true}
	if !reflect.DeepEqual(opts, want) {
		t.Fatalf("CLI defaults inherited GUI settings: got %+v, want %+v", opts, want)
	}
	assertUploadFiles(t, paths, opts, filepath.Join(dir, "root.jpg"))
	if !reflect.DeepEqual(backend.AppConfig, loaded) || backend.ConfigPath != configPath {
		t.Fatal("parsing CLI options changed the loaded GUI config")
	}
}

func TestUploadFlagsPropagateToRunOptions(t *testing.T) {
	opts, paths := parsedUploadOptions(t, "photo.jpg",
		"--account", "cli@example.test", "--proxy", "http://cli.invalid:8080",
		"--saver", "--use-quota", "-r", "-t", "7", "-f", "-d",
		"--disable-filter", "--date-from-filename", "-e", "excluded",
		"--pair-live-photos", "--upload-incomplete-live-photos",
		"--update-existing-photos-to-live", "--ignore-apple-metadata", "-a", "Trip")
	want := backend.UploadOptions{
		Api: backend.ApiOptions{
			Account: "cli@example.test", Proxy: "http://cli.invalid:8080", Saver: true, UseQuota: true,
		},
		Recursive: true, Threads: 7, ForceUpload: true, DeleteFromHost: true,
		DisableUnsupportedFilesFilter: true, SetDateFromFilename: true, ExcludePattern: "excluded",
		PairLivePhotos: true, SkipIncompleteLivePhotos: false,
		UpdateExistingPhotosToLive: true, IgnoreAppleMetadata: true, AlbumName: "Trip",
	}
	if !reflect.DeepEqual(opts, want) || !slices.Equal(paths, []string{"photo.jpg"}) {
		t.Fatalf("parsed upload = %+v, %q; want %+v, [photo.jpg]", opts, paths, want)
	}
	auto, _ := parsedUploadOptions(t, "photo.jpg", "--album", "AuTo", "--pair-live-photos")
	if !auto.AlbumAutoMode || auto.AlbumName != "" || !auto.SkipIncompleteLivePhotos {
		t.Fatalf("AUTO album or default incomplete-pair policy lost: %+v", auto)
	}
}

func TestUploadFlagsControlFileSelection(t *testing.T) {
	dir := uploadFilterFixture(t)
	cases := []struct {
		name  string
		flags []string
		want  []string
	}{
		{"default", nil, []string{"root.jpg"}},
		{"recursive", []string{"-r"}, []string{"root.jpg", "nested/child.jpg", "excluded/skip.jpg"}},
		{"exclude directory", []string{"-r", "-e", "excluded"}, []string{"root.jpg", "nested/child.jpg"}},
		{"disable extension filter", []string{"--disable-filter"}, []string{"root.jpg", "notes.txt"}},
		{"combined selection", []string{"-r", "-e", "excluded", "--disable-filter"}, []string{"root.jpg", "notes.txt", "nested/child.jpg"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts, paths := parsedUploadOptions(t, append([]string{dir}, tc.flags...)...)
			want := make([]string, len(tc.want))
			for i, name := range tc.want {
				want[i] = filepath.Join(dir, filepath.FromSlash(name))
			}
			assertUploadFiles(t, paths, opts, want...)
		})
	}
}

func uploadFilterFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"root.jpg", "notes.txt", "nested/child.jpg", "excluded/skip.jpg"} {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("filter fixture"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func assertUploadFiles(t *testing.T, paths []string, opts backend.UploadOptions, want ...string) {
	t.Helper()
	got, err := backend.FilterGooglePhotosFiles(paths, opts)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("selected files = %q, want %q", got, want)
	}
}
