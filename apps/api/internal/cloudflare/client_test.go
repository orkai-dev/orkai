package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(server *httptest.Server) *Client {
	return &Client{http: server.Client(), baseURL: server.URL}
}

func writeResult(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	payload := map[string]any{"success": true, "errors": []any{}, "result": result}
	require.NoError(t, json.NewEncoder(w).Encode(payload))
}

// TestDeleteDNSRecord_SameNameTXT verifies that deleting a TXT record selects
// the entry whose content matches, even when multiple records share the same
// name and type. Previously findRecord returned the first match and delete
// failed with a misleading "content mismatch" error.
func TestDeleteDNSRecord_SameNameTXT(t *testing.T) {
	creds := Credentials{AuthMode: AuthAPIToken, APIToken: "token"}

	var deletedID string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			// Honor the content filter: only return the matching record.
			content := r.URL.Query().Get("content")
			records := []DNSRecord{
				{ID: "rec-aaa", Name: "_acme.example.com", Type: "TXT", Content: "token-aaa", TTL: 300},
				{ID: "rec-bbb", Name: "_acme.example.com", Type: "TXT", Content: "token-bbb", TTL: 300},
			}
			var filtered []DNSRecord
			for _, rec := range records {
				if content == "" || rec.Content == content {
					filtered = append(filtered, rec)
				}
			}
			writeResult(t, w, filtered)
		case http.MethodDelete:
			deletedID = r.URL.Path[len(r.URL.Path)-7:]
			writeResult(t, w, map[string]string{"id": deletedID})
		default:
			http.Error(w, "unexpected method", http.StatusMethodNotAllowed)
		}
	}))
	defer server.Close()

	c := testClient(server)
	err := c.DeleteDNSRecord(context.Background(), creds, "zone1", DNSRecord{
		Name:    "_acme.example.com",
		Type:    "TXT",
		Content: "token-bbb",
	})
	require.NoError(t, err)
	assert.Equal(t, "rec-bbb", deletedID)
}

// TestDeleteDNSRecord_NotFound confirms a clear not-found error (not a content
// mismatch) when no record matches the requested content.
func TestDeleteDNSRecord_NotFound(t *testing.T) {
	creds := Credentials{AuthMode: AuthAPIToken, APIToken: "token"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No records ever match the requested content.
		writeResult(t, w, []DNSRecord{})
	}))
	defer server.Close()

	c := testClient(server)
	err := c.DeleteDNSRecord(context.Background(), creds, "zone1", DNSRecord{
		Name:    "_acme.example.com",
		Type:    "TXT",
		Content: "token-missing",
	})
	require.ErrorContains(t, err, "not found")
}
