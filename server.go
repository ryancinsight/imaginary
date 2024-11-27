// server.go
package main

import (
    "crypto/tls"
    "context"
    "log"
    "net/http"
    "net/url"
    "os"
    "os/signal"
    "path"
    "strconv"
    "strings"
    "syscall"
    "time"
)

// ServerOptions defines configuration options for the HTTP server
type ServerOptions struct {
    Port               int
    Burst              int
    Concurrency        int
    HTTPCacheTTL       int
    HTTPReadTimeout    int
    HTTPWriteTimeout   int
    MaxAllowedSize     int
    MaxAllowedPixels   float64
    CORS               bool
    Gzip               bool
    AuthForwarding     bool
    EnableURLSource    bool
    EnablePlaceholder  bool
    EnableURLSignature bool
    URLSignatureKey    string
    Address            string
    PathPrefix         string
    APIKey             string
    Mount              string
    CertFile          string
    KeyFile           string
    Authorization     string
    Placeholder       string
    PlaceholderStatus int
    ForwardHeaders    []string
    PlaceholderImage  []byte
    Endpoints         Endpoints
    AllowedOrigins    []*url.URL
    LogLevel          string
    ReturnSize        bool
}

// Endpoints represents a list of API endpoints
type Endpoints []string

// IsValid checks if the request endpoint is allowed
func (e Endpoints) IsValid(r *http.Request) bool {
    parts := strings.Split(r.URL.Path, "/")
    endpoint := parts[len(parts)-1]
    for _, name := range e {
        if endpoint == name {
            return false
        }
    }
    return true
}

// NewServerMux creates and configures the HTTP request multiplexer
func NewServerMux(o ServerOptions) http.Handler {
    mux := http.NewServeMux()
    
    // Core endpoints
    mux.Handle(path.Join(o.PathPrefix, "/"), Middleware(indexController(o), o))
    mux.Handle(path.Join(o.PathPrefix, "/form"), Middleware(formController(o), o))
    mux.Handle(path.Join(o.PathPrefix, "/health"), Middleware(healthController, o))

    // Image processing middleware
    image := ImageMiddleware(o)
    
	// Image operation endpoints
	endpoints := map[string]ImageOperation{
		"/resize": Resize,
		"/fit": Fit,
		"/enlarge": Enlarge,
		"/extract": Extract,
		"/crop": Crop,
		"/smartcrop": SmartCrop,
		"/rotate": Rotate,
		"/autorotate": AutoRotate,
		"/flip": Flip,
		"/flop": Flop,
		"/thumbnail": Thumbnail,
		"/zoom": Zoom,
		"/convert": Convert,
		"/watermark": Watermark,
		"/watermarkimage": WatermarkImage,
		"/info": Info,
		"/blur": GaussianBlur,
		"/pipeline": Pipeline,
	}


    for route, operation := range endpoints {
        mux.Handle(path.Join(o.PathPrefix, route), image(operation))
    }

    return mux
}

// Server initializes and runs the HTTP server
func Server(o ServerOptions) {
    addr := o.Address + ":" + strconv.Itoa(o.Port)
    
    // Configure TLS
    tlsConfig := &tls.Config{
        MinVersion: tls.VersionTLS12,
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
            tls.CurveP384,
        },
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        },
        NextProtos: []string{"h2", "http/1.1"},
    }

    // Initialize server
    server := &http.Server{
        Addr:           addr,
        Handler:        NewLog(NewServerMux(o), os.Stdout, o.LogLevel),
        MaxHeaderBytes: 1 << 20,
        ReadTimeout:    time.Duration(o.HTTPReadTimeout) * time.Second,
        WriteTimeout:   time.Duration(o.HTTPWriteTimeout) * time.Second,
        IdleTimeout:    120 * time.Second,
        TLSConfig:      tlsConfig,
    }

    // Setup graceful shutdown
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

    // Start server
    go func() {
        if err := listenAndServe(server, o); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    // Wait for shutdown signal
    <-shutdown
    log.Print("shutting down server")

    // Graceful shutdown with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := server.Shutdown(ctx); err != nil {
        log.Fatalf("server shutdown failed: %v", err)
    }
}

// listenAndServe starts the server with or without TLS
func listenAndServe(s *http.Server, o ServerOptions) error {
    if o.CertFile != "" && o.KeyFile != "" {
        return s.ListenAndServeTLS(o.CertFile, o.KeyFile)
    }
    return s.ListenAndServe()
}
