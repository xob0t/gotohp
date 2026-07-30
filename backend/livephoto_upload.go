package backend

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
)

type livePhotoUploadAPI interface {
	FindRemoteMediaByHash(hash []byte) (string, error)
	GetUploadToken(sha1Hash string, fileSize int64) (string, error)
	UploadFileWithProgress(ctx context.Context, filePath string, uploadToken string, onProgress UploadProgressCallback) (ScottyFinalizeToken, error)
	CommitLivePhoto(input LivePhotoCreateRequest) (string, error)
	ReconcileLivePhoto(input LivePhotoReconcileRequest) (string, error)
}

type LivePhotoUploadOptions struct {
	Policy                     LivePhotoCommitPolicy
	DeleteFromHost             bool
	SetDateFromFilename        bool
	UpdateExistingPhotosToLive bool
}

func uploadLivePhotoWithCallback(
	ctx context.Context,
	api livePhotoUploadAPI,
	pair LivePhotoPair,
	options LivePhotoUploadOptions,
	workerID int,
	callback ProgressCallback,
) (mediaKey string, skipped bool, err error) {
	photoInfo, err := os.Stat(pair.PhotoPath)
	if err != nil {
		return "", false, fmt.Errorf("stat Live Photo still: %w", err)
	}
	videoInfo, err := os.Stat(pair.VideoPath)
	if err != nil {
		return "", false, fmt.Errorf("stat Live Photo video: %w", err)
	}
	totalBytes := photoInfo.Size() + videoInfo.Size()
	displayName := filepath.Base(pair.PhotoPath) + " + " + filepath.Base(pair.VideoPath)
	emitLivePhotoStatus(callback, ThreadStatus{
		WorkerID: workerID,
		Status:   "hashing",
		FilePath: pair.PhotoPath,
		FileName: displayName,
		Message:  "Hashing Live Photo pair...",
	})

	photoSHA1, err := CalculateSHA1(ctx, pair.PhotoPath)
	if err != nil {
		return "", false, fmt.Errorf("hash Live Photo still: %w", err)
	}
	videoSHA1, err := CalculateSHA1(ctx, pair.VideoPath)
	if err != nil {
		return "", false, fmt.Errorf("hash Live Photo video: %w", err)
	}

	emitLivePhotoStatus(callback, ThreadStatus{
		WorkerID: workerID,
		Status:   "checking",
		FilePath: pair.PhotoPath,
		FileName: displayName,
		Message:  "Checking both Live Photo components...",
	})
	photoRemoteKey, photoCheckErr := api.FindRemoteMediaByHash(photoSHA1)
	videoRemoteKey, videoCheckErr := api.FindRemoteMediaByHash(videoSHA1)
	if photoCheckErr != nil {
		return "", false, fmt.Errorf("check Live Photo still deduplication: %w", photoCheckErr)
	}
	if videoCheckErr != nil {
		return "", false, fmt.Errorf("check Live Photo video deduplication: %w", videoCheckErr)
	}
	if photoRemoteKey != "" {
		if options.UpdateExistingPhotosToLive {
			mediaKey, err := reconcileExistingLivePhoto(
				ctx,
				api,
				pair,
				videoInfo,
				photoSHA1,
				videoSHA1,
				options,
				workerID,
				displayName,
				callback,
			)
			return mediaKey, false, err
		}
		emitLivePhotoStatus(callback, ThreadStatus{
			WorkerID: workerID,
			Status:   "skipped",
			FilePath: pair.PhotoPath,
			FileName: displayName,
			Message:  "Skipped: a Live Photo component already exists remotely",
		})
		return "", true, nil
	}
	// The VideoOriginal direction has not been recovered. A standalone remote MOV
	// must not be combined with a newly uploaded still using the Phodeo request.
	if videoRemoteKey != "" {
		emitLivePhotoStatus(callback, ThreadStatus{
			WorkerID: workerID,
			Status:   "skipped",
			FilePath: pair.PhotoPath,
			FileName: displayName,
			Message:  "Skipped: the Live Photo video already exists remotely",
		})
		return "", true, nil
	}

	photoToken, err := uploadLivePhotoComponent(ctx, api, pair.PhotoPath, pair.PhotoPath, photoInfo.Size(), photoSHA1, 0, totalBytes, workerID, displayName, callback)
	if err != nil {
		return "", false, fmt.Errorf("upload Live Photo still: %w", err)
	}
	videoToken, err := uploadLivePhotoComponent(ctx, api, pair.VideoPath, pair.PhotoPath, videoInfo.Size(), videoSHA1, photoInfo.Size(), totalBytes, workerID, displayName, callback)
	if err != nil {
		return "", false, fmt.Errorf("upload Live Photo video: %w", err)
	}

	uploadTime := photoInfo.ModTime()
	if options.SetDateFromFilename {
		if parsed, ok := parseTimestampFromFilename(pair.PhotoPath); ok {
			uploadTime = parsed
		}
	}
	emitLivePhotoStatus(callback, ThreadStatus{
		WorkerID: workerID,
		Status:   "finalizing",
		FilePath: pair.PhotoPath,
		FileName: displayName,
		Message:  "Committing linked Live Photo...",
	})
	mediaKey, err = api.CommitLivePhoto(LivePhotoCreateRequest{
		PhotoToken:       photoToken,
		VideoToken:       videoToken,
		FileName:         photoInfo.Name(),
		PhotoSHA1:        photoSHA1,
		VideoSHA1:        videoSHA1,
		CreatedAt:        uploadTime,
		ModifiedAt:       uploadTime,
		StoragePolicy:    options.Policy.StoragePolicy,
		UploadQuality:    options.Policy.UploadQuality,
		UploadDeviceInfo: options.Policy.UploadDeviceInfo,
	})
	if err != nil {
		return "", false, fmt.Errorf("commit Live Photo: %w", err)
	}
	if mediaKey == "" {
		return "", false, fmt.Errorf("Live Photo media key not received")
	}

	if options.DeleteFromHost {
		if err := removeLivePhotoFiles(pair); err != nil {
			return mediaKey, false, err
		}
	}
	return mediaKey, false, nil
}

func reconcileExistingLivePhoto(
	ctx context.Context,
	api livePhotoUploadAPI,
	pair LivePhotoPair,
	videoInfo os.FileInfo,
	photoSHA1 []byte,
	videoSHA1 []byte,
	options LivePhotoUploadOptions,
	workerID int,
	displayName string,
	callback ProgressCallback,
) (string, error) {
	emitLivePhotoStatus(callback, ThreadStatus{
		WorkerID: workerID,
		Status:   "uploading",
		FilePath: pair.PhotoPath,
		FileName: displayName,
		Message:  "Uploading Live Photo video for existing photo...",
	})
	videoToken, err := uploadLivePhotoComponent(
		ctx,
		api,
		pair.VideoPath,
		pair.PhotoPath,
		videoInfo.Size(),
		videoSHA1,
		0,
		videoInfo.Size(),
		workerID,
		displayName,
		callback,
	)
	if err != nil {
		return "", fmt.Errorf("upload Live Photo video for existing photo: %w", err)
	}

	emitLivePhotoStatus(callback, ThreadStatus{
		WorkerID: workerID,
		Status:   "finalizing",
		FilePath: pair.PhotoPath,
		FileName: displayName,
		Message:  "Updating existing photo to Live...",
	})
	mediaKey, err := api.ReconcileLivePhoto(LivePhotoReconcileRequest{
		VideoToken:       videoToken,
		FileName:         videoInfo.Name(),
		PhotoSHA1:        photoSHA1,
		VideoSHA1:        videoSHA1,
		CreatedAt:        videoInfo.ModTime(),
		ModifiedAt:       videoInfo.ModTime(),
		StoragePolicy:    options.Policy.StoragePolicy,
		UploadQuality:    options.Policy.UploadQuality,
		UploadDeviceInfo: options.Policy.UploadDeviceInfo,
	})
	if err != nil {
		return "", fmt.Errorf("update existing photo to Live: %w", err)
	}
	if mediaKey == "" {
		return "", fmt.Errorf("updated Live Photo media key not received")
	}
	if options.DeleteFromHost {
		if err := removeLivePhotoFiles(pair); err != nil {
			return mediaKey, err
		}
	}
	return mediaKey, nil
}

func uploadLivePhotoComponent(
	ctx context.Context,
	api livePhotoUploadAPI,
	componentPath string,
	progressPath string,
	size int64,
	hash []byte,
	completedBytes int64,
	totalBytes int64,
	workerID int,
	displayName string,
	callback ProgressCallback,
) (ScottyFinalizeToken, error) {
	uploadSession, err := api.GetUploadToken(base64.StdEncoding.EncodeToString(hash), size)
	if err != nil {
		return ScottyFinalizeToken{}, err
	}
	progress := func(bytesUploaded, _ int64, attempt int) {
		message := "Uploading Live Photo pair..."
		if attempt > 1 {
			message = fmt.Sprintf("Retrying Live Photo component (attempt %d)...", attempt)
		}
		emitLivePhotoStatus(callback, ThreadStatus{
			WorkerID:      workerID,
			Status:        "uploading",
			FilePath:      progressPath,
			FileName:      displayName,
			Message:       message,
			BytesUploaded: completedBytes + bytesUploaded,
			BytesTotal:    totalBytes,
			Attempt:       attempt,
		})
	}
	return api.UploadFileWithProgress(ctx, componentPath, uploadSession, progress)
}

func emitLivePhotoStatus(callback ProgressCallback, status ThreadStatus) {
	if callback != nil {
		callback("ThreadStatus", status)
	}
}

func removeLivePhotoFiles(pair LivePhotoPair) error {
	for _, path := range []string{pair.VideoPath, pair.PhotoPath} {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("Live Photo uploaded but failed to delete %s: %w", filepath.Base(path), err)
		}
	}
	return nil
}
