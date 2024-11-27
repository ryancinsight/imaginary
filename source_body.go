package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
)

const (
	formFieldName   = "file"
	maxMemory       = 64 << 20 // 64 MB using bit shifting
	multipartPrefix = "multipart/"
)

// Add error definition
var ErrEntityTooLarge = errors.New("entity too large")

const ImageSourceTypeBody ImageSourceType = "payload"

type BodyImageSource struct {
	Config *SourceConfig
}

func NewBodyImageSource(config *SourceConfig) ImageSource {
	return &BodyImageSource{config}
}

func (s *BodyImageSource) Matches(r *http.Request) bool {
	switch r.Method {
	case http.MethodPost, http.MethodPut:
		return true
	default:
		return false
	}
}

func (s *BodyImageSource) GetImage(r *http.Request) ([]byte, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), multipartPrefix) {
		return readFormBody(r)
	}
	return readRawBody(r)
}

func readFormBody(r *http.Request) ([]byte, error) {
	// Parse with memory limit
	if err := r.ParseMultipartForm(maxMemory); err != nil {
		return nil, err
	}
	defer r.MultipartForm.RemoveAll()

	file, _, err := r.FormFile(formFieldName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Use buffer pooling for large files
	var buf *bytes.Buffer
	if size := r.ContentLength; size > 0 && size <= maxMemory {
		buf = bytes.NewBuffer(make([]byte, 0, size))
	} else {
		buf = bytes.NewBuffer(make([]byte, 0, bytes.MinRead))
	}

	// Copy with size limit
	written, err := io.CopyN(buf, file, maxMemory+1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if written > maxMemory {
		return nil, ErrEntityTooLarge
	}
	if buf.Len() == 0 {
		return nil, ErrEmptyBody
	}

	return buf.Bytes(), nil
}

func readRawBody(r *http.Request) ([]byte, error) {
	defer r.Body.Close()

	// Use LimitReader for memory safety
	limitReader := io.LimitReader(r.Body, maxMemory+1)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}

	if len(body) > maxMemory {
		return nil, ErrEntityTooLarge
	}
	if len(body) == 0 {
		return nil, ErrEmptyBody
	}

	return body, nil
}

func init() {
	RegisterSource(ImageSourceTypeBody, NewBodyImageSource)
}
