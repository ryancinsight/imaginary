// log.go
package main

import (
    "fmt"
    "io"
    "net/http"
    "strings"
    "time"
)

const formatPattern = "%s - - [%s] \"%s\" %d %d %.4f\n"

// LogRecord implements an Apache-compatible HTTP logging
type LogRecord struct {
    http.ResponseWriter
    status         int
    responseBytes  int64
    ip            string
    method        string
    uri           string
    protocol      string
    time          time.Time
    elapsedTime   time.Duration
}

// Log writes a log entry to the output stream
func (r *LogRecord) Log(out io.Writer) {
    timeFormat := r.time.Format("02/Jan/2006 15:04:05")
    request := fmt.Sprintf("%s %s %s", r.method, r.uri, r.protocol)
    _, _ = fmt.Fprintf(out, formatPattern, r.ip, timeFormat, request, r.status, r.responseBytes, r.elapsedTime.Seconds())
}

// Write counts bytes written and forwards to ResponseWriter
func (r *LogRecord) Write(p []byte) (int, error) {
    written, err := r.ResponseWriter.Write(p)
    r.responseBytes += int64(written)
    return written, err
}

// WriteHeader sets status code and forwards to ResponseWriter
func (r *LogRecord) WriteHeader(status int) {
    r.status = status
    r.ResponseWriter.WriteHeader(status)
}

// LogHandler handles HTTP request logging
type LogHandler struct {
    handler  http.Handler
    io       io.Writer
    logLevel string
}

// NewLog creates a new logger handler
func NewLog(handler http.Handler, io io.Writer, logLevel string) http.Handler {
    return &LogHandler{handler, io, logLevel}
}

// ServeHTTP implements http.Handler interface
func (h *LogHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract client IP without port
    clientIP := r.RemoteAddr
    if colon := strings.LastIndex(clientIP, ":"); colon != -1 {
        clientIP = clientIP[:colon]
    }

    // Create log record
    record := &LogRecord{
        ResponseWriter: w,
        ip:            clientIP,
        time:          time.Time{},
        method:        r.Method,
        uri:           r.RequestURI,
        protocol:      r.Proto,
        status:        http.StatusOK,
        elapsedTime:   time.Duration(0),
    }

    // Track request timing
    startTime := time.Now()
    h.handler.ServeHTTP(record, r)
    finishTime := time.Now()

    record.time = finishTime.UTC()
    record.elapsedTime = finishTime.Sub(startTime)

    // Log based on configured level
    switch h.logLevel {
    case "error":
        if record.status >= http.StatusInternalServerError {
            record.Log(h.io)
        }
    case "warning":
        if record.status >= http.StatusBadRequest {
            record.Log(h.io)
        }
    case "info":
        record.Log(h.io)
    }
}
