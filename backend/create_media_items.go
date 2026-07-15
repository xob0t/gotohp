package backend

import (
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"google.golang.org/protobuf/encoding/protowire"
)

// This opaque result-item mask occupied top-level field 5 and was identical in
// all 17 accepted official-client requests across the scoped captures.
const livePhotoResultItemMaskBase64 = "CnsKAggBEAEaAggBIgIIASoQCgIIARICCAEaAggBIgIIATICCAGCAQIIAYoBAggBmgECCAGiAQIIAaoBCggBKgIgATICCAHKAQIIAfIBBggBEgIIAfoBAggBggICCAGKAgQKAggBkgIGCAESAggBogICCAGqAgC6AgDCAgAQASpiCAESKggBEhIIARoGCAESAggBIgQIASIAKAEiBggBEgIQASoGCAESAggBMAE4ARoWCAESDAgBGgIIASICCAEoARoCCAEwASIKCAESBggBEgIIASoOCgQIATABEgYIARICCAE6AggBQgIIAUoKCAESAggBGgISAFoSEgIIARoCCAEiCAgBEgQIARACYgIIAXISEgIIARoCCAEiCAgBEgQIARACeggKAggBIgIIAYoBBAoCCAGSAQQIARABqgECCgA="

type UploadDeviceInfo struct {
	Model             string
	Make              string
	AndroidAPIVersion int64
}

type LivePhotoCreateRequest struct {
	PhotoToken       ScottyFinalizeToken
	VideoToken       ScottyFinalizeToken
	FileName         string
	PhotoSHA1        []byte
	VideoSHA1        []byte
	CreatedAt        time.Time
	ModifiedAt       time.Time
	StoragePolicy    int64
	UploadQuality    int64
	UploadDeviceInfo UploadDeviceInfo
}

type LivePhotoCommitPolicy struct {
	StoragePolicy    int64
	UploadQuality    int64
	UploadDeviceInfo UploadDeviceInfo
}

func buildLivePhotoCommitPolicy(api *Api, config Config) LivePhotoCommitPolicy {
	storagePolicy := int64(3)
	model := api.model
	if config.Saver {
		storagePolicy = 1
		model = "Pixel 2"
	}
	// Use Quota selects the current quota-counting device regardless of quality.
	if config.UseQuota {
		model = "Pixel 8"
	}
	return LivePhotoCommitPolicy{
		StoragePolicy: storagePolicy,
		UploadQuality: 1,
		UploadDeviceInfo: UploadDeviceInfo{
			Model:             model,
			Make:              api.make,
			AndroidAPIVersion: api.androidAPIVersion,
		},
	}
}

func BuildLivePhotoCreateMediaItemsRequest(input LivePhotoCreateRequest) ([]byte, error) {
	if _, err := ParseScottyFinalizeToken(input.PhotoToken.Raw); err != nil {
		return nil, fmt.Errorf("invalid photo finalize token: %w", err)
	}
	if _, err := ParseScottyFinalizeToken(input.VideoToken.Raw); err != nil {
		return nil, fmt.Errorf("invalid video finalize token: %w", err)
	}
	if strings.TrimSpace(input.FileName) == "" {
		return nil, fmt.Errorf("photo filename is required")
	}
	if len(input.PhotoSHA1) != sha1.Size || len(input.VideoSHA1) != sha1.Size {
		return nil, fmt.Errorf("photo and video SHA-1 values must each contain %d bytes", sha1.Size)
	}
	if input.CreatedAt.IsZero() || input.ModifiedAt.IsZero() {
		return nil, fmt.Errorf("photo creation and modification times are required")
	}
	if input.StoragePolicy <= 0 || input.UploadQuality <= 0 {
		return nil, fmt.Errorf("storage policy and upload quality must be positive")
	}
	if input.UploadDeviceInfo.Model == "" || input.UploadDeviceInfo.Make == "" || input.UploadDeviceInfo.AndroidAPIVersion <= 0 {
		return nil, fmt.Errorf("complete Android upload device information is required")
	}

	resultItemMask, err := base64.StdEncoding.DecodeString(livePhotoResultItemMaskBase64)
	if err != nil {
		return nil, fmt.Errorf("decode result item mask: %w", err)
	}

	// Emit the official-client invariant shape directly so Scotty tokens remain
	// byte-identical. Blueprint fields are uploadToken=1, fileName=2,
	// sourceSha1=3, createTime=5, modTime=6, storagePolicy=7,
	// uploadQuality=10, and livePhotoInfo=24. LivePhotoInfo contains
	// videoUploadToken=1 and videoSourceSha1=2. Client capabilities (top-level 3)
	// and videoCrc32C (24.3) are schema fields but were absent from every accepted
	// scoped request; passive captures do not prove the server's absolute minimum.

	liveInfo := appendBytesWireField(nil, 1, input.VideoToken.Raw)
	liveInfo = appendBytesWireField(liveInfo, 2, input.VideoSHA1)

	blueprint := appendBytesWireField(nil, 1, input.PhotoToken.Raw)
	blueprint = appendBytesWireField(blueprint, 2, []byte(input.FileName))
	blueprint = appendBytesWireField(blueprint, 3, input.PhotoSHA1)
	blueprint = appendBytesWireField(blueprint, 5, marshalUploadTimestamp(input.CreatedAt))
	blueprint = appendBytesWireField(blueprint, 6, marshalUploadTimestamp(input.ModifiedAt))
	blueprint = appendVarintWireField(blueprint, 7, uint64(input.StoragePolicy))
	blueprint = appendVarintWireField(blueprint, 10, uint64(input.UploadQuality))
	blueprint = appendBytesWireField(blueprint, 24, liveInfo)

	device := appendBytesWireField(nil, 3, []byte(input.UploadDeviceInfo.Model))
	device = appendBytesWireField(device, 4, []byte(input.UploadDeviceInfo.Make))
	device = appendVarintWireField(device, 5, uint64(input.UploadDeviceInfo.AndroidAPIVersion))

	request := appendBytesWireField(nil, 1, blueprint)
	request = appendBytesWireField(request, 2, device)
	request = appendBytesWireField(request, 5, resultItemMask)
	return request, nil
}

func marshalUploadTimestamp(value time.Time) []byte {
	timestamp := appendVarintWireField(nil, 1, uint64(value.Unix()))
	return appendVarintWireField(timestamp, 2, uint64(value.Nanosecond()))
}

func appendBytesWireField(message []byte, number protowire.Number, value []byte) []byte {
	message = protowire.AppendTag(message, number, protowire.BytesType)
	return protowire.AppendBytes(message, value)
}

func appendVarintWireField(message []byte, number protowire.Number, value uint64) []byte {
	message = protowire.AppendTag(message, number, protowire.VarintType)
	return protowire.AppendVarint(message, value)
}
