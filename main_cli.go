//go:build cli

package main

import (
	"os"

	"app/internal/cli"
)

// gotohp-cli is built without Wails, so it is CLI only.
func main() {
	os.Exit(cli.Run(os.Args[1:], cli.Info{
		ExecutableName: "gotohp-cli",
		HasGUI:         false,
		Version:        getAppVersion(),
	}))
}
