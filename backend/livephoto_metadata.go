package backend

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	appleMakerNoteHeader = "Apple iOS\x00"
	maxMakerNoteScanSize = 64 << 20
	maxContentIdentifier = 256
)

var ErrContentIdentifierMissing = errors.New("live photo content identifier is missing")

type LivePhotoMetadata struct {
	ContentIdentifier string
	HasStillImageTime bool
}

func ReadPhotoContentIdentifier(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open photo metadata: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("stat photo metadata: %w", err)
	}
	return readPhotoContentIdentifier(file, info.Size())
}

func ReadVideoLivePhotoMetadata(path string) (LivePhotoMetadata, error) {
	file, err := os.Open(path)
	if err != nil {
		return LivePhotoMetadata{}, fmt.Errorf("open video metadata: %w", err)
	}
	defer func() { _ = file.Close() }()

	info, err := file.Stat()
	if err != nil {
		return LivePhotoMetadata{}, fmt.Errorf("stat video metadata: %w", err)
	}
	return readVideoLivePhotoMetadata(file, info.Size())
}

func readPhotoContentIdentifier(reader io.ReaderAt, size int64) (string, error) {
	if size <= 0 {
		return "", ErrContentIdentifierMissing
	}
	scanSize := min(size, maxMakerNoteScanSize)
	data := make([]byte, scanSize)
	if _, err := reader.ReadAt(data, 0); err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read photo metadata: %w", err)
	}

	marker := []byte(appleMakerNoteHeader)
	var parseErr error
	for searchFrom := 0; searchFrom < len(data); {
		relativeOffset := bytes.Index(data[searchFrom:], marker)
		if relativeOffset < 0 {
			break
		}
		markerOffset := searchFrom + relativeOffset
		identifier, err := parseAppleMakerNoteIdentifier(data[markerOffset:])
		if err == nil {
			return identifier, nil
		}
		parseErr = err
		searchFrom = markerOffset + len(marker)
	}
	if parseErr != nil {
		return "", parseErr
	}
	return "", ErrContentIdentifierMissing
}

func parseAppleMakerNoteIdentifier(data []byte) (string, error) {
	if len(data) < 16 || string(data[:len(appleMakerNoteHeader)]) != appleMakerNoteHeader {
		return "", fmt.Errorf("invalid Apple Maker Note header")
	}

	var order binary.ByteOrder
	switch string(data[12:14]) {
	case "MM":
		order = binary.BigEndian
	case "II":
		order = binary.LittleEndian
	default:
		return "", fmt.Errorf("invalid Apple Maker Note byte order")
	}

	entryCount := int(order.Uint16(data[14:16]))
	entriesEnd := 16 + entryCount*12
	if entryCount <= 0 || entriesEnd > len(data) {
		return "", fmt.Errorf("invalid Apple Maker Note entry table")
	}

	// Apple Maker Note tag 17 is the asset content identifier shared with the
	// paired QuickTime file; it is the pairing key rather than the filenames.
	for index := 0; index < entryCount; index++ {
		entry := data[16+index*12 : 16+(index+1)*12]
		if order.Uint16(entry[0:2]) != 17 {
			continue
		}
		if order.Uint16(entry[2:4]) != 2 {
			return "", fmt.Errorf("Apple content identifier has unexpected type")
		}
		valueLength := uint64(order.Uint32(entry[4:8]))
		if valueLength == 0 || valueLength > maxContentIdentifier {
			return "", fmt.Errorf("Apple content identifier has invalid length")
		}

		var value []byte
		if valueLength <= 4 {
			value = entry[8 : 8+valueLength]
		} else {
			valueOffset := uint64(order.Uint32(entry[8:12]))
			if valueOffset > uint64(len(data)) || valueLength > uint64(len(data))-valueOffset {
				return "", fmt.Errorf("Apple content identifier points outside Maker Note")
			}
			value = data[valueOffset : valueOffset+valueLength]
		}

		identifier := strings.Trim(string(value), "\x00 \t\r\n")
		if !isCanonicalUUID(identifier) {
			return "", fmt.Errorf("Apple content identifier is not a canonical UUID")
		}
		return identifier, nil
	}
	return "", ErrContentIdentifierMissing
}

func isCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		switch index {
		case 8, 13, 18, 23:
			if character != '-' {
				return false
			}
		default:
			if !((character >= '0' && character <= '9') ||
				(character >= 'a' && character <= 'f') ||
				(character >= 'A' && character <= 'F')) {
				return false
			}
		}
	}
	return true
}

type mp4Box struct {
	typ          [4]byte
	payloadStart int64
	end          int64
}

func readVideoLivePhotoMetadata(reader io.ReaderAt, size int64) (LivePhotoMetadata, error) {
	if size < 8 {
		return LivePhotoMetadata{}, fmt.Errorf("invalid QuickTime file size")
	}

	metadata := LivePhotoMetadata{}
	err := walkMP4Boxes(reader, 0, size, func(box mp4Box) error {
		switch string(box.typ[:]) {
		case "meta":
			identifier, found, err := readQuickTimeContentIdentifier(reader, box)
			if err != nil {
				return err
			}
			if found {
				metadata.ContentIdentifier = identifier
			}
		case "stbl":
			hasStillImageTime, err := readStillImageTimeTrack(reader, box)
			if err != nil {
				return err
			}
			metadata.HasStillImageTime = metadata.HasStillImageTime || hasStillImageTime
		}
		return nil
	})
	if err != nil {
		return LivePhotoMetadata{}, err
	}
	if metadata.ContentIdentifier == "" {
		return metadata, ErrContentIdentifierMissing
	}
	return metadata, nil
}

func walkMP4Boxes(reader io.ReaderAt, start, end int64, visit func(mp4Box) error) error {
	for offset := start; offset < end; {
		box, err := readMP4Box(reader, offset, end)
		if err != nil {
			return err
		}
		if err := visit(box); err != nil {
			return err
		}
		if isMP4Container(box.typ) {
			childStart := box.payloadStart
			if string(box.typ[:]) == "meta" {
				var err error
				childStart, err = quickTimeMetaChildrenStart(reader, box)
				if err != nil {
					return err
				}
			}
			if childStart > box.end {
				return fmt.Errorf("invalid %s container", string(box.typ[:]))
			}
			if err := walkMP4Boxes(reader, childStart, box.end, visit); err != nil {
				return err
			}
		}
		offset = box.end
	}
	return nil
}

// QuickTime permits a headerless meta atom, while ISO BMFF meta boxes begin
// with four version/flags bytes. Detect the form from the first child header.
func quickTimeMetaChildrenStart(reader io.ReaderAt, box mp4Box) (int64, error) {
	payloadSize := box.end - box.payloadStart
	if payloadSize < 8 {
		return 0, fmt.Errorf("truncated QuickTime meta box")
	}
	header := make([]byte, 8)
	if _, err := reader.ReadAt(header, box.payloadStart); err != nil {
		return 0, fmt.Errorf("read QuickTime meta box: %w", err)
	}
	firstChildSize := int64(binary.BigEndian.Uint32(header[:4]))
	if firstChildSize >= 8 && firstChildSize <= payloadSize {
		return box.payloadStart, nil
	}
	if payloadSize < 12 {
		return 0, fmt.Errorf("truncated ISO meta box")
	}
	return box.payloadStart + 4, nil
}

func readMP4Box(reader io.ReaderAt, offset, parentEnd int64) (mp4Box, error) {
	header := make([]byte, 16)
	if offset < 0 || parentEnd-offset < 8 {
		return mp4Box{}, fmt.Errorf("truncated QuickTime box header at %d", offset)
	}
	if _, err := reader.ReadAt(header[:8], offset); err != nil {
		return mp4Box{}, fmt.Errorf("read QuickTime box header at %d: %w", offset, err)
	}

	boxSize := int64(binary.BigEndian.Uint32(header[:4]))
	headerSize := int64(8)
	if boxSize == 1 {
		if parentEnd-offset < 16 {
			return mp4Box{}, fmt.Errorf("truncated extended QuickTime box header at %d", offset)
		}
		if _, err := reader.ReadAt(header[8:16], offset+8); err != nil {
			return mp4Box{}, fmt.Errorf("read extended QuickTime box header at %d: %w", offset, err)
		}
		boxSize = int64(binary.BigEndian.Uint64(header[8:16]))
		headerSize = 16
	} else if boxSize == 0 {
		boxSize = parentEnd - offset
	}
	if boxSize < headerSize || boxSize > parentEnd-offset {
		return mp4Box{}, fmt.Errorf("invalid QuickTime box size at %d", offset)
	}

	var boxType [4]byte
	copy(boxType[:], header[4:8])
	return mp4Box{typ: boxType, payloadStart: offset + headerSize, end: offset + boxSize}, nil
}

func isMP4Container(boxType [4]byte) bool {
	switch string(boxType[:]) {
	case "moov", "trak", "mdia", "minf", "stbl", "udta", "meta":
		return true
	default:
		return false
	}
}

func readQuickTimeContentIdentifier(reader io.ReaderAt, meta mp4Box) (string, bool, error) {
	childrenStart, err := quickTimeMetaChildrenStart(reader, meta)
	if err != nil {
		return "", false, err
	}
	children, err := readDirectMP4Boxes(reader, childrenStart, meta.end)
	if err != nil {
		return "", false, err
	}
	var keysBox, itemListBox *mp4Box
	for index := range children {
		switch string(children[index].typ[:]) {
		case "keys":
			keysBox = &children[index]
		case "ilst":
			itemListBox = &children[index]
		}
	}
	if keysBox == nil || itemListBox == nil {
		return "", false, nil
	}

	keys, err := readQuickTimeKeys(reader, *keysBox)
	if err != nil {
		return "", false, err
	}
	contentIndex := 0
	for index, key := range keys {
		if key == "com.apple.quicktime.content.identifier" {
			contentIndex = index + 1
			break
		}
	}
	if contentIndex == 0 {
		return "", false, nil
	}

	identifier, found, err := readQuickTimeItemValue(reader, *itemListBox, uint32(contentIndex))
	if err != nil || !found {
		return "", found, err
	}
	identifier = strings.Trim(identifier, "\x00 \t\r\n")
	if !isCanonicalUUID(identifier) {
		return "", false, fmt.Errorf("QuickTime content identifier is not a canonical UUID")
	}
	return identifier, true, nil
}

func readDirectMP4Boxes(reader io.ReaderAt, start, end int64) ([]mp4Box, error) {
	var boxes []mp4Box
	for offset := start; offset < end; {
		box, err := readMP4Box(reader, offset, end)
		if err != nil {
			return nil, err
		}
		boxes = append(boxes, box)
		offset = box.end
	}
	return boxes, nil
}

func readQuickTimeKeys(reader io.ReaderAt, box mp4Box) ([]string, error) {
	payload, err := readBoxPayload(reader, box, 4<<20)
	if err != nil {
		return nil, err
	}
	if len(payload) < 8 {
		return nil, fmt.Errorf("truncated QuickTime keys box")
	}
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	offset := 8
	keys := make([]string, 0, entryCount)
	for index := 0; index < entryCount; index++ {
		if len(payload)-offset < 8 {
			return nil, fmt.Errorf("truncated QuickTime metadata key")
		}
		entrySize := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
		if entrySize < 8 || entrySize > len(payload)-offset {
			return nil, fmt.Errorf("invalid QuickTime metadata key size")
		}
		if string(payload[offset+4:offset+8]) != "mdta" {
			keys = append(keys, "")
		} else {
			keys = append(keys, string(payload[offset+8:offset+entrySize]))
		}
		offset += entrySize
	}
	return keys, nil
}

func readQuickTimeItemValue(reader io.ReaderAt, itemList mp4Box, wantedIndex uint32) (string, bool, error) {
	items, err := readDirectMP4Boxes(reader, itemList.payloadStart, itemList.end)
	if err != nil {
		return "", false, err
	}
	for _, item := range items {
		if binary.BigEndian.Uint32(item.typ[:]) != wantedIndex {
			continue
		}
		values, err := readDirectMP4Boxes(reader, item.payloadStart, item.end)
		if err != nil {
			return "", false, err
		}
		for _, value := range values {
			if string(value.typ[:]) != "data" || value.end-value.payloadStart < 8 {
				continue
			}
			payload, err := readBoxPayload(reader, value, maxContentIdentifier+8)
			if err != nil {
				return "", false, err
			}
			return string(payload[8:]), true, nil
		}
	}
	return "", false, nil
}

func readStillImageTimeTrack(reader io.ReaderAt, sampleTable mp4Box) (bool, error) {
	children, err := readDirectMP4Boxes(reader, sampleTable.payloadStart, sampleTable.end)
	if err != nil {
		return false, err
	}
	var sampleDescription, sampleSizes, chunkOffsets *mp4Box
	for index := range children {
		switch string(children[index].typ[:]) {
		case "stsd":
			sampleDescription = &children[index]
		case "stsz":
			sampleSizes = &children[index]
		case "stco", "co64":
			chunkOffsets = &children[index]
		}
	}
	if sampleDescription == nil || sampleSizes == nil || chunkOffsets == nil {
		return false, nil
	}

	isTimedTrack, err := sampleDescriptionContainsStillImageTime(reader, *sampleDescription)
	if err != nil || !isTimedTrack {
		return false, err
	}
	sampleSize, err := readFirstSampleSize(reader, *sampleSizes)
	if err != nil {
		return false, err
	}
	sampleOffset, err := readFirstChunkOffset(reader, *chunkOffsets)
	if err != nil {
		return false, err
	}
	if sampleSize < 9 || sampleSize > 1<<20 {
		return false, nil
	}
	sample := make([]byte, sampleSize)
	if _, err := reader.ReadAt(sample, sampleOffset); err != nil {
		return false, fmt.Errorf("read still-image-time sample: %w", err)
	}
	return sampleContainsStillImageTime(sample)
}

func sampleContainsStillImageTime(sample []byte) (bool, error) {
	// A timed metadata sample may contain several length-prefixed values, so walk
	// every record instead of assuming the still-image-time value fills the sample.
	for offset := 0; offset < len(sample); {
		if len(sample)-offset < 8 {
			return false, fmt.Errorf("truncated timed metadata value")
		}
		valueSize := int(binary.BigEndian.Uint32(sample[offset : offset+4]))
		if valueSize < 9 || valueSize > len(sample)-offset {
			return false, fmt.Errorf("invalid timed metadata value size")
		}
		keyIndex := binary.BigEndian.Uint32(sample[offset+4 : offset+8])
		if keyIndex == 1 && sample[offset+8] == 0xff {
			return true, nil
		}
		offset += valueSize
	}
	return false, nil
}

func sampleDescriptionContainsStillImageTime(reader io.ReaderAt, box mp4Box) (bool, error) {
	payload, err := readBoxPayload(reader, box, 4<<20)
	if err != nil {
		return false, err
	}
	if len(payload) < 8 {
		return false, fmt.Errorf("truncated sample description")
	}
	entryCount := int(binary.BigEndian.Uint32(payload[4:8]))
	offset := 8
	needle := []byte("com.apple.quicktime.still-image-time")
	for index := 0; index < entryCount; index++ {
		if len(payload)-offset < 8 {
			return false, fmt.Errorf("truncated sample description entry")
		}
		entrySize := int(binary.BigEndian.Uint32(payload[offset : offset+4]))
		if entrySize < 8 || entrySize > len(payload)-offset {
			return false, fmt.Errorf("invalid sample description entry size")
		}
		if string(payload[offset+4:offset+8]) == "mebx" && bytes.Contains(payload[offset+8:offset+entrySize], needle) {
			return true, nil
		}
		offset += entrySize
	}
	return false, nil
}

func readFirstSampleSize(reader io.ReaderAt, box mp4Box) (uint32, error) {
	payload, err := readBoxPayload(reader, box, 1<<20)
	if err != nil {
		return 0, err
	}
	if len(payload) < 12 {
		return 0, fmt.Errorf("truncated sample size box")
	}
	fixedSize := binary.BigEndian.Uint32(payload[4:8])
	sampleCount := binary.BigEndian.Uint32(payload[8:12])
	if sampleCount == 0 {
		return 0, fmt.Errorf("still-image-time track has no samples")
	}
	if fixedSize != 0 {
		return fixedSize, nil
	}
	if len(payload) < 16 {
		return 0, fmt.Errorf("truncated variable sample size box")
	}
	return binary.BigEndian.Uint32(payload[12:16]), nil
}

func readFirstChunkOffset(reader io.ReaderAt, box mp4Box) (int64, error) {
	payload, err := readBoxPayload(reader, box, 1<<20)
	if err != nil {
		return 0, err
	}
	if len(payload) < 12 || binary.BigEndian.Uint32(payload[4:8]) == 0 {
		return 0, fmt.Errorf("still-image-time track has no chunks")
	}
	if string(box.typ[:]) == "co64" {
		if len(payload) < 16 {
			return 0, fmt.Errorf("truncated 64-bit chunk offset box")
		}
		offset := binary.BigEndian.Uint64(payload[8:16])
		if offset > uint64(^uint64(0)>>1) {
			return 0, fmt.Errorf("chunk offset is too large")
		}
		return int64(offset), nil
	}
	return int64(binary.BigEndian.Uint32(payload[8:12])), nil
}

func readBoxPayload(reader io.ReaderAt, box mp4Box, maxSize int64) ([]byte, error) {
	size := box.end - box.payloadStart
	if size < 0 || size > maxSize {
		return nil, fmt.Errorf("%s box payload is too large", string(box.typ[:]))
	}
	payload := make([]byte, size)
	if _, err := reader.ReadAt(payload, box.payloadStart); err != nil {
		return nil, fmt.Errorf("read %s box payload: %w", string(box.typ[:]), err)
	}
	return payload, nil
}
