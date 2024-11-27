package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/h2non/bimg"
)

var (
	ErrNotFound             = NewError("Not found", http.StatusNotFound)
	ErrInvalidAPIKey        = NewError("Invalid or missing API key", http.StatusUnauthorized)
	ErrMethodNotAllowed     = NewError("HTTP method not allowed. Try with a POST or GET method (-enable-url-source flag must be defined)", http.StatusMethodNotAllowed)
	ErrGetMethodNotAllowed  = NewError("GET method not allowed. Make sure remote URL source is enabled by using the flag: -enable-url-source", http.StatusMethodNotAllowed)
	ErrUnsupportedMedia     = NewError("Unsupported media type", http.StatusNotAcceptable)
	ErrOutputFormat         = NewError("Unsupported output image format", http.StatusBadRequest)
	ErrEmptyBody            = NewError("Empty or unreadable image", http.StatusBadRequest)
	ErrMissingParamFile     = NewError("Missing required param: file", http.StatusBadRequest)
	ErrInvalidFilePath      = NewError("Invalid file path", http.StatusBadRequest)
	ErrInvalidImageURL      = NewError("Invalid image URL", http.StatusBadRequest)
	ErrMissingImageSource   = NewError("Cannot process the image due to missing or invalid params", http.StatusBadRequest)
	ErrNotImplemented       = NewError("Not implemented endpoint", http.StatusNotImplemented)
	ErrInvalidURLSignature  = NewError("Invalid URL signature", http.StatusBadRequest)
	ErrURLSignatureMismatch = NewError("URL signature mismatch", http.StatusForbidden)
	ErrResolutionTooBig     = NewError("Image resolution is too big", http.StatusUnprocessableEntity)
)

type Error struct {
	Message string `json:"message,omitempty"`
	Code    int    `json:"status"`
}

func (e Error) JSON() []byte {
	buf, _ := json.Marshal(e)
	return buf
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) HTTPCode() int {
	if e.Code >= 400 && e.Code <= 511 {
		return e.Code
	}
	return http.StatusServiceUnavailable
}

func NewError(err string, code int) Error {
	return Error{
		Message: strings.ReplaceAll(err, "\n", ""),
		Code:    code,
	}
}

func ErrorReply(req *http.Request, w http.ResponseWriter, err Error, o ServerOptions) {
	if o.EnablePlaceholder || o.Placeholder != "" {
		_ = replyWithPlaceholder(req, w, err, o)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.HTTPCode())
	w.Write(err.JSON())
}

func replyWithPlaceholder(req *http.Request, w http.ResponseWriter, errCaller Error, o ServerOptions) error {
	opts := bimg.Options{
		Force:   true,
		Crop:    true,
		Enlarge: true,
		Type:    ImageType(req.URL.Query().Get("type")),
	}

	query := req.URL.Query()
	width, err := parseInt(query.Get("width"))
	if err != nil {
		return sendError(w, http.StatusBadRequest, err)
	}
	opts.Width = width

	height, err := parseInt(query.Get("height"))
	if err != nil {
		return sendError(w, http.StatusBadRequest, err)
	}
	opts.Height = height

	image, err := bimg.Resize(o.PlaceholderImage, opts)
	if err != nil {
		return sendError(w, http.StatusBadRequest, err)
	}

	header := w.Header()
	header.Set("Content-Type", GetImageMimeType(bimg.DetermineImageType(image)))
	header.Set("Error", string(errCaller.JSON()))

	if o.PlaceholderStatus != 0 {
		w.WriteHeader(o.PlaceholderStatus)
	} else {
		w.WriteHeader(errCaller.HTTPCode())
	}

	w.Write(image)
	return errCaller
}

func sendError(w http.ResponseWriter, code int, err error) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write([]byte(fmt.Sprintf(`{"error":"%s", "status":%d}`, err.Error(), code)))
	return err
}
