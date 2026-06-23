package dns

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
)

// Cloudflare implements Provider for Cloudflare DNS.
type Cloudflare struct {
	client *cloudflare.Client
}

// NewCloudflare returns a Cloudflare DNS provider.
func NewCloudflare() *Cloudflare {
	return &Cloudflare{client: cloudflare.NewClient()}
}

func parseCloudflareCredentials(cfg json.RawMessage) (cloudflare.Credentials, error) {
	var creds cloudflare.Credentials
	if err := json.Unmarshal(cfg, &creds); err != nil {
		return creds, fmt.Errorf("invalid config")
	}
	if err := creds.Validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

func (p *Cloudflare) ListZones(ctx context.Context, cfg json.RawMessage) ([]Zone, error) {
	creds, err := parseCloudflareCredentials(cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	zones, err := p.client.ListZones(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("list zones: %w", err)
	}
	out := make([]Zone, 0, len(zones))
	for _, z := range zones {
		out = append(out, Zone{
			ID:      z.ID,
			Name:    strings.TrimSuffix(z.Name, "."),
			Private: false,
		})
	}
	return out, nil
}

func (p *Cloudflare) ListRecords(ctx context.Context, cfg json.RawMessage, zoneID string) ([]Record, error) {
	creds, err := parseCloudflareCredentials(cfg)
	if err != nil {
		return nil, err
	}
	if zoneID = strings.TrimSpace(zoneID); zoneID == "" {
		return nil, fmt.Errorf("zone_id is required")
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	records, err := p.client.ListDNSRecords(ctx, creds, zoneID)
	if err != nil {
		return nil, fmt.Errorf("list records: %w", err)
	}
	out := make([]Record, 0, len(records))
	for _, rec := range records {
		mapped, ok := fromCloudflareRecord(rec)
		if !ok {
			continue
		}
		out = append(out, mapped)
	}
	return out, nil
}

func (p *Cloudflare) UpsertRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error {
	if err := ValidateRecord(rec); err != nil {
		return err
	}
	if rec.Alias != nil {
		return fmt.Errorf("alias records are not supported for Cloudflare (use CNAME instead)")
	}
	creds, err := parseCloudflareCredentials(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cfRec, err := toCloudflareRecord(rec)
	if err != nil {
		return err
	}
	if err := p.client.UpsertDNSRecord(ctx, creds, zoneID, cfRec); err != nil {
		return fmt.Errorf("upsert record: %w", err)
	}
	return nil
}

func (p *Cloudflare) DeleteRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error {
	if err := ValidateRecord(rec); err != nil {
		return err
	}
	if rec.Alias != nil {
		return fmt.Errorf("alias records are not supported for Cloudflare (use CNAME instead)")
	}
	creds, err := parseCloudflareCredentials(cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cfRec, err := toCloudflareRecord(rec)
	if err != nil {
		return err
	}
	if err := p.client.DeleteDNSRecord(ctx, creds, zoneID, cfRec); err != nil {
		return fmt.Errorf("delete record: %w", err)
	}
	return nil
}

func fromCloudflareRecord(rec cloudflare.DNSRecord) (Record, bool) {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	switch typ {
	case "A", "AAAA", "CNAME", "TXT":
	default:
		return Record{}, false
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 300
	}
	return Record{
		Name:   strings.TrimSuffix(rec.Name, "."),
		Type:   typ,
		TTL:    ttl,
		Values: []string{rec.Content},
	}, true
}

func toCloudflareRecord(rec Record) (cloudflare.DNSRecord, error) {
	typ := strings.ToUpper(strings.TrimSpace(rec.Type))
	if len(rec.Values) == 0 {
		return cloudflare.DNSRecord{}, fmt.Errorf("at least one value is required")
	}
	// Cloudflare stores each value as a separate DNS record, so a single
	// upsert/delete maps to exactly one value. Reject multi-value records rather
	// than silently dropping the extras.
	if len(rec.Values) > 1 {
		return cloudflare.DNSRecord{}, fmt.Errorf("Cloudflare records support a single value per record; create separate records for each value")
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 300
	}
	return cloudflare.DNSRecord{
		Name:    strings.TrimSpace(rec.Name),
		Type:    typ,
		Content: strings.TrimSpace(rec.Values[0]),
		TTL:     ttl,
	}, nil
}
