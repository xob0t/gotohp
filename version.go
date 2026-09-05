package main

import (
	"embed"

	"app/backend"
)

//go:embed build/windows/info.json
var versionInfo embed.FS

// getAppVersion returns the version from the embedded info.json.
func getAppVersion() string {
	return backend.GetVersion(versionInfo)
}
