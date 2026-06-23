package dns

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"
	"github.com/aws/smithy-go"

	"github.com/orkai-dev/orkai/apps/api/internal/pages"
)

const defaultRegion = "us-east-1"

// Route53 implements Provider for AWS Route53.
type Route53 struct{}

// NewRoute53 returns a Route53 DNS provider.
func NewRoute53() *Route53 { return &Route53{} }

func parseAWSCredentials(cfg json.RawMessage) (pages.Credentials, error) {
	var creds pages.Credentials
	if err := json.Unmarshal(cfg, &creds); err != nil {
		return creds, fmt.Errorf("invalid config")
	}
	if err := creds.Validate(); err != nil {
		return creds, err
	}
	return creds, nil
}

func (p *Route53) client(ctx context.Context, cfg json.RawMessage) (*route53.Client, error) {
	creds, err := parseAWSCredentials(cfg)
	if err != nil {
		return nil, err
	}
	region := creds.DefaultRegion
	if region == "" {
		region = defaultRegion
	}
	awsCfg, err := loadConfig(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return route53.NewFromConfig(awsCfg), nil
}

func loadConfig(ctx context.Context, creds pages.Credentials, region string) (awssdk.Config, error) {
	return creds.AWSConfig(ctx, region)
}

func (p *Route53) ListZones(ctx context.Context, cfg json.RawMessage) ([]Zone, error) {
	c, err := p.client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var zones []Zone
	pager := route53.NewListHostedZonesPaginator(c, &route53.ListHostedZonesInput{})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list hosted zones: %s", awsErrMessage(err))
		}
		for _, hz := range out.HostedZones {
			name := strings.TrimSuffix(awssdk.ToString(hz.Name), ".")
			zones = append(zones, Zone{
				ID:      strings.TrimPrefix(awssdk.ToString(hz.Id), "/hostedzone/"),
				Name:    name,
				Private: hz.Config != nil && hz.Config.PrivateZone,
			})
		}
	}
	return zones, nil
}

func (p *Route53) ListRecords(ctx context.Context, cfg json.RawMessage, zoneID string) ([]Record, error) {
	if zoneID = strings.TrimSpace(zoneID); zoneID == "" {
		return nil, fmt.Errorf("zone_id is required")
	}
	c, err := p.client(ctx, cfg)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	hostedZoneID := normalizeZoneID(zoneID)
	var records []Record
	pager := route53.NewListResourceRecordSetsPaginator(c, &route53.ListResourceRecordSetsInput{
		HostedZoneId: awssdk.String(hostedZoneID),
	})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list records: %s", awsErrMessage(err))
		}
		for _, rr := range out.ResourceRecordSets {
			rec, ok := fromResourceRecordSet(rr)
			if !ok {
				continue
			}
			records = append(records, rec)
		}
	}
	return records, nil
}

func (p *Route53) UpsertRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error {
	if err := ValidateRecord(rec); err != nil {
		return err
	}
	return p.changeRecord(ctx, cfg, zoneID, rec, r53types.ChangeActionUpsert)
}

func (p *Route53) DeleteRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record) error {
	if err := ValidateRecord(rec); err != nil {
		return err
	}
	return p.changeRecord(ctx, cfg, zoneID, rec, r53types.ChangeActionDelete)
}

func (p *Route53) changeRecord(ctx context.Context, cfg json.RawMessage, zoneID string, rec Record, action r53types.ChangeAction) error {
	if zoneID = strings.TrimSpace(zoneID); zoneID == "" {
		return fmt.Errorf("zone_id is required")
	}
	c, err := p.client(ctx, cfg)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	rrs, err := toResourceRecordSet(rec)
	if err != nil {
		return err
	}
	_, err = c.ChangeResourceRecordSets(ctx, &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: awssdk.String(normalizeZoneID(zoneID)),
		ChangeBatch: &r53types.ChangeBatch{
			Changes: []r53types.Change{{
				Action:            action,
				ResourceRecordSet: rrs,
			}},
		},
	})
	if err != nil {
		return fmt.Errorf("change record: %s", awsErrMessage(err))
	}
	return nil
}

func normalizeZoneID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "/hostedzone/") {
		return id
	}
	return "/hostedzone/" + id
}

func fqdn(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".")
	if name == "" {
		return ""
	}
	return name + "."
}

func fromResourceRecordSet(rr r53types.ResourceRecordSet) (Record, bool) {
	typ := string(rr.Type)
	switch typ {
	case "A", "AAAA", "CNAME", "TXT":
	default:
		return Record{}, false
	}
	name := strings.TrimSuffix(awssdk.ToString(rr.Name), ".")
	if rr.AliasTarget != nil {
		if typ != "A" && typ != "AAAA" {
			return Record{}, false
		}
		return Record{
			Name: name,
			Type: typ,
			Alias: &Alias{
				TargetZoneID:         awssdk.ToString(rr.AliasTarget.HostedZoneId),
				TargetDNSName:        strings.TrimSuffix(awssdk.ToString(rr.AliasTarget.DNSName), "."),
				EvaluateTargetHealth: rr.AliasTarget.EvaluateTargetHealth,
			},
		}, true
	}
	if len(rr.ResourceRecords) == 0 {
		return Record{}, false
	}
	values := make([]string, 0, len(rr.ResourceRecords))
	for _, v := range rr.ResourceRecords {
		val := awssdk.ToString(v.Value)
		if typ == "TXT" {
			val = unquoteTXTValue(val)
		}
		values = append(values, val)
	}
	ttl := int64(300)
	if rr.TTL != nil {
		ttl = *rr.TTL
	}
	return Record{
		Name:   name,
		Type:   typ,
		TTL:    ttl,
		Values: values,
	}, true
}

func toResourceRecordSet(rec Record) (*r53types.ResourceRecordSet, error) {
	rec.Type = strings.ToUpper(strings.TrimSpace(rec.Type))
	name := fqdn(rec.Name)
	if name == "" {
		return nil, fmt.Errorf("record name is required")
	}
	if rec.Alias != nil {
		target := fqdn(rec.Alias.TargetDNSName)
		if target == "" {
			return nil, fmt.Errorf("alias target DNS name is required")
		}
		return &r53types.ResourceRecordSet{
			Name: awssdk.String(name),
			Type: r53types.RRType(rec.Type),
			AliasTarget: &r53types.AliasTarget{
				HostedZoneId:         awssdk.String(rec.Alias.TargetZoneID),
				DNSName:              awssdk.String(target),
				EvaluateTargetHealth: rec.Alias.EvaluateTargetHealth,
			},
		}, nil
	}
	ttl := rec.TTL
	if ttl <= 0 {
		ttl = 300
	}
	rrs := make([]r53types.ResourceRecord, 0, len(rec.Values))
	for _, v := range rec.Values {
		val := strings.TrimSpace(v)
		if rec.Type == "TXT" {
			val = quoteTXTValue(val)
		}
		rrs = append(rrs, r53types.ResourceRecord{Value: awssdk.String(val)})
	}
	return &r53types.ResourceRecordSet{
		Name:            awssdk.String(name),
		Type:            r53types.RRType(rec.Type),
		TTL:             awssdk.Int64(ttl),
		ResourceRecords: rrs,
	}, nil
}

// quoteTXTValue wraps a TXT record value in double quotes as required by Route53.
// Values longer than 255 bytes are split into multiple space-separated quoted chunks
// to comply with the DNS TXT record string length limit (RFC 1035 §3.3.14).
func quoteTXTValue(v string) string {
	const maxChunk = 255
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v
	}
	escaped := strings.ReplaceAll(v, `"`, `\"`)
	if len(escaped) <= maxChunk {
		return `"` + escaped + `"`
	}
	var sb strings.Builder
	for len(escaped) > 0 {
		chunk := escaped
		if len(chunk) > maxChunk {
			chunk = escaped[:maxChunk]
		}
		escaped = escaped[len(chunk):]
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteByte('"')
		sb.WriteString(chunk)
		sb.WriteByte('"')
	}
	return sb.String()
}

// unquoteTXTValue strips Route53 TXT quoting for API responses.
// Long TXT values are stored as multiple space-separated quoted chunks; each chunk
// is parsed and concatenated to recover the original value.
func unquoteTXTValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v[0] != '"' {
		return v
	}

	var parts []string
	for i := 0; i < len(v); {
		for i < len(v) && v[i] == ' ' {
			i++
		}
		if i >= len(v) {
			break
		}
		if v[i] != '"' {
			return v
		}
		part, next, ok := parseTXTQuotedSegment(v, i)
		if !ok {
			return v
		}
		parts = append(parts, part)
		i = next
	}
	return strings.Join(parts, "")
}

func parseTXTQuotedSegment(s string, start int) (inner string, next int, ok bool) {
	if start >= len(s) || s[start] != '"' {
		return "", start, false
	}
	var b strings.Builder
	for i := start + 1; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			b.WriteByte(s[i+1])
			i++
			continue
		}
		if s[i] == '"' {
			return b.String(), i + 1, true
		}
		b.WriteByte(s[i])
	}
	return "", start, false
}

func awsErrMessage(err error) string {
	if err == nil {
		return ""
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		return fmt.Sprintf("%s: %s", apiErr.ErrorCode(), apiErr.ErrorMessage())
	}
	return err.Error()
}
