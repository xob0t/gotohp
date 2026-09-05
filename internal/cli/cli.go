// Package cli implements the gotohp command-line interface. It is a thin
// consumer of the backend package: it resolves options, runs the upload, and
// renders progress in the terminal.
package cli

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Info describes the binary hosting the CLI.
type Info struct {
	// ExecutableName is used in usage text.
	ExecutableName string
	// HasGUI reports whether running without a command opens the GUI.
	HasGUI  bool
	Version string
}

// IsCLIInvocation reports whether args (excluding the program name) address
// the CLI: a recognised command, optionally preceded by root flags such as
// --config. Anything else is left for the GUI.
func IsCLIInvocation(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-c":
			i++ // skip the flag value
		case strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "-c="):
		case arg == "--help" || arg == "-h" || arg == "--version" || arg == "-v":
			return true
		case strings.HasPrefix(arg, "-"):
			return false
		default:
			return slices.Contains([]string{"upload", "credentials", "creds", "help", "version"}, arg)
		}
	}
	return false
}

// Run executes the CLI with args (excluding the program name) and returns the
// process exit code.
func Run(args []string, info Info) int {
	root := newRootCommand(info)
	root.SetArgs(normalizeLegacyArgs(root, args))
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
func normalizeLegacyArgs(root *cobra.Command, args []string) []string {
	out := slices.Clone(args)
	cmd, _, err := root.Find(args)
	if err != nil {
		return out // Let Cobra report invalid commands.
	}
	flags := cmd.Flags()
	flags.AddFlagSet(cmd.PersistentFlags())
	flags.AddFlagSet(cmd.InheritedFlags())
	for i := 0; i < len(out); i++ {
		arg := out[i]
		if arg == "--" {
			break
		}
		if arg == "-df" && flags.Lookup("disable-filter") != nil {
			// The explicit value also lets Cobra find a later subcommand.
			out[i] = "--disable-filter=true"
			continue
		}
		if flagConsumesNextArg(flags, arg) {
			i++
		}
	}
	return out
}

// Follow pflag's value rules using the command's flag definitions, including
// short bundles such as -ra value and attached values such as -ra=value.
func flagConsumesNextArg(flags *pflag.FlagSet, arg string) bool {
	if strings.HasPrefix(arg, "--") {
		name, _, attached := strings.Cut(arg[2:], "=")
		flag := flags.Lookup(name)
		return !attached && flag != nil && flag.NoOptDefVal == ""
	}
	if !strings.HasPrefix(arg, "-") {
		return false
	}
	for shorts := arg[1:]; len(shorts) > 0; shorts = shorts[1:] {
		flag := flags.ShorthandLookup(shorts[:1])
		if flag == nil || (len(shorts) > 1 && shorts[1] == '=') {
			return false
		}
		if flag.NoOptDefVal == "" {
			return len(shorts) == 1
		}
	}
	return false
}
