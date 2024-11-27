// source_body.go
package main

import (
    "bytes"
    "io"
    "net/http"
    "strings"
)

const (
    formFieldName = "file"
    maxMemory     = 64 << 20 // 64 MB using bit shifting
)

const ImageSourceTypeBody ImageSourceType = "payload"

type BodyImageSource struct {
    Config *SourceConfig
}

func NewBodyImageSource(config *SourceConfig) ImageSource {
    return &BodyImageSource{config}
}

func (s *BodyImageSource) Matches(r *http.Request) bool {
    return r.Method == http.MethodPost || r.Method == http.MethodPut
}

func (s *BodyImageSource) GetImage(r *http.Request) ([]byte, error) {
    if isFormBody(r) {
        return readFormBody(r)
    }
    return readRawBody(r)
}

// isFormBody checks if request contains multipart form data
func isFormBody(r *http.Request) bool {
    return strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/")
}

// readFormBody handles multipart form file uploads
func readFormBody(r *http.Request) ([]byte, error) {
    if err := r.ParseMultipartForm(maxMemory); err != nil {
        return nil, err
    }
    defer r.MultipartForm.RemoveAll()

    file, _, err := r.FormFile(formFieldName)
    if err != nil {
        return nil, err
    }
    defer file.Close()

    // Pre-allocate buffer with known size if available
    var buf *bytes.Buffer
    if size := r.ContentLength; size > 0 {
        buf = bytes.NewBuffer(make([]byte, 0, size))
    } else {
        buf = new(bytes.Buffer)
    }

    if _, err := io.Copy(buf, file); err != nil {
        return nil, err
    }

    if buf.Len() == 0 {
        return nil, ErrEmptyBody
    }

    return buf.Bytes(), nil
}

// readRawBody handles raw request body data
func readRawBody(r *http.Request) ([]byte, error) {
    // Use LimitReader to prevent memory exhaustion
    body, err := io.ReadAll(io.LimitReader(r.Body, maxMemory))
    defer r.Body.Close()
    
    if err != nil {
        return nil, err
    }
    
    if len(body) == 0 {
        return nil, ErrEmptyBody
    }
    
    return body, nil
}

func init() {
    RegisterSource(ImageSourceTypeBody, NewBodyImageSource)
}
