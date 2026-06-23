package pages

import (
	"path/filepath"
	"strings"
)

// webContentTypes is an explicit Content-Type map for the web extensions that
// matter. We deliberately do NOT rely on Go's mime.TypeByExtension alone: it
// reads /etc/mime.types and is platform-dependent, so common web assets can be
// served with a missing or wrong type (breaking .js modules, fonts, .wasm). A
// blank page that "deployed successfully" is a frequent symptom.
var webContentTypes = map[string]string{
	".html":  "text/html; charset=utf-8",
	".htm":   "text/html; charset=utf-8",
	".css":   "text/css; charset=utf-8",
	".js":    "text/javascript; charset=utf-8",
	".mjs":   "text/javascript; charset=utf-8",
	".json":  "application/json",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".wasm":  "application/wasm",
	".map":   "application/json",
	".txt":   "text/plain; charset=utf-8",
	".xml":   "application/xml",
	".pdf":   "application/pdf",
}

// ContentType returns the Content-Type for a file path based on its extension,
// falling back to application/octet-stream for anything unknown.
func ContentType(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ct, ok := webContentTypes[ext]; ok {
		return ct
	}
	return "application/octet-stream"
}
