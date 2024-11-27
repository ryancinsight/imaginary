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

const ImageSourceTypeFileSystem ImageSourceType = "fs"

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
    file, err := s.getFileParam(r)
    return err == nil && file != ""
}

func (s *FileSystemImageSource) GetImage(r *http.Request) ([]byte, error) {
    file, err := s.getFileParam(r)
    if err != nil {
        return nil, err
    }

    if file == "" {
        return nil, ErrMissingParamFile
    }

    file, err = s.buildPath(file)
    if err != nil {
        return nil, err
    }

    return s.read(file)
}

func (s *FileSystemImageSource) buildPath(file string) (string, error) {
    cleanPath := filepath.Clean(filepath.Join(s.Config.MountPath, file))
    if !strings.HasPrefix(cleanPath, s.Config.MountPath) {
        return "", ErrInvalidFilePath
    }
    return cleanPath, nil
}

func (s *FileSystemImageSource) read(file string) ([]byte, error) {
    buf, err := os.ReadFile(file)
    if err != nil {
        if errors.Is(err, fs.ErrNotExist) || errors.Is(err, fs.ErrPermission) {
            return nil, ErrInvalidFilePath
        }
        return nil, fmt.Errorf("failed to read file: %w", err)
    }
    return buf, nil
}

func (s *FileSystemImageSource) getFileParam(r *http.Request) (string, error) {
    unescaped, err := url.QueryUnescape(r.URL.Query().Get("file"))
    if err != nil {
        return "", fmt.Errorf("failed to unescape file param: %w", err)
    }
    return unescaped, nil
}

func init() {
    RegisterSource(ImageSourceTypeFileSystem, NewFileSystemImageSource)
}
