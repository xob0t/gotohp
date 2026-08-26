package backend

// UploadOptions fully describes how a single upload run behaves. The GUI
// derives them from its Preferences; the CLI builds them from flags.
type UploadOptions struct {
	// Api selects the account and upload policy used for every request.
	Api ApiOptions

	Recursive                     bool
	ExcludePattern                string
	Threads                       int
	ForceUpload                   bool
	DeleteFromHost                bool
	DisableUnsupportedFilesFilter bool
	SetDateFromFilename           bool

	PairLivePhotos             bool
	SkipIncompleteLivePhotos   bool
	UpdateExistingPhotosToLive bool
	// IgnoreAppleMetadata matches Live Photo pairs by filename stem instead of
	// Apple content identifiers. It is never persisted.
	IgnoreAppleMetadata bool

	// AlbumName is a name or album key to add uploads to. Ignored when
	// AlbumAutoMode is set, which creates one album per source directory.
	AlbumName     string
	AlbumAutoMode bool
}

// UploadOptions derives run options from GUI preferences.
func (c Preferences) UploadOptions() UploadOptions {
	return UploadOptions{
		Api: ApiOptions{
			Proxy:    c.Proxy,
			Saver:    c.Saver,
			UseQuota: c.UseQuota,
		},
		Recursive:                     c.Recursive,
		ExcludePattern:                c.ExcludePattern,
		Threads:                       c.UploadThreads,
		ForceUpload:                   c.ForceUpload,
		DeleteFromHost:                c.DeleteFromHost,
		DisableUnsupportedFilesFilter: c.DisableUnsupportedFilesFilter,
		SetDateFromFilename:           c.SetDateFromFilename,
		PairLivePhotos:                c.PairLivePhotos,
		SkipIncompleteLivePhotos:      c.SkipIncompleteLivePhotos,
		UpdateExistingPhotosToLive:    c.UpdateExistingPhotosToLive,
		AlbumName:                     c.AlbumName,
		AlbumAutoMode:                 c.AlbumAutoMode,
	}
}

func (o UploadOptions) normalized() UploadOptions {
	if o.Threads < 1 {
		o.Threads = 1
	}
	if o.AlbumAutoMode {
		o.AlbumName = ""
	}
	return o
}
