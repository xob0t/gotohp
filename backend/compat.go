package backend

import (
	"app/core"
	"embed"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

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
type AccountConfig = core.AccountConfig
type AccountSummary = core.AccountSummary
type AccountsState = core.AccountsState
type ConfigManager = core.ConfigManager
type UploadOptions = core.UploadOptions
type UploadManager = core.UploadManager
type AlbumManager = core.AlbumManager
type UploadWorkItem = core.UploadWorkItem
type UploadWorkKind = core.UploadWorkKind
type SingleMedia = core.SingleMedia
type LivePhotoPair = core.LivePhotoPair
type LivePhotoCreateRequest = core.LivePhotoCreateRequest
type LivePhotoReconcileRequest = core.LivePhotoReconcileRequest
type UploadDeviceInfo = core.UploadDeviceInfo
type LivePhotoCommitPolicy = core.LivePhotoCommitPolicy
type LivePhotoUploadOptions = core.LivePhotoUploadOptions
type LivePhotoMetadata = core.LivePhotoMetadata
type UploadProgressCallback = core.UploadProgressCallback
type ProgressReader = core.ProgressReader
type RetryConfig = core.RetryConfig
type ScottyFinalizeToken = core.ScottyFinalizeToken

// Deprecated: use core.AppConfig. This snapshot is retained for source compatibility.\nvar AppConfig = core.AppConfig
// Deprecated: use core.ConfigPath. This snapshot is retained for source compatibility.\nvar ConfigPath = core.ConfigPath
// Deprecated: use core.DefaultPreferences. This snapshot is retained for source compatibility.\nvar DefaultPreferences = core.DefaultPreferences

func NewApi(o core.ApiOptions) (*core.Api, error) { return core.NewApi(o) }
func NewUploadManager(r core.UploadReporter, l *slog.Logger) *core.UploadManager {
	return core.NewUploadManager(r, l)
}
func NewAlbumManager(a *core.Api, r core.UploadReporter, l *slog.Logger, c <-chan struct{}) *core.AlbumManager {
	return core.NewAlbumManager(a, r, l, c)
}
func NewHTTPClientWithProxy(p string) (*http.Client, error) { return core.NewHTTPClientWithProxy(p) }
func NewProgressReader(r io.Reader, t int64, p func(int64, int64)) *core.ProgressReader {
	return core.NewProgressReader(r, t, p)
}
func LoadConfig(p string) error                    { return core.LoadConfig(p) }
func ParseAuthString(s string) (url.Values, error) { return core.ParseAuthString(s) }
func LooksLikeAuthString(s string) bool            { return core.LooksLikeAuthString(s) }
func FilterGooglePhotosFiles(p []string, o core.UploadOptions) ([]string, error) {
	return core.FilterGooglePhotosFiles(p, o)
}
func GetVersion(f embed.FS) string { return core.GetVersion(f) }

// CurrentConfig returns the canonical core configuration.
func CurrentConfig() core.Config { return core.AppConfig }
