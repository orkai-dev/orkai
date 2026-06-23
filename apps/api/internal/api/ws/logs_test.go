package ws

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckSameOrigin(t *testing.T) {
	cases := []struct {
		name   string
		origin string
		host   string
		fwd    string
		want   bool
	}{
		{"no origin (non-browser) allowed", "", "example.com", "", true},
		{"same host allowed", "https://example.com", "example.com", "", true},
		{"same host case-insensitive", "https://EXAMPLE.com", "example.com", "", true},
		{"dev proxy preserves host", "http://localhost:3000", "localhost:3000", "", true},
		{"cross origin rejected", "https://evil.com", "example.com", "", false},
		{"forwarded host match allowed", "http://localhost:3000", "localhost:8080", "localhost:3000", true},
		{"forwarded host list first value", "http://localhost:3000", "localhost:8080", "localhost:3000, proxy", true},
		{"forwarded host mismatch rejected", "https://evil.com", "localhost:8080", "localhost:3000", false},
		{"malformed origin rejected", "://bad", "example.com", "", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "http://"+tc.host+"/ws/events", nil)
			req.Host = tc.host
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.fwd != "" {
				req.Header.Set("X-Forwarded-Host", tc.fwd)
			}
			assert.Equal(t, tc.want, checkSameOrigin(req))
		})
	}
}
