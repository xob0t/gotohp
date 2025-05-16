package backend

import (
	"embed"
	"encoding/json"
)

type VersionInfo struct {
	Fixed struct {
		FileVersion string `json:"file_version"`
	} `json:"fixed"`
}

func GetVersion(content embed.FS) string {
	// Read the embedded file
	data, err := content.ReadFile("build/windows/info.json")
	if err != nil {
		return ""
	}

	// Parse the JSON
	var versionInfo VersionInfo
	err = json.Unmarshal(data, &versionInfo)
	if err != nil {
		return ""
	}
	return versionInfo.Fixed.FileVersion
}
