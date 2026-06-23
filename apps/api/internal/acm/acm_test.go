package acm

import (
	"context"
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	"github.com/stretchr/testify/require"
)

func TestNewClient(t *testing.T) {
	require.NotNil(t, New())
}

func TestRequestCertificateMissingCredentials(t *testing.T) {
	c := New()
	_, err := c.RequestCertificate(context.Background(), pages.Credentials{}, "app.example.com", "token", nil)
	require.ErrorContains(t, err, "access key ID and secret access key are required")
}

func TestValidationRecordMissingCredentials(t *testing.T) {
	c := New()
	_, _, _, err := c.ValidationRecord(context.Background(), pages.Credentials{}, "arn:aws:acm:us-east-1:123:certificate/abc")
	require.ErrorContains(t, err, "access key ID and secret access key are required")
}

func TestIsIssuedMissingCredentials(t *testing.T) {
	c := New()
	_, err := c.IsIssued(context.Background(), pages.Credentials{}, "arn:aws:acm:us-east-1:123:certificate/abc")
	require.ErrorContains(t, err, "access key ID and secret access key are required")
}
