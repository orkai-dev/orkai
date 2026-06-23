package cloudflare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const baseURL = "https://api.cloudflare.com/client/v4"

// Client is a minimal Cloudflare API v4 HTTP client.
type Client struct {
	http    *http.Client
	baseURL string
}

// NewClient returns a Client with a sensible default timeout.
func NewClient() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}, baseURL: baseURL}
}

// NewTestClient returns a Client aimed at a test server.
func NewTestClient(httpClient *http.Client, testBaseURL string) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{http: httpClient, baseURL: testBaseURL}
}

type apiResponse struct {
	Success  bool              `json:"success"`
	Errors   []apiError        `json:"errors"`
	Messages []json.RawMessage `json:"messages"`
	Result   json.RawMessage   `json:"result"`
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Zone is a Cloudflare DNS zone.
type Zone struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// DNSRecord is a Cloudflare DNS record.
type DNSRecord struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Content string `json:"content"`
	TTL     int64  `json:"ttl"`
	Proxied *bool  `json:"proxied,omitempty"`
}

// TestConnection validates credentials by listing zones (or verifying the token).
func TestConnection(ctx context.Context, creds Credentials) (bool, string, error) {
	if err := creds.Validate(); err != nil {
		return false, err.Error(), nil
	}
	c := NewClient()
	if creds.UseAPIToken() {
		var out struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := c.do(ctx, creds, http.MethodGet, "/user/tokens/verify", nil, &out); err != nil {
			return false, err.Error(), nil
		}
		if out.Status != "active" {
			return false, fmt.Sprintf("token status is %q", out.Status), nil
		}
		return true, "Cloudflare API token verified", nil
	}
	zones, err := c.ListZones(ctx, creds)
	if err != nil {
		return false, err.Error(), nil
	}
	return true, fmt.Sprintf("Connected — %d zone(s) visible", len(zones)), nil
}

// ListZones returns DNS zones visible to the credentials.
func (c *Client) ListZones(ctx context.Context, creds Credentials) ([]Zone, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	var zones []Zone
	page := 1
	for {
		path := fmt.Sprintf("/zones?per_page=50&page=%d", page)
		if creds.AccountID != "" {
			path += "&account.id=" + url.QueryEscape(creds.AccountID)
		}
		var batch []Zone
		if err := c.do(ctx, creds, http.MethodGet, path, nil, &batch); err != nil {
			return nil, err
		}
		zones = append(zones, batch...)
		if len(batch) < 50 {
			break
		}
		page++
	}
	return zones, nil
}

// ListDNSRecords returns DNS records in a zone.
func (c *Client) ListDNSRecords(ctx context.Context, creds Credentials, zoneID string) ([]DNSRecord, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return nil, fmt.Errorf("zone_id is required")
	}
	var records []DNSRecord
	page := 1
	for {
		path := fmt.Sprintf("/zones/%s/dns_records?per_page=100&page=%d", zoneID, page)
		var batch []DNSRecord
		if err := c.do(ctx, creds, http.MethodGet, path, nil, &batch); err != nil {
			return nil, err
		}
		records = append(records, batch...)
		if len(batch) < 100 {
			break
		}
		page++
	}
	return records, nil
}

// UpsertDNSRecord creates or updates a DNS record by name+type.
func (c *Client) UpsertDNSRecord(ctx context.Context, creds Credentials, zoneID string, rec DNSRecord) error {
	if err := creds.Validate(); err != nil {
		return err
	}
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return fmt.Errorf("zone_id is required")
	}
	existing, err := c.findRecord(ctx, creds, zoneID, rec.Name, rec.Type, "")
	if err != nil {
		return err
	}
	body := map[string]any{
		"type":    strings.ToUpper(rec.Type),
		"name":    rec.Name,
		"content": rec.Content,
		"ttl":     rec.TTL,
	}
	if rec.Proxied != nil {
		body["proxied"] = *rec.Proxied
		if *rec.Proxied {
			// Cloudflare requires auto-TTL (1) for proxied records.
			body["ttl"] = int64(1)
		}
	}
	if existing != nil {
		return c.do(ctx, creds, http.MethodPut, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, existing.ID), body, nil)
	}
	return c.do(ctx, creds, http.MethodPost, fmt.Sprintf("/zones/%s/dns_records", zoneID), body, nil)
}

// DeleteDNSRecord removes a DNS record matching name, type, and content.
func (c *Client) DeleteDNSRecord(ctx context.Context, creds Credentials, zoneID string, rec DNSRecord) error {
	if err := creds.Validate(); err != nil {
		return err
	}
	zoneID = strings.TrimSpace(zoneID)
	if zoneID == "" {
		return fmt.Errorf("zone_id is required")
	}
	// Pass the content so the exact record is selected: multiple records can
	// share the same name+type (e.g. several TXT values), and matching by
	// name+type alone would delete (or mis-report) the wrong one.
	existing, err := c.findRecord(ctx, creds, zoneID, rec.Name, rec.Type, rec.Content)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("record %s %s not found", rec.Name, rec.Type)
	}
	return c.do(ctx, creds, http.MethodDelete, fmt.Sprintf("/zones/%s/dns_records/%s", zoneID, existing.ID), nil, nil)
}

// findRecord returns the first DNS record matching name+type. When content is
// non-empty, it narrows the match to that exact content, so a specific record
// can be selected even when several records share the same name+type (common
// for TXT records). Returns (nil, nil) when no record matches.
func (c *Client) findRecord(ctx context.Context, creds Credentials, zoneID, name, typ, content string) (*DNSRecord, error) {
	typ = strings.ToUpper(strings.TrimSpace(typ))
	name = strings.TrimSpace(name)
	content = strings.TrimSpace(content)
	path := fmt.Sprintf("/zones/%s/dns_records?name=%s&type=%s", zoneID, url.QueryEscape(name), url.QueryEscape(typ))
	if content != "" {
		path += "&content=" + url.QueryEscape(content)
	}
	var records []DNSRecord
	if err := c.do(ctx, creds, http.MethodGet, path, nil, &records); err != nil {
		return nil, err
	}
	if content != "" {
		// Don't rely solely on the server-side content filter: confirm an exact
		// match locally so we never delete a record whose content differs.
		for i := range records {
			if strings.EqualFold(strings.TrimSpace(records[i].Content), content) {
				return &records[i], nil
			}
		}
		return nil, nil
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func (c *Client) do(ctx context.Context, creds Credentials, method, path string, body any, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	}
	base := c.baseURL
	if base == "" {
		base = baseURL
	}
	req, err := http.NewRequestWithContext(ctx, method, base+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if creds.UseAPIToken() {
		req.Header.Set("Authorization", "Bearer "+creds.APIToken)
	} else {
		req.Header.Set("X-Auth-Email", creds.Email)
		req.Header.Set("X-Auth-Key", creds.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var envelope apiResponse
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("parse response: %w", err)
	}
	if !envelope.Success {
		if len(envelope.Errors) > 0 {
			return fmt.Errorf("%s", envelope.Errors[0].Message)
		}
		return fmt.Errorf("cloudflare API request failed")
	}
	if out != nil && len(envelope.Result) > 0 && string(envelope.Result) != "null" {
		if err := json.Unmarshal(envelope.Result, out); err != nil {
			return fmt.Errorf("parse result: %w", err)
		}
	}
	return nil
}
