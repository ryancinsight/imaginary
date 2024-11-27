// controllers.go
package main

import (
	"encoding/json"
	"fmt"
	"github.com/h2non/filetype"
	"github.com/h2non/bimg"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
)

// indexController handles the root endpoint, returning version information
func indexController(o ServerOptions) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path.Join(o.PathPrefix, "/") {
			ErrorReply(r, w, ErrNotFound, ServerOptions{})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(Versions{Version, bimg.Version, bimg.VipsVersion})
	}
}

// healthController returns server health statistics
func healthController(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(GetHealthStats())
}

// imageController processes image operations based on the source
func imageController(o ServerOptions, operation Operation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		source := MatchSource(r)
		if source == nil {
			ErrorReply(r, w, ErrMissingImageSource, o)
			return
		}

		buf, err := source.GetImage(r)
		if err != nil {
			if xerr, ok := err.(Error); ok {
				ErrorReply(r, w, xerr, o)
			} else {
				ErrorReply(r, w, NewError(err.Error(), http.StatusBadRequest), o)
			}
			return
		}

		if len(buf) == 0 {
			ErrorReply(r, w, ErrEmptyBody, o)
			return
		}

		imageHandler(w, r, buf, operation, o)
	}
}

// determineAcceptMimeType extracts preferred image format from Accept header
func determineAcceptMimeType(accept string) string {
	mimeMap := map[string]string{
		"image/webp": "webp",
		"image/png":  "png",
		"image/jpeg": "jpeg",
	}

	for _, v := range strings.Split(accept, ",") {
		if mediaType, _, _ := mime.ParseMediaType(v); mimeMap[mediaType] != "" {
			return mimeMap[mediaType]
		}
	}
	return ""
}

// imageHandler processes and responds with the transformed image
func imageHandler(w http.ResponseWriter, r *http.Request, buf []byte, operation Operation, o ServerOptions) {
	mimeType := detectMimeType(buf)
	if !IsImageMimeTypeSupported(mimeType) {
		ErrorReply(r, w, ErrUnsupportedMedia, o)
		return
	}

	opts, err := buildParamsFromQuery(r.URL.Query())
	if err != nil {
		ErrorReply(r, w, NewError("Error while processing parameters: "+err.Error(), http.StatusBadRequest), o)
		return
	}

	vary := ""
	if opts.Type == "auto" {
		opts.Type = determineAcceptMimeType(r.Header.Get("Accept"))
		vary = "Accept"
	} else if opts.Type != "" && ImageType(opts.Type) == 0 {
		ErrorReply(r, w, ErrOutputFormat, o)
		return
	}

	sizeInfo, err := bimg.Size(buf)
	if err != nil {
		ErrorReply(r, w, NewError("Error processing image: "+err.Error(), http.StatusBadRequest), o)
		return
	}

	if (float64(sizeInfo.Width) * float64(sizeInfo.Height) / 1000000) > o.MaxAllowedPixels {
		ErrorReply(r, w, ErrResolutionTooBig, o)
		return
	}

	image, err := operation.Run(buf, opts)
	if err != nil {
		if vary != "" {
			w.Header().Set("Vary", vary)
		}
		ErrorReply(r, w, NewError("Error processing image: "+err.Error(), http.StatusBadRequest), o)
		return
	}

	writeImageResponse(w, image, vary, o)
}

// detectMimeType determines the MIME type of the image buffer
func detectMimeType(buf []byte) string {
	mimeType := http.DetectContentType(buf)
	if mimeType == "application/octet-stream" {
		if kind, err := filetype.Get(buf); err == nil && kind.MIME.Value != "" {
			mimeType = kind.MIME.Value
		}
	}
	if strings.Contains(mimeType, "text/plain") && len(buf) > 8 && bimg.IsSVGImage(buf) {
		mimeType = "image/svg+xml"
	}
	return mimeType
}

// writeImageResponse writes the processed image to the response
func writeImageResponse(w http.ResponseWriter, image Image, vary string, o ServerOptions) {
	header := w.Header()
	header.Set("Content-Length", strconv.Itoa(len(image.Body)))
	header.Set("Content-Type", image.Mime)

	if image.Mime != "application/json" && o.ReturnSize {
		if meta, err := bimg.Metadata(image.Body); err == nil {
			header.Set("Image-Width", strconv.Itoa(meta.Size.Width))
			header.Set("Image-Height", strconv.Itoa(meta.Size.Height))
		}
	}

	if vary != "" {
		header.Set("Vary", vary)
	}

	w.Write(image.Body)
}

// formController generates HTML form for image operations
func formController(o ServerOptions) http.HandlerFunc {
	operations := []struct {
		name, method, args string
	}{
		{"Resize", "resize", "width=300&height=200&type=jpeg"},
		{"Force resize", "resize", "width=300&height=200&force=true"},
		{"Crop", "crop", "width=300&quality=95"},
		{"SmartCrop", "crop", "width=300&height=260&quality=95&gravity=smart"},
		{"Extract", "extract", "top=100&left=100&areawidth=300&areaheight=150"},
		{"Enlarge", "enlarge", "width=1440&height=900&quality=95"},
		{"Rotate", "rotate", "rotate=180"},
		{"AutoRotate", "autorotate", "quality=90"},
		{"Flip", "flip", ""},
		{"Flop", "flop", ""},
		{"Thumbnail", "thumbnail", "width=100"},
		{"Zoom", "zoom", "factor=2&areawidth=300&top=80&left=80"},
		{"Color space (black&white)", "resize", "width=400&height=300&colorspace=bw"},
		{"Add watermark", "watermark", "textwidth=100&text=Hello&font=sans%2012&opacity=0.5&color=255,200,50"},
		{"Convert format", "convert", "type=png"},
		{"Image metadata", "info", ""},
		{"Gaussian blur", "blur", "sigma=15.0&minampl=0.2"},
		{"Pipeline", "pipeline", "operations=%5B%7B%22operation%22:%20%22crop%22,%20%22params%22:%20%7B%22width%22:%20300,%20%22height%22:%20260%7D%7D,%20%7B%22operation%22:%20%22convert%22,%20%22params%22:%20%7B%22type%22:%20%22webp%22%7D%7D%5D"},
	}

	return func(w http.ResponseWriter, r *http.Request) {
		var html strings.Builder
		html.WriteString("<html><body>")
		for _, op := range operations {
			fmt.Fprintf(&html, `<h1>%s</h1><form method="POST" action="%s?%s" enctype="multipart/form-data"><input type="file" name="file" /><input type="submit" value="Upload" /></form>`,
				op.name, path.Join(o.PathPrefix, op.method), op.args)
		}
		html.WriteString("</body></html>")
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(html.String()))
	}
}
