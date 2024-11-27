package main

import (
    "net/http"
    "net/url"
    "sync"
)

// ImageSourceType represents the type of image source
type ImageSourceType string

// ImageSourceFactoryFunction defines the factory function signature
type ImageSourceFactoryFunction func(*SourceConfig) ImageSource

// SourceConfig holds configuration for image sources
type SourceConfig struct {
    AuthForwarding  bool
    Authorization   string
    MountPath       string
    Type           ImageSourceType
    ForwardHeaders []string
    AllowedOrigins []*url.URL
    MaxAllowedSize int
}

// ImageSource interface defines methods for image source handlers
type ImageSource interface {
    Matches(*http.Request) bool
    GetImage(*http.Request) ([]byte, error)
}

// sourceRegistry manages image source registration and lookup
type sourceRegistry struct {
    sources   map[ImageSourceType]ImageSource
    factories map[ImageSourceType]ImageSourceFactoryFunction
    mu       sync.RWMutex
}

// Initialize registry with pre-allocated maps
var registry = &sourceRegistry{
    sources:   make(map[ImageSourceType]ImageSource, 4),    // Pre-allocate for common sources
    factories: make(map[ImageSourceType]ImageSourceFactoryFunction, 4),
}

// RegisterSource registers a new image source factory
func RegisterSource(sourceType ImageSourceType, factory ImageSourceFactoryFunction) {
    if factory == nil {
        return // Early return to prevent nil factory registration
    }
    
    registry.mu.Lock()
    registry.factories[sourceType] = factory
    registry.mu.Unlock()
}

// LoadSources initializes all registered image sources
func LoadSources(o ServerOptions) {
    registry.mu.Lock()
    defer registry.mu.Unlock()
    
    // Reuse existing maps if possible
    if len(registry.sources) > 0 {
        for k := range registry.sources {
            delete(registry.sources, k)
        }
    }
    
    // Create single config instance
    config := &SourceConfig{
        AuthForwarding:  o.AuthForwarding,
        Authorization:   o.Authorization,
        MountPath:      o.Mount,
        AllowedOrigins: o.AllowedOrigins,
        MaxAllowedSize: o.MaxAllowedSize,
        ForwardHeaders: o.ForwardHeaders,
    }
    
    // Initialize sources with shared config
    for name, factory := range registry.factories {
        config.Type = name
        if source := factory(config); source != nil {
            registry.sources[name] = source
        }
    }
}

// MatchSource finds the appropriate source for a request
func MatchSource(req *http.Request) ImageSource {
    registry.mu.RLock()
    defer registry.mu.RUnlock()
    
    // Use read-only lock for concurrent access
    for _, source := range registry.sources {
        if source != nil && source.Matches(req) {
            return source
        }
    }
    return nil
}
