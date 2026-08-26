// Package cli implements the gotohp command-line interface. It is a thin
// consumer of the backend package: it resolves options, runs the upload, and
// renders progress in the terminal.
package cli

import (
	"fmt"
	"os"
	"slices"

	"github.com/spf13/cobra"
)

// Info describes the binary hosting the CLI.
type Info struct {
	// ExecutableName is used in usage text.
	ExecutableName string
	// HasGUI reports whether running without a command opens the GUI.
	HasGUI  bool
	Version string
}

// IsCommand reports whether arg is a recognised top-level CLI command.
func IsCommand(arg string) bool {
	return slices.Contains([]string{
		"upload",
		"credentials", "creds",
		"help", "--help", "-h",
		"version", "--version", "-v",
	}, arg)
}

// Run executes the CLI with args (excluding the program name) and returns the
// process exit code.
func Run(args []string, info Info) int {
	root := newRootCommand(info)
	root.SetArgs(normalizeLegacyArgs(args))
	if err := root.Execute(); err != nil {
		var exit exitError
		if errorsAs(err, &exit) {
			return exit.code
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		return 1
	}
	return 0
}

// exitError carries an exit code for failures that already printed their message.
type exitError struct{ code int }

func (e exitError) Error() string { return fmt.Sprintf("exit %d", e.code) }

func newRootCommand(info Info) *cobra.Command {
	long := "gotohp - Google Photos unofficial client"
	if info.HasGUI {
		long += fmt.Sprintf("\n\nRun %s without a command to launch the GUI.", info.ExecutableName)
	}
	root := &cobra.Command{
		Use:           info.ExecutableName,
		Short:         "Google Photos unofficial client",
		Long:          long,
		Version:       info.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("gotohp v{{.Version}}\n")
	root.PersistentFlags().StringP("config", "c", "", "path to config file")
	root.AddCommand(newUploadCommand(), newCredentialsCommand(), &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Args:  cobra.NoArgs,
		Run: func(*cobra.Command, []string) {
			fmt.Printf("gotohp v%s\n", info.Version)
		},
	})
	return root
}

// normalizeLegacyArgs rewrites flags from the previous hand-rolled parser that
// POSIX-style parsing would misread. "-df" used to mean --disable-filter; under
// pflag it would expand to -d -f (delete + force), which must not happen silently.
func normalizeLegacyArgs(args []string) []string {
	out := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "-df" {
			arg = "--disable-filter"
		}
		out = append(out, arg)
	}
	return out
}
