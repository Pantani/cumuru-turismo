package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
)

const maxRequestBodyBytes = 1 << 20

var errInvalidJSON = errors.New("invalid JSON")

func decodeStrict(request *http.Request, expectedMediaType string, target any) error {
	mediaType, _, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil || mediaType != expectedMediaType {
		return errInvalidJSON
	}
	content, err := io.ReadAll(io.LimitReader(request.Body, maxRequestBodyBytes+1))
	if err != nil || len(content) == 0 || len(content) > maxRequestBodyBytes {
		return errInvalidJSON
	}
	if err := rejectDuplicateJSONKeys(content); err != nil {
		return errInvalidJSON
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errInvalidJSON
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return errInvalidJSON
	}
	return nil
}

func rejectDuplicateJSONKeys(content []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(content))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if err := scanJSONValue(decoder, token); err != nil {
		return err
	}
	return ensureJSONEnd(decoder)
}

func scanJSONValue(decoder *json.Decoder, token json.Token) error {
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		return scanJSONObject(decoder)
	case '[':
		return scanJSONArray(decoder)
	default:
		return errInvalidJSON
	}
}

func scanJSONObject(decoder *json.Decoder) error {
	seen := make(map[string]struct{})
	for decoder.More() {
		if err := scanJSONObjectEntry(decoder, seen); err != nil {
			return err
		}
	}
	return consumeDelimiter(decoder, '}')
}

func scanJSONObjectEntry(decoder *json.Decoder, seen map[string]struct{}) error {
	keyToken, err := decoder.Token()
	if err != nil {
		return err
	}
	key, ok := keyToken.(string)
	if !ok {
		return errInvalidJSON
	}
	if _, duplicate := seen[key]; duplicate {
		return errInvalidJSON
	}
	seen[key] = struct{}{}
	valueToken, err := decoder.Token()
	if err != nil {
		return err
	}
	return scanJSONValue(decoder, valueToken)
}
func scanJSONArray(decoder *json.Decoder) error {
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		if err := scanJSONValue(decoder, token); err != nil {
			return err
		}
	}
	return consumeDelimiter(decoder, ']')
}

func consumeDelimiter(decoder *json.Decoder, expected json.Delim) error {
	token, err := decoder.Token()
	if err != nil || token != expected {
		return errInvalidJSON
	}
	return nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errInvalidJSON
	}
	return nil
}
