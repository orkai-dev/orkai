// Package dns abstracts DNS providers (Route53, Cloudflare).
// Services never call cloud SDKs directly — they go through Provider.
package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CloudFrontHostedZoneID is the Route53 hosted zone ID for CloudFront alias targets.
const CloudFrontHostedZoneID = "Z2FDTNDATAQYW2"

// Zone is a DNS hosted zone.
type Zone struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Private bool   `json:"private"`
}

// Alias is a Route53 alias target (A/AAAA only).
type Alias struct {
	TargetZoneID         string `json:"target_zone_id"`
	TargetDNSName        string `json:"target_dns_name"`
	EvaluateTargetHealth bool   `json:"evaluate_target_health"`
}

// Record is a DNS resource record (A / AAAA / CNAME / TXT, or A/AAAA alias).
type Record struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	TTL    int64    `json:"ttl"`
	Values []string `json:"values"`
	Alias  *Alias   `json:"alias,omitempty"`
}

// Provider abstracts a DNS backend.
type Provider interface {
	ListZones(ctx context.Context, cfg json.RawMessage) ([]Zone, error)
	ListRecords(ctx context.Context, cfg json.RawMessage, zoneID string) ([]Record, error)
	UpsertRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error
	DeleteRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error
}

// ValidateRecord checks a record before upsert/delete. Normalization and TTL
// defaulting are the responsibility of the provider (see toResourceRecordSet),
// so this only validates a read-only copy.
func ValidateRecord(rec Record) error {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	name := strings.TrimSpace(rec.Name)
	if name == "" {
		return fmt.Errorf("record name is required")
	}
	switch typ {
	case "A", "AAAA", "CNAME", "TXT":
	default:
		return fmt.Errorf("unsupported record type %q (use A, AAAA, CNAME, or TXT)", typ)
	}
	if rec.Alias != nil {
		if typ != "A" && typ != "AAAA" {
			return fmt.Errorf("alias records must be type A or AAAA")
		}
		if len(rec.Values) > 0 {
			return fmt.Errorf("alias records cannot have values")
		}
		if strings.TrimSpace(rec.Alias.TargetDNSName) == "" {
			return fmt.Errorf("alias target DNS name is required")
		}
		if strings.TrimSpace(rec.Alias.TargetZoneID) == "" {
			return fmt.Errorf("alias target zone ID is required")
		}
		return nil
	}
	if len(rec.Values) == 0 {
		return fmt.Errorf("at least one value is required")
	}
	for _, v := range rec.Values {
		if strings.TrimSpace(v) == "" {
			return fmt.Errorf("record values cannot be empty")
		}
	}
	return nil
}

// For returns the DNS provider implementation for a cloud-account provider key.
func For(provider string) (Provider, error) {
	switch provider {
	case "", "aws", "route53":
		return NewRoute53(), nil
	case "cloudflare":
		return NewCloudflare(), nil
	default:
		return nil, fmt.Errorf("DNS management is not supported for provider %q", provider)
	}
}
