package main

import (
    "net/http"
    "net/url"
    "sync"
)

type ImageSourceType string
type ImageSourceFactoryFunction func(*SourceConfig) ImageSource

type SourceConfig struct {
    AuthForwarding  bool
    Authorization   string
    MountPath       string
    Type           ImageSourceType
    ForwardHeaders []string
    AllowedOrigins []*url.URL
    MaxAllowedSize int
}

type ImageSource interface {
    Matches(*http.Request) bool
    GetImage(*http.Request) ([]byte, error)
}

type sourceRegistry struct {
    sources     map[ImageSourceType]ImageSource
    factories   map[ImageSourceType]ImageSourceFactoryFunction
    mu          sync.RWMutex
}

var registry = &sourceRegistry{
    sources:   make(map[ImageSourceType]ImageSource),
    factories: make(map[ImageSourceType]ImageSourceFactoryFunction),
}

func RegisterSource(sourceType ImageSourceType, factory ImageSourceFactoryFunction) {
    registry.mu.Lock()
    defer registry.mu.Unlock()
    registry.factories[sourceType] = factory
}

func LoadSources(o ServerOptions) {
    registry.mu.Lock()
    defer registry.mu.Unlock()
    
    config := &SourceConfig{
        AuthForwarding: o.AuthForwarding,
        Authorization:  o.Authorization,
        MountPath:     o.Mount,
        AllowedOrigins: o.AllowedOrigins,
        MaxAllowedSize: o.MaxAllowedSize,
        ForwardHeaders: o.ForwardHeaders,
    }
    
    for name, factory := range registry.factories {
        config.Type = name
        registry.sources[name] = factory(config)
    }
}

func MatchSource(req *http.Request) ImageSource {
    registry.mu.RLock()
    defer registry.mu.RUnlock()
    
    for _, source := range registry.sources {
        if source.Matches(req) {
            return source
        }
    }
    return nil
}
