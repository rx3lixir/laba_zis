package audio

import (
	"path/filepath"
	"strings"
)

// detectAudioFormat determines file extension based on Content-Type and filename
func DetectAudioFormat(contentType, filename string) string {
	// Priority 1: Trust filename extension
	if filename != "" {
		ext := strings.ToLower(filepath.Ext(filename))
		switch ext {
		case ".webm":
			return "webm"
		case ".m4a", ".mp4":
			return "m4a"
		case ".mp3":
			return "mp3"
		case ".ogg", ".opus":
			return "ogg"
		case ".wav":
			return "wav"
		}
	}

	// Priority 2: Trust Content-Type
	switch {
	case strings.Contains(contentType, "webm"):
		return "webm"
	case strings.Contains(contentType, "mp4"), strings.Contains(contentType, "aac"):
		return "m4a"
	case strings.Contains(contentType, "mpeg"), strings.Contains(contentType, "mp3"):
		return "mp3"
	case strings.Contains(contentType, "ogg"):
		return "ogg"
	case strings.Contains(contentType, "wav"):
		return "wav"
	default:
		return "webm"
	}
}
