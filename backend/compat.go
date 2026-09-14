package backend

import (
	"app/core"
`t"embed"
`t"net/url"

type UploadBatchStart = core.UploadBatchStart
type PreflightWarning = core.PreflightWarning
type ThreadStatus = core.ThreadStatus
type FileUploadResult = core.FileUploadResult
type AlbumStatus = core.AlbumStatus
type AlbumError = core.AlbumError
type UploadReporter = core.UploadReporter
type FilesDroppedEvent = core.FilesDroppedEvent
type StartUploadEvent = core.StartUploadEvent
type Api = core.Api
type ApiOptions = core.ApiOptions
type AuthResponse = core.AuthResponse
type Config = core.Config
type Preferences = core.Preferences
type ConfigManager = core.ConfigManager
type UploadOptions = core.UploadOptions
type AlbumManager = core.AlbumManager
type UploadWorkItem = core.UploadWorkItem
type LivePhotoPair = core.LivePhotoPair

var AppConfig = core.AppConfig
var ConfigPath = core.ConfigPath
var DefaultPreferences = core.DefaultPreferences

func NewApi(o core.ApiOptions) (*core.Api, error) { return core.NewApi(o) }
func LoadConfig(p string) error                   { return core.LoadConfig(p) }
func ParseAuthString(s string) (url.Values, error)    { return core.ParseAuthString(s) }
func LooksLikeAuthString(s string) bool           { return core.LooksLikeAuthString(s) }
func FilterGooglePhotosFiles(p []string, o core.UploadOptions) ([]string, error) {
	return core.FilterGooglePhotosFiles(p, o)
}
func GetVersion(f embed.FS) string { return core.GetVersion(f) }


