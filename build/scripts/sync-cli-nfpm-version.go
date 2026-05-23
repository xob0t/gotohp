package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
)

type versionInfo struct {
	Fixed struct {
		FileVersion string `json:"file_version"`
	} `json:"fixed"`
}

func main() {
	info, err := os.ReadFile("windows/info.json")
	if err != nil {
		exitf("read Windows version info: %v", err)
	}

	version, err := appVersion(info)
	if err != nil {
		exitf("%v", err)
	}

	path := "linux/nfpm/nfpm-cli.yaml"
	contents, err := os.ReadFile(path)
	if err != nil {
		exitf("read CLI nfpm config: %v", err)
	}

	next, err := replaceVersion(contents, version)
	if err != nil {
		exitf("%v", err)
	}

	if err := os.WriteFile(path, next, 0o644); err != nil {
		exitf("write CLI nfpm config: %v", err)
	}
}

func appVersion(info []byte) (string, error) {
	var versionInfo versionInfo
	if err := json.Unmarshal(info, &versionInfo); err != nil {
		return "", fmt.Errorf("parse Windows version info: %v", err)
	}
	if versionInfo.Fixed.FileVersion == "" {
		return "", fmt.Errorf("could not find fixed.file_version in build/windows/info.json")
	}
	return versionInfo.Fixed.FileVersion, nil
}

func replaceVersion(contents []byte, version string) ([]byte, error) {
	re := regexp.MustCompile(`(?m)^version:\s*"[^"]*"`)
	if !re.Match(contents) {
		return nil, fmt.Errorf("could not find top-level version in build/linux/nfpm/nfpm-cli.yaml")
	}
	return re.ReplaceAll(contents, []byte(fmt.Sprintf(`version: "%s"`, version))), nil
}

func exitf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
