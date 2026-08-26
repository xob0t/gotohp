package backend

// UploadReporter receives progress from an upload run. Each frontend renders
// these however it likes: the GUI forwards them as Wails events, the CLI feeds
// them to its terminal UI.
type UploadReporter interface {
	UploadStart(UploadBatchStart)
	UploadStop()
	// TotalBytes reports the full byte count once preflight has sized the batch.
	TotalBytes(int64)
	// TotalBytesDelta adjusts the total, e.g. when a Live Photo component is skipped.
	TotalBytesDelta(int64)
	Warning(PreflightWarning)
	ThreadStatus(ThreadStatus)
	FileResult(FileUploadResult)
	AlbumProgress(AlbumStatus)
	AlbumComplete(AlbumStatus)
	AlbumError(AlbumError)
}

// NopReporter discards every event.
type NopReporter struct{}

func (NopReporter) UploadStart(UploadBatchStart) {}
func (NopReporter) UploadStop()                  {}
func (NopReporter) TotalBytes(int64)             {}
func (NopReporter) TotalBytesDelta(int64)        {}
func (NopReporter) Warning(PreflightWarning)     {}
func (NopReporter) ThreadStatus(ThreadStatus)    {}
func (NopReporter) FileResult(FileUploadResult)  {}
func (NopReporter) AlbumProgress(AlbumStatus)    {}
func (NopReporter) AlbumComplete(AlbumStatus)    {}
func (NopReporter) AlbumError(AlbumError)        {}
