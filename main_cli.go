//go:build cli

package main

import (
	"os"

	"app/internal/cli"
)

func main() {
	// CLI-only build without Wails/WebView: there is no GUI to fall back to.
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	os.Exit(cli.Run(os.Args[1:], cli.Info{
		ExecutableName: "gotohp-cli",
		HasGUI:         false,
		Version:        getAppVersion(),
	}))
}
