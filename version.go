package main

import (
	"embed"

	"app/core"
)

//go:embed build/windows/info.json
var versionInfo embed.FS

// getAppVersion returns the version from the embedded info.json.
func getAppVersion() string {
	return core.GetVersion(versionInfo)
}
