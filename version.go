package main

import (
	"embed"
	"encoding/json"
)

//go:embed build/windows/info.json
var content embed.FS

type VersionInfo struct {
	Fixed struct {
		FileVersion string `json:"file_version"`
	} `json:"fixed"`
}

func GetVersion() string {
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
