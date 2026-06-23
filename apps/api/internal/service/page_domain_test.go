package service

import (
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/dns"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeDomain(t *testing.T) {
	assert.Equal(t, "app.example.com", normalizeDomain(" App.Example.com. "))
	assert.Equal(t, "", normalizeDomain("  "))
}

func TestRecordExistsForDomain(t *testing.T) {
	records := []dns.Record{
		{Name: "app.example.com", Type: "A", Values: []string{"1.2.3.4"}},
		{Name: "other.example.com", Type: "CNAME", Values: []string{"target"}},
	}
	assert.True(t, recordExistsForDomain(records, "app.example.com"))
	assert.True(t, recordExistsForDomain(records, "APP.example.com."))
	assert.False(t, recordExistsForDomain(records, "missing.example.com"))
}

func TestCloudFrontDomainFromURL(t *testing.T) {
	assert.Equal(t, "d123.cloudfront.net", cloudFrontDomainFromURL("https://d123.cloudfront.net"))
	assert.Equal(t, "d123.cloudfront.net", cloudFrontDomainFromURL("http://d123.cloudfront.net/"))
}
