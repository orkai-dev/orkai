package dns

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	r53types "github.com/aws/aws-sdk-go-v2/service/route53/types"

	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func awsConfig(t *testing.T, creds pages.Credentials) json.RawMessage {
	t.Helper()
	cfg, err := json.Marshal(creds)
	require.NoError(t, err)
	return cfg
}

func TestNewRoute53(t *testing.T) {
	p := NewRoute53()
	require.NotNil(t, p)
}

func TestValidateRecord(t *testing.T) {
	tests := []struct {
		name    string
		rec     Record
		wantErr string
	}{
		{
			name: "valid A",
			rec:  Record{Name: "app.example.com", Type: "A", TTL: 300, Values: []string{"203.0.113.5"}},
		},
		{
			name:    "missing name",
			rec:     Record{Type: "A", Values: []string{"1.2.3.4"}},
			wantErr: "name is required",
		},
		{
			name:    "unsupported type",
			rec:     Record{Name: "x.example.com", Type: "MX", Values: []string{"mail"}},
			wantErr: "unsupported record type",
		},
		{
			name:    "empty values",
			rec:     Record{Name: "x.example.com", Type: "A", Values: []string{}},
			wantErr: "at least one value",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRecord(tc.rec)
			if tc.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.wantErr)
			}
		})
	}
}

func TestListZonesMissingCredentials(t *testing.T) {
	p := NewRoute53()
	_, err := p.ListZones(context.Background(), json.RawMessage(`{}`))
	require.ErrorContains(t, err, "access key ID and secret access key are required")
}

func TestListRecordsEmptyZoneID(t *testing.T) {
	p := NewRoute53()
	_, err := p.ListRecords(context.Background(), awsConfig(t, pages.Credentials{
		AccessKeyID:     "AKIA",
		SecretAccessKey: "secret",
	}), "")
	require.ErrorContains(t, err, "zone_id is required")
}

func TestListZonesInvalidCredentials(t *testing.T) {
	p := NewRoute53()
	_, err := p.ListZones(context.Background(), awsConfig(t, pages.Credentials{
		AccessKeyID:     "AKIAINVALID",
		SecretAccessKey: "not-a-real-secret",
		DefaultRegion:   "us-east-1",
	}))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "list hosted zones")
}

func TestNormalizeZoneID(t *testing.T) {
	assert.Equal(t, "/hostedzone/Z123", normalizeZoneID("Z123"))
	assert.Equal(t, "/hostedzone/Z123", normalizeZoneID("/hostedzone/Z123"))
}

func TestFQDN(t *testing.T) {
	assert.Equal(t, "app.example.com.", fqdn("app.example.com"))
	assert.Equal(t, "app.example.com.", fqdn("app.example.com."))
}

func TestFromResourceRecordSetAlias(t *testing.T) {
	rec, ok := fromResourceRecordSet(r53types.ResourceRecordSet{
		Name: awssdk.String("example.com."),
		Type: r53types.RRTypeA,
		AliasTarget: &r53types.AliasTarget{
			HostedZoneId:         awssdk.String(CloudFrontHostedZoneID),
			DNSName:              awssdk.String("d123.cloudfront.net."),
			EvaluateTargetHealth: false,
		},
	})
	require.True(t, ok)
	require.NotNil(t, rec.Alias)
	assert.Equal(t, "example.com", rec.Name)
	assert.Equal(t, CloudFrontHostedZoneID, rec.Alias.TargetZoneID)
	assert.Equal(t, "d123.cloudfront.net", rec.Alias.TargetDNSName)
}

func TestToResourceRecordSetAlias(t *testing.T) {
	rrs, err := toResourceRecordSet(Record{
		Name: "example.com",
		Type: "A",
		Alias: &Alias{
			TargetZoneID:         CloudFrontHostedZoneID,
			TargetDNSName:        "d123.cloudfront.net",
			EvaluateTargetHealth: false,
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "example.com.", awssdk.ToString(rrs.Name))
	require.NotNil(t, rrs.AliasTarget)
	assert.Equal(t, "d123.cloudfront.net.", awssdk.ToString(rrs.AliasTarget.DNSName))
}

func TestValidateRecordAlias(t *testing.T) {
	err := ValidateRecord(Record{
		Name: "example.com",
		Type: "A",
		Alias: &Alias{
			TargetZoneID:  CloudFrontHostedZoneID,
			TargetDNSName: "d123.cloudfront.net",
		},
	})
	require.NoError(t, err)

	err = ValidateRecord(Record{
		Name:   "example.com",
		Type:   "CNAME",
		Alias:  &Alias{TargetZoneID: CloudFrontHostedZoneID, TargetDNSName: "d123.cloudfront.net"},
		Values: []string{"x"},
	})
	require.ErrorContains(t, err, "alias records must be type A or AAAA")
}

func TestFromResourceRecordSetA(t *testing.T) {
	rec, ok := fromResourceRecordSet(r53types.ResourceRecordSet{
		Name: awssdk.String("app.example.com."),
		Type: r53types.RRTypeA,
		TTL:  awssdk.Int64(300),
		ResourceRecords: []r53types.ResourceRecord{
			{Value: awssdk.String("1.2.3.4")},
		},
	})
	require.True(t, ok)
	assert.Equal(t, "app.example.com", rec.Name)
	assert.Equal(t, "A", rec.Type)
	assert.Equal(t, []string{"1.2.3.4"}, rec.Values)
}

func TestToResourceRecordSet(t *testing.T) {
	rrs, err := toResourceRecordSet(Record{
		Name:   "panel.example.com",
		Type:   "A",
		TTL:    600,
		Values: []string{"203.0.113.5"},
	})
	require.NoError(t, err)
	assert.Equal(t, "panel.example.com.", awssdk.ToString(rrs.Name))
	assert.Equal(t, r53types.RRTypeA, rrs.Type)
	assert.Equal(t, int64(600), awssdk.ToInt64(rrs.TTL))
}

func TestToResourceRecordSetTXT(t *testing.T) {
	rrs, err := toResourceRecordSet(Record{
		Name:   "_dmarc.example.com",
		Type:   "TXT",
		TTL:    300,
		Values: []string{"v=DMARC1; p=none"},
	})
	require.NoError(t, err)
	assert.Equal(t, `"v=DMARC1; p=none"`, awssdk.ToString(rrs.ResourceRecords[0].Value))

	rrs, err = toResourceRecordSet(Record{
		Name:   "example.com",
		Type:   "TXT",
		TTL:    300,
		Values: []string{`"v=spf1 include:amazonses.com ~all"`},
	})
	require.NoError(t, err)
	assert.Equal(t, `"v=spf1 include:amazonses.com ~all"`, awssdk.ToString(rrs.ResourceRecords[0].Value))

	longValue := strings.Repeat("k", 300)
	rrs, err = toResourceRecordSet(Record{
		Name:   "_domainkey.example.com",
		Type:   "TXT",
		TTL:    300,
		Values: []string{longValue},
	})
	require.NoError(t, err)
	assert.Equal(t, `"`+strings.Repeat("k", 255)+`" "`+strings.Repeat("k", 45)+`"`, awssdk.ToString(rrs.ResourceRecords[0].Value))
}

func TestFromResourceRecordSetTXT(t *testing.T) {
	rec, ok := fromResourceRecordSet(r53types.ResourceRecordSet{
		Name: awssdk.String("_acme-challenge.example.com."),
		Type: r53types.RRTypeTxt,
		TTL:  awssdk.Int64(300),
		ResourceRecords: []r53types.ResourceRecord{
			{Value: awssdk.String(`"verification-token"`)},
		},
	})
	require.True(t, ok)
	assert.Equal(t, "verification-token", rec.Values[0])
}

func TestQuoteTXTValue(t *testing.T) {
	assert.Equal(t, `"v=spf1 ~all"`, quoteTXTValue("v=spf1 ~all"))
	assert.Equal(t, `"v=spf1 ~all"`, quoteTXTValue(`"v=spf1 ~all"`))
	assert.Equal(t, `"say \"hello\""`, quoteTXTValue(`say "hello"`))

	longValue := strings.Repeat("a", 300)
	quoted := quoteTXTValue(longValue)
	assert.Equal(t, `"`+strings.Repeat("a", 255)+`" "`+strings.Repeat("a", 45)+`"`, quoted)
	assert.Equal(t, longValue, unquoteTXTValue(quoted))
}

func TestUnquoteTXTValue(t *testing.T) {
	assert.Equal(t, "v=spf1 ~all", unquoteTXTValue(`"v=spf1 ~all"`))
	assert.Equal(t, `say "hello"`, unquoteTXTValue(`"say \"hello\""`))
	assert.Equal(t, "bare", unquoteTXTValue("bare"))
	assert.Equal(t, "chunk1chunk2", unquoteTXTValue(`"chunk1" "chunk2"`))
	assert.Equal(t, strings.Repeat("a", 300), unquoteTXTValue(
		`"`+strings.Repeat("a", 255)+`" "`+strings.Repeat("a", 45)+`"`,
	))
}

func TestFromResourceRecordSetMultiPartTXT(t *testing.T) {
	longValue := strings.Repeat("x", 300)
	rec, ok := fromResourceRecordSet(r53types.ResourceRecordSet{
		Name: awssdk.String("_domainkey.example.com."),
		Type: r53types.RRTypeTxt,
		TTL:  awssdk.Int64(300),
		ResourceRecords: []r53types.ResourceRecord{
			{Value: awssdk.String(`"` + strings.Repeat("x", 255) + `" "` + strings.Repeat("x", 45) + `"`)},
		},
	})
	require.True(t, ok)
	assert.Equal(t, longValue, rec.Values[0])
}
