// source_http.go
package main

import (
    "context"
    "fmt"
    "io"
    "net/http"
    "net/url"
    "strings"
    "time"
)

const (
    ImageSourceTypeHTTP ImageSourceType = "http"
    URLQueryKey = "url"
)

type HTTPImageSource struct {
    Config *SourceConfig
    client *http.Client
}

func NewHTTPImageSource(config *SourceConfig) ImageSource {
    return &HTTPImageSource{
        Config: config,
        client: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

func (s *HTTPImageSource) Matches(r *http.Request) bool {
    return r.Method == http.MethodGet && r.URL.Query().Get(URLQueryKey) != ""
}

func (s *HTTPImageSource) GetImage(req *http.Request) ([]byte, error) {
    u, err := url.Parse(req.URL.Query().Get(URLQueryKey))
    if err != nil {
        return nil, ErrInvalidImageURL
    }
    
    if s.shouldRestrictOrigin(u) {
        return nil, fmt.Errorf("not allowed remote URL origin: %s%s", u.Host, u.Path)
    }
    
    return s.fetchImage(u, req)
}

func (s *HTTPImageSource) shouldRestrictOrigin(url *url.URL) bool {
    if len(s.Config.AllowedOrigins) == 0 {
        return false
    }

    for _, origin := range s.Config.AllowedOrigins {
        if origin.Host == url.Host && strings.HasPrefix(url.Path, origin.Path) {
            return false
        }

        if strings.HasPrefix(origin.Host, "*.") {
            suffix := origin.Host[1:]
            if (url.Host == origin.Host[2:] || strings.HasSuffix(url.Host, suffix)) && 
                strings.HasPrefix(url.Path, origin.Path) {
                return false
            }
        }
    }
    return true
}

func (s *HTTPImageSource) fetchImage(url *url.URL, ireq *http.Request) ([]byte, error) {
    ctx := ireq.Context()
    
    if s.Config.MaxAllowedSize > 0 {
        if err := s.checkImageSize(ctx, url, ireq); err != nil {
            return nil, err
        }
    }

    req := s.newRequest(ctx, http.MethodGet, url, ireq)
    res, err := s.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("error fetching remote http image: %w", err)
    }
    defer res.Body.Close()
    
    if res.StatusCode != http.StatusOK {
        return nil, NewError(fmt.Sprintf("error fetching remote http image: (status=%d) (url=%s)", 
            res.StatusCode, req.URL.String()), res.StatusCode)
    }

    // Pre-allocate buffer with response content length if available
    var buf []byte
    if res.ContentLength > 0 {
        buf = make([]byte, 0, res.ContentLength)
    }
    buf, err = io.ReadAll(res.Body)
    if err != nil {
        return nil, fmt.Errorf("unable to read image from response: %w", err)
    }
    
    return buf, nil
}

func (s *HTTPImageSource) checkImageSize(ctx context.Context, url *url.URL, ireq *http.Request) error {
    req := s.newRequest(ctx, http.MethodHead, url, ireq)
    res, err := s.client.Do(req)
    if err != nil {
        return fmt.Errorf("error checking image size: %w", err)
    }
    defer res.Body.Close()

    if res.StatusCode < 200 || res.StatusCode > 206 {
        return NewError(fmt.Sprintf("invalid status checking image size: (status=%d) (url=%s)", 
            res.StatusCode, url.String()), res.StatusCode)
    }

    if contentLength := res.ContentLength; contentLength > int64(s.Config.MaxAllowedSize) {
        return fmt.Errorf("content length %d exceeds maximum allowed %d bytes", 
            contentLength, s.Config.MaxAllowedSize)
    }
    
    return nil
}

func (s *HTTPImageSource) newRequest(ctx context.Context, method string, url *url.URL, ireq *http.Request) *http.Request {
    req, _ := http.NewRequestWithContext(ctx, method, url.String(), nil)
    req.Header.Set("User-Agent", "imaginary/"+Version)
    req.URL = url

    if len(s.Config.ForwardHeaders) > 0 {
        s.setForwardHeaders(req, ireq)
    }

    if s.Config.AuthForwarding || s.Config.Authorization != "" {
        s.setAuthorizationHeader(req, ireq)
    }

    return req
}

func (s *HTTPImageSource) setAuthorizationHeader(req, ireq *http.Request) {
    if auth := s.Config.Authorization; auth != "" {
        req.Header.Set("Authorization", auth)
        return
    }
    
    if auth := ireq.Header.Get("X-Forward-Authorization"); auth != "" {
        req.Header.Set("Authorization", auth)
        return
    }
    
    if auth := ireq.Header.Get("Authorization"); auth != "" {
        req.Header.Set("Authorization", auth)
    }
}

func (s *HTTPImageSource) setForwardHeaders(req, ireq *http.Request) {
    for _, header := range s.Config.ForwardHeaders {
        if value := ireq.Header.Get(header); value != "" {
            req.Header.Set(header, value)
        }
    }
}

func init() {
    RegisterSource(ImageSourceTypeHTTP, NewHTTPImageSource)
}
