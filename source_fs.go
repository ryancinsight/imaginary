// source_fs.go
package main

import (
    "errors"
    "fmt"
    "io/fs"
    "os"
    "net/http"
    "net/url"
    "path/filepath"
    "strings"
)

const (
    ImageSourceTypeFileSystem ImageSourceType = "fs"
    fileParam = "file"
)

type FileSystemImageSource struct {
    Config *SourceConfig
}

func NewFileSystemImageSource(config *SourceConfig) ImageSource {
    return &FileSystemImageSource{config}
}

func (s *FileSystemImageSource) Matches(r *http.Request) bool {
    if r.Method != http.MethodGet {
        return false
    }
    // Avoid allocating memory for file param if method is not GET
    return r.URL.Query().Get(fileParam) != ""
}

func (s *FileSystemImageSource) GetImage(r *http.Request) ([]byte, error) {
    file, err := s.getFileParam(r)
    if err != nil {
        return nil, err
    }

    if file == "" {
        return nil, ErrMissingParamFile
    }

    // Build path and validate in one step
    cleanPath := filepath.Clean(filepath.Join(s.Config.MountPath, file))
    if !strings.HasPrefix(cleanPath, s.Config.MountPath) {
        return nil, ErrInvalidFilePath
    }

    // Read file with proper error handling
    return s.read(cleanPath)
}

func (s *FileSystemImageSource) read(file string) ([]byte, error) {
    // Use os.Open instead of ReadFile for better memory control
    f, err := os.Open(file)
    if err != nil {
        if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
            return nil, ErrInvalidFilePath
        }
        return nil, fmt.Errorf("failed to read file: %w", err)
    }
    defer f.Close()

    // Get file info for size
    info, err := f.Stat()
    if err != nil {
        return nil, fmt.Errorf("failed to stat file: %w", err)
    }

    // Pre-allocate buffer with exact size
    buf := make([]byte, info.Size())
    _, err = f.Read(buf)
    if err != nil {
        return nil, fmt.Errorf("failed to read file contents: %w", err)
    }

    return buf, nil
}

func (s *FileSystemImageSource) getFileParam(r *http.Request) (string, error) {
    // Get query value without allocating a new map
    fileQuery := r.URL.Query().Get(fileParam)
    if fileQuery == "" {
        return "", nil
    }
    
    return url.QueryUnescape(fileQuery)
}

func init() {
    RegisterSource(ImageSourceTypeFileSystem, NewFileSystemImageSource)
}
