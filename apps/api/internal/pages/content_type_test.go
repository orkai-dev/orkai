package pages

import "testing"

func TestContentType(t *testing.T) {
	cases := map[string]string{
		"index.html":       "text/html; charset=utf-8",
		"assets/app.js":    "text/javascript; charset=utf-8",
		"assets/app.mjs":   "text/javascript; charset=utf-8",
		"style.css":        "text/css; charset=utf-8",
		"data.json":        "application/json",
		"app.js.map":       "application/json",
		"logo.svg":         "image/svg+xml",
		"img/photo.JPG":    "image/jpeg", // case-insensitive
		"fonts/font.woff2": "font/woff2",
		"module.wasm":      "application/wasm",
		"favicon.ico":      "image/x-icon",
		"unknownfile.xyz":  "application/octet-stream",
		"noext":            "application/octet-stream",
		"archive.tar.gz":   "application/octet-stream",
	}
	for path, want := range cases {
		if got := ContentType(path); got != want {
			t.Errorf("ContentType(%q) = %q, want %q", path, got, want)
		}
	}
}
