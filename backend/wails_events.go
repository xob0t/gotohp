//go:build !cli

package backend

import "github.com/wailsapp/wails/v3/pkg/application"

func init() {
	application.RegisterEvent[AlbumStatus]("albumProgress")
	application.RegisterEvent[AlbumStatus]("albumComplete")
	application.RegisterEvent[AlbumError]("albumError")
	application.RegisterEvent[UploadBatchStart]("uploadStart")
	application.RegisterEvent[application.Void]("uploadStop")
	application.RegisterEvent[FileUploadResult]("FileStatus")
	application.RegisterEvent[ThreadStatus]("ThreadStatus")
	application.RegisterEvent[application.Void]("uploadCancel")
	application.RegisterEvent[int64]("uploadTotalBytes")
	application.RegisterEvent[FilesDroppedEvent]("files-dropped")
	application.RegisterEvent[StartUploadEvent]("startUpload")
}
