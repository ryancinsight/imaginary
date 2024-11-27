// middleware.go
package main

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "fmt"
    "net/http"
    "strings"
    "time"
	"io"
    "github.com/h2non/bimg"
    "github.com/rs/cors"
    "github.com/throttled/throttled/v2"
    "github.com/throttled/throttled/v2/store/memstore"
)

type ImageOperation func([]byte, ImageOptions) (Image, error)

func Middleware(fn http.HandlerFunc, o ServerOptions) http.Handler {
    next := http.Handler(fn)

    if len(o.Endpoints) > 0 {
        next = validateEndpoints(next, o)
    }
    if o.Concurrency > 0 {
        next = throttleRequests(next, o)
    }
    if o.CORS {
        next = cors.Default().Handler(next)
    }
    if o.APIKey != "" {
        next = authorize(next, o)
    }
    if o.HTTPCacheTTL >= 0 {
        next = addCacheHeaders(next, o.HTTPCacheTTL)
    }

    return validateRequest(addDefaultHeaders(next), o)
}

func ImageMiddleware(o ServerOptions) func(ImageOperation) http.Handler {
    return func(operation ImageOperation) http.Handler {
        fn := createImageHandler(o, operation)
        handler := validateImageRequest(Middleware(fn, o), o)

        if o.EnableURLSignature {
            handler = checkURLSignature(handler, o)
        }

        return handler
    }
}
// Helper functions for image retrieval
func getImageFromURL(r *http.Request, o ServerOptions) ([]byte, error) {
    source := MatchSource(r)
    if source == nil {
        return nil, fmt.Errorf("missing image source")
    }
    
    return source.GetImage(r)
}

func getImageFromRequest(r *http.Request) ([]byte, error) {
    file, _, err := r.FormFile("file")
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    return io.ReadAll(file)
}

func createImageHandler(o ServerOptions, operation ImageOperation) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var buf []byte
        var err error

        if r.Method == http.MethodGet {
            buf, err = getImageFromURL(r, o)
        } else {
            buf, err = getImageFromRequest(r)
        }

        if err != nil {
            ErrorReply(r, w, NewError("Error getting image: "+err.Error(), http.StatusBadRequest), o)
            return
        }

        if len(buf) == 0 {
            ErrorReply(r, w, ErrEmptyBody, o)
            return
        }

        opts, err := buildParamsFromQuery(r.URL.Query())
        if err != nil {
            ErrorReply(r, w, NewError("Error parsing parameters: "+err.Error(), http.StatusBadRequest), o)
            return
        }

        image, err := operation(buf, opts)
        if err != nil {
            ErrorReply(r, w, NewError("Error processing image: "+err.Error(), http.StatusBadRequest), o)
            return
        }

        w.Header().Set("Content-Type", image.Mime)
        w.Header().Set("Content-Length", fmt.Sprint(len(image.Body)))
        w.Write(image.Body)
    }
}

func validateEndpoints(next http.Handler, o ServerOptions) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if o.Endpoints.IsValid(r) {
            next.ServeHTTP(w, r)
            return
        }
        ErrorReply(r, w, ErrNotImplemented, o)
    })
}

func throttleRequests(next http.Handler, o ServerOptions) http.Handler {
    store, err := memstore.New(65536)
    if err != nil {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            http.Error(w, fmt.Sprintf("throttle error: %v", err), http.StatusInternalServerError)
        })
    }

    quota := throttled.RateQuota{MaxRate: throttled.PerSec(o.Concurrency), MaxBurst: o.Burst}
    rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
    if err != nil {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            http.Error(w, fmt.Sprintf("throttle error: %v", err), http.StatusInternalServerError)
        })
    }

    return (&throttled.HTTPRateLimiter{
        RateLimiter: rateLimiter,
        VaryBy:      &throttled.VaryBy{Method: true},
    }).RateLimit(next)
}

func authorize(next http.Handler, o ServerOptions) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        key := r.Header.Get("API-Key")
        if key == "" {
            key = r.URL.Query().Get("key")
        }
        if key != o.APIKey {
            ErrorReply(r, w, ErrInvalidAPIKey, o)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func addDefaultHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Server", fmt.Sprintf("imaginary %s (bimg %s)", Version, bimg.Version))
        next.ServeHTTP(w, r)
    })
}

func addCacheHeaders(next http.Handler, ttl int) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet && !isPublicPath(r.URL.Path) {
            expires := time.Now().Add(time.Duration(ttl) * time.Second)
            w.Header().Set("Expires", strings.Replace(expires.Format(time.RFC1123), "UTC", "GMT", -1))
            w.Header().Set("Cache-Control", getCacheControl(ttl))
        }
        next.ServeHTTP(w, r)
    })
}

func validateRequest(next http.Handler, o ServerOptions) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodGet && r.Method != http.MethodPost {
            ErrorReply(r, w, ErrMethodNotAllowed, o)
            return
        }
        next.ServeHTTP(w, r)
    })
}

func validateImageRequest(next http.Handler, o ServerOptions) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if r.Method == http.MethodGet {
            if isPublicPath(r.URL.Path) {
                next.ServeHTTP(w, r)
                return
            }
            if o.Mount == "" && !o.EnableURLSource {
                ErrorReply(r, w, ErrGetMethodNotAllowed, o)
                return
            }
        }
        next.ServeHTTP(w, r)
    })
}

func checkURLSignature(next http.Handler, o ServerOptions) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        query := r.URL.Query()
        sign := query.Get("sign")
        query.Del("sign")

        h := hmac.New(sha256.New, []byte(o.URLSignatureKey))
        h.Write([]byte(r.URL.Path))
        h.Write([]byte(query.Encode()))
        expectedSign := h.Sum(nil)

        urlSign, err := base64.RawURLEncoding.DecodeString(sign)
        if err != nil {
            ErrorReply(r, w, ErrInvalidURLSignature, o)
            return
        }

        if !hmac.Equal(urlSign, expectedSign) {
            ErrorReply(r, w, ErrURLSignatureMismatch, o)
            return
        }

        next.ServeHTTP(w, r)
    })
}

func isPublicPath(path string) bool {
    switch path {
    case "/", "/health", "/form":
        return true
    default:
        return false
    }
}

func getCacheControl(ttl int) string {
    if ttl == 0 {
        return "private, no-cache, no-store, must-revalidate"
    }
    return fmt.Sprintf("public, s-maxage=%d, max-age=%d, no-transform", ttl, ttl)
}
