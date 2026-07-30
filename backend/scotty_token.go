package backend

import (
	"bytes"
	"fmt"

	"app/generated"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
)

const scottyFinalizeTokenVersion = 2

// ScottyFinalizeToken retains the complete upload-finalize response because
// create-media embeds the outer envelope byte-for-byte, not just field 2. All
// observed envelopes used field 1 == 2 and one protected, opaque field 2; its
// contents and the two observed envelope lengths must not be interpreted as a
// media type or normalized by decoding and re-encoding.
type ScottyFinalizeToken struct {
	Raw []byte
}

func ParseScottyFinalizeToken(raw []byte) (ScottyFinalizeToken, error) {
	field1Count := 0
	field2Count := 0
	for offset := 0; offset < len(raw); {
		number, wireType, tagLength := protowire.ConsumeTag(raw[offset:])
		if tagLength < 0 {
			return ScottyFinalizeToken{}, fmt.Errorf("invalid Scotty finalize token tag: %w", protowire.ParseError(tagLength))
		}
		value := raw[offset+tagLength:]
		valueLength := protowire.ConsumeFieldValue(number, wireType, value)
		if valueLength < 0 {
			return ScottyFinalizeToken{}, fmt.Errorf("invalid Scotty finalize token field %d: %w", number, protowire.ParseError(valueLength))
		}

		switch number {
		case 1:
			if wireType != protowire.VarintType {
				return ScottyFinalizeToken{}, fmt.Errorf("Scotty finalize token field 1 has wire type %d", wireType)
			}
			version, consumed := protowire.ConsumeVarint(value)
			if consumed != valueLength || version != scottyFinalizeTokenVersion {
				return ScottyFinalizeToken{}, fmt.Errorf("unsupported Scotty finalize token version %d", version)
			}
			field1Count++
		case 2:
			if wireType != protowire.BytesType {
				return ScottyFinalizeToken{}, fmt.Errorf("Scotty finalize token field 2 has wire type %d", wireType)
			}
			opaque, consumed := protowire.ConsumeBytes(value)
			if consumed != valueLength || len(opaque) == 0 {
				return ScottyFinalizeToken{}, fmt.Errorf("Scotty finalize token field 2 is empty or incomplete")
			}
			field2Count++
		}
		offset += tagLength + valueLength
	}

	if field1Count != 1 {
		return ScottyFinalizeToken{}, fmt.Errorf("Scotty finalize token contains %d field-1 values, want 1", field1Count)
	}
	if field2Count != 1 {
		return ScottyFinalizeToken{}, fmt.Errorf("Scotty finalize token contains %d field-2 values, want 1", field2Count)
	}
	return ScottyFinalizeToken{Raw: bytes.Clone(raw)}, nil
}

// legacyCommitToken keeps the established single-file CommitUpload path intact.
// Linked Live Photos bypass this conversion and embed token.Raw unchanged.
func (token ScottyFinalizeToken) legacyCommitToken() (*generated.CommitToken, error) {
	validated, err := ParseScottyFinalizeToken(token.Raw)
	if err != nil {
		return nil, err
	}
	var decoded generated.CommitToken
	if err := proto.Unmarshal(validated.Raw, &decoded); err != nil {
		return nil, fmt.Errorf("decode legacy commit token: %w", err)
	}
	return &decoded, nil
}
