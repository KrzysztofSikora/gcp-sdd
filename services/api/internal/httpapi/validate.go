package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"unicode/utf8"
)

// Limits from specs/requirements.md FR-03 (Q-02).
const (
	maxBodyBytes  = 256 << 10
	maxTextLength = 10_000
)

// decodeText checks the content type and body of POST /analyze and returns the text.
// The method has already been checked by the caller.
func decodeText(w http.ResponseWriter, r *http.Request) (string, *apiError) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return "", errUnsupportedMediaType
	}

	raw, apiErr := readSingleObject(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if apiErr != nil {
		return "", apiErr
	}

	var req struct {
		Text json.RawMessage `json:"text"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		return "", errInvalidJSON
	}

	var text string
	if len(req.Text) == 0 || bytes.Equal(req.Text, []byte("null")) || json.Unmarshal(req.Text, &text) != nil {
		return "", errInvalidText
	}
	if strings.TrimSpace(text) == "" {
		return "", errEmptyText
	}
	if utf8.RuneCountInString(text) > maxTextLength {
		return "", errTextTooLong
	}
	return text, nil
}

// readSingleObject reads exactly one JSON value from body and requires it to be an object.
func readSingleObject(body io.Reader) (json.RawMessage, *apiError) {
	dec := json.NewDecoder(body)
	var raw json.RawMessage
	if err := dec.Decode(&raw); err != nil {
		return nil, bodyError(err)
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		if err != nil {
			return nil, bodyError(err)
		}
		return nil, errInvalidJSON // trailing data after the object
	}
	if raw[0] != '{' {
		return nil, errInvalidJSON
	}
	return raw, nil
}

func bodyError(err error) *apiError {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return errPayloadTooLarge
	}
	return errInvalidJSON
}
