package issue

import (
	"bytes"
	"net/http"
)

// Allowed attachment media types, keyed by the sniffed type, with the file
// extension used when storing them.
var allowedMedia = map[string]string{
	"image/png":  "png",
	"image/jpeg": "jpg",
	"image/webp": "webp",
	"video/webm": "webm",
	"video/mp4":  "mp4",
}

// mp4Brands are ISO-BMFF major brands we accept as MP4. QuickTime ("qt  ")
// and HEIF images ("heic", "mif1", ...) share the ftyp container but are not
// allowed.
var mp4Brands = map[string]bool{
	"isom": true, "iso2": true, "iso4": true, "iso5": true, "iso6": true,
	"mp41": true, "mp42": true, "avc1": true, "M4V ": true, "dash": true,
	"mmp4": true,
}

// SniffMedia determines the media type from the file's content (never the
// client-declared type). It returns the type and storage extension, or
// ok=false if the content is not an allowed type.
func SniffMedia(data []byte) (contentType, ext string, ok bool) {
	ct := http.DetectContentType(data)
	if ext, ok := allowedMedia[ct]; ok {
		return ct, ext, true
	}
	// http.DetectContentType only recognises MP4 when a brand starts with
	// "mp4"; MediaRecorder/Safari output may use e.g. "iso5" without one.
	if len(data) >= 12 && bytes.Equal(data[4:8], []byte("ftyp")) && mp4Brands[string(data[8:12])] {
		return "video/mp4", "mp4", true
	}
	return "", "", false
}

// IsVideo reports whether an allowed media type is a video.
func IsVideo(contentType string) bool {
	return contentType == "video/webm" || contentType == "video/mp4"
}
