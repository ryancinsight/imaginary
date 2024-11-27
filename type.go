package main

import (
    "strings"
    "github.com/h2non/bimg"
)

func ExtractImageTypeFromMime(mime string) string {
    parts := strings.SplitN(mime, ";", 2)[0]
    subParts := strings.SplitN(parts, "/", 2)
    if len(subParts) < 2 {
        return ""
    }
    return strings.ToLower(strings.SplitN(subParts[1], "+", 2)[0])
}

func IsImageMimeTypeSupported(mime string) bool {
    format := ExtractImageTypeFromMime(mime)
    if format == "xml" {
        format = "svg"
    }
    return bimg.IsTypeNameSupported(format)
}

func ImageType(name string) bimg.ImageType {
    switch strings.ToLower(name) {
    case "jpeg", "jpg":
        return bimg.JPEG
    case "png":
        return bimg.PNG
    case "webp":
        return bimg.WEBP
    case "tiff":
        return bimg.TIFF
    case "gif":
        return bimg.GIF
    case "svg":
        return bimg.SVG
    case "pdf":
        return bimg.PDF
    default:
        return bimg.UNKNOWN
    }
}

func GetImageMimeType(code bimg.ImageType) string {
    mimeTypes := map[bimg.ImageType]string{
        bimg.PNG:  "image/png",
        bimg.WEBP: "image/webp",
        bimg.TIFF: "image/tiff",
        bimg.GIF:  "image/gif",
        bimg.SVG:  "image/svg+xml",
        bimg.PDF:  "application/pdf",
    }
    
    if mime, ok := mimeTypes[code]; ok {
        return mime
    }
    return "image/jpeg"
}
