package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"time"

	"github.com/Pantani/cumuru/apps/api/internal/platform/config"
	"github.com/google/uuid"
)

var errInvalidCursor = errors.New("invalid cursor")

const (
	cursorTimestampBytes = 8
	cursorUUIDBytes      = 16
	cursorMACBytes       = sha256.Size
)

type pageCursor struct {
	CreatedAt time.Time `json:"created_at"`
	ID        uuid.UUID `json:"id"`
}

type cursorCodec struct {
	keyring config.KeyringConfig
}

func newCursorCodec(keyring config.KeyringConfig) (cursorCodec, error) {
	if keyring.CurrentVersion == "" || len(keyring.CurrentVersion) > 255 {
		return cursorCodec{}, errInvalidCursor
	}
	if _, ok := keyring.Key(keyring.CurrentVersion); !ok {
		return cursorCodec{}, errInvalidCursor
	}
	return cursorCodec{keyring: keyring}, nil
}

func parseLimit(requestLimit string) (int32, error) {
	limit := int64(25)
	var err error
	if requestLimit != "" {
		limit, err = strconv.ParseInt(requestLimit, 10, 32)
	}
	if err != nil || limit < 1 || limit > 100 {
		return 0, errInvalidCursor
	}
	return int32(limit), nil
}

func parsePage(codec cursorCodec, requestLimit, cursorValue string) (int32, pageCursor, error) {
	limit, err := parseLimit(requestLimit)
	if err != nil {
		return 0, pageCursor{}, err
	}
	if cursorValue == "" {
		return limit, pageCursor{}, nil
	}
	cursor, err := codec.decode(cursorValue)
	return limit, cursor, err
}

func (c cursorCodec) encode(cursor pageCursor) *string {
	if cursor.ID == uuid.Nil || cursor.CreatedAt.IsZero() {
		return nil
	}
	version := c.keyring.CurrentVersion
	key, ok := c.keyring.Key(version)
	if !ok || len(version) > 255 {
		return nil
	}
	payload := make([]byte, 1+len(version)+cursorTimestampBytes+cursorUUIDBytes)
	payload[0] = byte(len(version))
	copy(payload[1:], version)
	offset := 1 + len(version)
	binary.BigEndian.PutUint64(payload[offset:], uint64(cursor.CreatedAt.UTC().UnixNano()))
	copy(payload[offset+cursorTimestampBytes:], cursor.ID[:])
	content := append(payload, cursorMAC(key, payload)...)
	encoded := base64.RawURLEncoding.EncodeToString(content)
	return &encoded
}

func (c cursorCodec) decode(value string) (pageCursor, error) {
	content, err := decodeCursorContent(value)
	if err != nil {
		return pageCursor{}, errInvalidCursor
	}
	versionLength, payloadLength, version, err := cursorEnvelope(content)
	if err != nil {
		return pageCursor{}, errInvalidCursor
	}
	key, ok := c.keyring.Key(version)
	if !ok || !hmac.Equal(content[payloadLength:], cursorMAC(key, content[:payloadLength])) {
		return pageCursor{}, errInvalidCursor
	}
	return cursorFromPayload(content, versionLength, payloadLength)
}

func decodeCursorContent(value string) ([]byte, error) {
	content, err := base64.RawURLEncoding.DecodeString(value)
	minimum := 1 + cursorTimestampBytes + cursorUUIDBytes + cursorMACBytes
	if err != nil || len(content) < minimum {
		return nil, errInvalidCursor
	}
	return content, nil
}

func cursorEnvelope(content []byte) (int, int, string, error) {
	versionLength := int(content[0])
	payloadLength := 1 + versionLength + cursorTimestampBytes + cursorUUIDBytes
	if versionLength == 0 || len(content) != payloadLength+cursorMACBytes {
		return 0, 0, "", errInvalidCursor
	}
	return versionLength, payloadLength, string(content[1 : 1+versionLength]), nil
}

func cursorFromPayload(content []byte, versionLength, payloadLength int) (pageCursor, error) {
	offset := 1 + versionLength
	createdAt := time.Unix(0, int64(binary.BigEndian.Uint64(content[offset:]))).UTC()
	id, err := uuid.FromBytes(content[offset+cursorTimestampBytes : payloadLength])
	if err != nil || id == uuid.Nil || createdAt.IsZero() {
		return pageCursor{}, errInvalidCursor
	}
	return pageCursor{CreatedAt: createdAt, ID: id}, nil
}

func cursorMAC(key, payload []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte("page-cursor\x00"))
	_, _ = mac.Write(payload)
	return mac.Sum(nil)
}
