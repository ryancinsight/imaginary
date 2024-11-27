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
	URLQueryKey                         = "url"
	defaultTimeout                      = 60 * time.Second
)

type HTTPImageSource struct {
	Config *SourceConfig
	client *http.Client
}

func NewHTTPImageSource(config *SourceConfig) ImageSource {
	return &HTTPImageSource{
		Config: config,
		client: &http.Client{
			Timeout: defaultTimeout,
			Transport: &http.Transport{
				MaxIdleConns:       100,
				IdleConnTimeout:    90 * time.Second,
				DisableCompression: true,
				MaxConnsPerHost:    10,
				DisableKeepAlives:  false,
			},
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

	urlPath := url.Path
	urlHost := url.Host
	for _, origin := range s.Config.AllowedOrigins {
		if origin.Host == urlHost && strings.HasPrefix(urlPath, origin.Path) {
			return false
		}

		if strings.HasPrefix(origin.Host, "*.") {
			suffix := origin.Host[1:]
			if (urlHost == origin.Host[2:] || strings.HasSuffix(urlHost, suffix)) &&
				strings.HasPrefix(urlPath, origin.Path) {
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

	// Use io.ReadAll directly since we don't need the pre-allocated buffer
	return io.ReadAll(io.LimitReader(res.Body, int64(s.Config.MaxAllowedSize)))
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

	if s.Config.AuthForwarding || s.Config.Authorization != "" {
		s.setAuthorizationHeader(req, ireq)
	}

	if len(s.Config.ForwardHeaders) > 0 {
		s.setForwardHeaders(req, ireq)
	}

	return req
}

func (s *HTTPImageSource) setAuthorizationHeader(req, ireq *http.Request) {
	switch {
	case s.Config.Authorization != "":
		req.Header.Set("Authorization", s.Config.Authorization)
	case ireq.Header.Get("X-Forward-Authorization") != "":
		req.Header.Set("Authorization", ireq.Header.Get("X-Forward-Authorization"))
	case ireq.Header.Get("Authorization") != "":
		req.Header.Set("Authorization", ireq.Header.Get("Authorization"))
	}
}

func (s *HTTPImageSource) setForwardHeaders(req, ireq *http.Request) {
	headers := s.Config.ForwardHeaders
	for _, header := range headers {
		if value := ireq.Header.Get(header); value != "" {
			req.Header.Set(header, value)
		}
	}
}

func init() {
	RegisterSource(ImageSourceTypeHTTP, NewHTTPImageSource)
}
