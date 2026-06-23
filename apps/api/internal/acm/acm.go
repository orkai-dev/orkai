// Package acm wraps AWS Certificate Manager for CloudFront custom domains.
// ACM certificates used by CloudFront must be in us-east-1.
package acm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/acm"
	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	"github.com/aws/smithy-go"

	"github.com/orkai-dev/orkai/apps/api/internal/pages"
)

// CloudFrontRegion is the only region ACM certificates for CloudFront may live in.
const CloudFrontRegion = "us-east-1"

// Client wraps ACM operations for CloudFront certificates.
type Client struct{}

// New returns an ACM client wrapper.
func New() *Client { return &Client{} }

func (c *Client) acmClient(ctx context.Context, creds pages.Credentials) (*acm.Client, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	cfg, err := creds.AWSConfig(ctx, CloudFrontRegion)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return acm.NewFromConfig(cfg), nil
}

// RequestCertificate requests a DNS-validated ACM certificate for domain.
// idempotencyToken should be stable per page so retries don't duplicate certs.
func (c *Client) RequestCertificate(ctx context.Context, creds pages.Credentials, domain, idempotencyToken string, tags map[string]string) (string, error) {
	client, err := c.acmClient(ctx, creds)
	if err != nil {
		return "", err
	}
	// ACM includes DomainName as the primary SAN automatically; passing it again
	// in SubjectAlternativeNames would be redundant.
	in := &acm.RequestCertificateInput{
		DomainName:       awssdk.String(domain),
		ValidationMethod: acmtypes.ValidationMethodDns,
		IdempotencyToken: awssdk.String(idempotencyToken),
	}
	if len(tags) > 0 {
		in.Tags = acmTags(tags)
	}
	out, err := client.RequestCertificate(ctx, in)
	if err != nil {
		return "", fmt.Errorf("request certificate: %s", awsErrMessage(err))
	}
	return awssdk.ToString(out.CertificateArn), nil
}

// ValidationRecord returns the DNS CNAME ACM requires for validation.
// Polls DescribeCertificate until ResourceRecord is populated (usually seconds).
func (c *Client) ValidationRecord(ctx context.Context, creds pages.Credentials, certARN string) (name, typ, value string, err error) {
	client, err := c.acmClient(ctx, creds)
	if err != nil {
		return "", "", "", err
	}
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		name, typ, value, ok, derr := c.describeValidationRecord(ctx, client, certARN)
		if derr != nil {
			return "", "", "", derr
		}
		if ok {
			return name, typ, value, nil
		}
		select {
		case <-ctx.Done():
			return "", "", "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return "", "", "", fmt.Errorf("ACM validation record not ready yet — retry the deploy in a few seconds")
}

func (c *Client) describeValidationRecord(ctx context.Context, client *acm.Client, certARN string) (name, typ, value string, ok bool, err error) {
	out, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: awssdk.String(certARN),
	})
	if err != nil {
		return "", "", "", false, fmt.Errorf("describe certificate: %s", awsErrMessage(err))
	}
	if out.Certificate == nil {
		return "", "", "", false, fmt.Errorf("describe certificate: empty response")
	}
	for _, opt := range out.Certificate.DomainValidationOptions {
		if opt.ResourceRecord == nil {
			continue
		}
		return strings.TrimSuffix(awssdk.ToString(opt.ResourceRecord.Name), "."),
			string(opt.ResourceRecord.Type),
			strings.TrimSuffix(awssdk.ToString(opt.ResourceRecord.Value), "."),
			true, nil
	}
	return "", "", "", false, nil
}

// CertificateStatus returns the ACM certificate status (e.g. PENDING_VALIDATION, ISSUED).
func (c *Client) CertificateStatus(ctx context.Context, creds pages.Credentials, certARN string) (string, error) {
	client, err := c.acmClient(ctx, creds)
	if err != nil {
		return "", err
	}
	out, err := client.DescribeCertificate(ctx, &acm.DescribeCertificateInput{
		CertificateArn: awssdk.String(certARN),
	})
	if err != nil {
		return "", fmt.Errorf("describe certificate: %s", awsErrMessage(err))
	}
	if out.Certificate == nil {
		return "", fmt.Errorf("describe certificate: empty response")
	}
	return string(out.Certificate.Status), nil
}

// IsIssued reports whether the certificate has reached ISSUED status.
func (c *Client) IsIssued(ctx context.Context, creds pages.Credentials, certARN string) (bool, error) {
	status, err := c.CertificateStatus(ctx, creds, certARN)
	if err != nil {
		return false, err
	}
	return status == string(acmtypes.CertificateStatusIssued), nil
}

// WaitIssued blocks until the certificate is ISSUED or timeout elapses.
func (c *Client) WaitIssued(ctx context.Context, creds pages.Credentials, certARN string, timeout time.Duration) error {
	client, err := c.acmClient(ctx, creds)
	if err != nil {
		return err
	}
	w := acm.NewCertificateValidatedWaiter(client)
	if err := w.Wait(ctx, &acm.DescribeCertificateInput{
		CertificateArn: awssdk.String(certARN),
	}, timeout); err != nil {
		return fmt.Errorf("wait for certificate issued: %s", awsErrMessage(err))
	}
	return nil
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

func acmTags(tags map[string]string) []acmtypes.Tag {
	items := make([]acmtypes.Tag, 0, len(tags))
	for k, v := range tags {
		items = append(items, acmtypes.Tag{
			Key:   awssdk.String(k),
			Value: awssdk.String(v),
		})
	}
	return items
}
