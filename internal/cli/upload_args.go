package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"app/backend"
)

func errorsAs(err error, target *exitError) bool { return errors.As(err, target) }

func newUploadCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <path> [<path> ...]",
		Short: "Upload files or directories",
		Long: "Upload files or directories to Google Photos.\n\n" +
			"The config file only supplies credentials. Everything else, including the\n" +
			"proxy and quality settings, is controlled by flags; GUI preferences do not apply.",
		Args: cobra.MinimumNArgs(1),
		RunE: runUploadCommand,
	}
	f := cmd.Flags()
	f.SortFlags = false
	f.String("account", "", "account email to upload with (default: the selected account)")
	f.String("proxy", "", "proxy URL for all requests")
	f.Bool("use-quota", false, "count uploads against the account's storage quota")
	f.Bool("saver", false, "upload in storage saver quality")
	f.BoolP("recursive", "r", false, "include subdirectories")
	f.IntP("threads", "t", backend.DefaultPreferences.UploadThreads, "number of upload threads")
	f.BoolP("force", "f", false, "upload even if the file already exists in the library")
	f.BoolP("delete", "d", false, "delete from host after upload")
	f.Bool("disable-filter", false, "disable file type filtering")
	f.Bool("date-from-filename", false, "set media date from filename (e.g. 20240709_182027.jpg)")
	f.StringP("exclude", "e", "", "exclude directories whose name matches pattern (e.g. @eaDir)")
	f.StringP("album", "a", "", "add uploads to album (creates if needed); 'AUTO' creates albums from folder names")
	f.Bool("pair-live-photos", false, "pair Apple Live Photo files; incomplete pairs are skipped")
	f.Bool("upload-incomplete-live-photos", false, "upload an unmatched Live Photo member as a single file")
	f.Bool("update-existing-photos-to-live", false, "attach matching MOV files to existing photos")
	f.Bool("ignore-apple-metadata", false, "match Live Photo pairs by filename stem instead of Apple metadata")
	f.StringP("log-level", "l", "info", "log level: debug, info, warn, error")
	f.Bool("no-tui", false, "disable the interactive progress UI")

	// Accepted for compatibility; skipping incomplete pairs is already the default.
	f.Bool("skip-incomplete-live-photos", false, "")
	_ = f.MarkHidden("skip-incomplete-live-photos")
	cmd.MarkFlagsMutuallyExclusive("skip-incomplete-live-photos", "upload-incomplete-live-photos")
	return cmd
}

// uploadRunSettings holds CLI-only settings that are not part of UploadOptions.
type uploadRunSettings struct {
	logLevel string
	noTUI    bool
}

func runUploadCommand(cmd *cobra.Command, paths []string) error {
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("invalid upload path %q: %w", path, err)
		}
	}
	f := cmd.Flags()
	if threads, _ := f.GetInt("threads"); threads < 1 {
		return fmt.Errorf("threads must be a positive integer, got %d", threads)
	}

	// Like rclone, the config file holds only the account; every upload option
	// is per-invocation so a scripted run behaves the same regardless of GUI state.
	configPath, _ := f.GetString("config")
	if err := backend.LoadConfig(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	opts := uploadOptionsFromFlags(f)
	if err := validateUploadOptions(f, opts); err != nil {
		return err
	}

	logLevel, _ := f.GetString("log-level")
	noTUI, _ := f.GetBool("no-tui")
	return runUpload(paths, opts, uploadRunSettings{logLevel: logLevel, noTUI: noTUI})
}

// uploadOptionsFromFlags builds run options from defaults plus the parsed flags.
func uploadOptionsFromFlags(f *pflag.FlagSet) backend.UploadOptions {
	opts := backend.DefaultPreferences.UploadOptions()
	getBool := func(name string) bool { v, _ := f.GetBool(name); return v }

	opts.Api.Account, _ = f.GetString("account")
	opts.Api.Proxy, _ = f.GetString("proxy")
	opts.Api.UseQuota = getBool("use-quota")
	opts.Api.Saver = getBool("saver")

	opts.Recursive = getBool("recursive")
	opts.ForceUpload = getBool("force")
	opts.DeleteFromHost = getBool("delete")
	opts.DisableUnsupportedFilesFilter = getBool("disable-filter")
	opts.SetDateFromFilename = getBool("date-from-filename")
	opts.PairLivePhotos = getBool("pair-live-photos")
	opts.SkipIncompleteLivePhotos = !getBool("upload-incomplete-live-photos")
	opts.UpdateExistingPhotosToLive = getBool("update-existing-photos-to-live")
	opts.IgnoreAppleMetadata = getBool("ignore-apple-metadata")
	opts.Threads, _ = f.GetInt("threads")
	opts.ExcludePattern, _ = f.GetString("exclude")

	album, _ := f.GetString("album")
	if strings.EqualFold(album, "AUTO") {
		opts.AlbumAutoMode = true
		opts.AlbumName = ""
	} else {
		opts.AlbumAutoMode = false
		opts.AlbumName = album
	}
	return opts
}

// validateUploadOptions rejects flags that only make sense with Live Photo
// pairing enabled.
func validateUploadOptions(f *pflag.FlagSet, opts backend.UploadOptions) error {
	if opts.PairLivePhotos {
		return nil
	}
	for _, name := range []string{
		"update-existing-photos-to-live",
		"ignore-apple-metadata",
		"skip-incomplete-live-photos",
		"upload-incomplete-live-photos",
	} {
		if f.Changed(name) {
			return fmt.Errorf("--%s requires --pair-live-photos", name)
		}
	}
	return nil
}
