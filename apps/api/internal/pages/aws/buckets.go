package aws

import (
	"context"
	"fmt"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/orkai-dev/orkai/apps/api/internal/pages"
)

// ListBuckets returns the names of all S3 buckets the cloud-account credentials
// can see. Used to let an operator pick a backup-target bucket from an existing
// AWS account instead of re-entering keys and a bucket name by hand.
func (p *Provider) ListBuckets(ctx context.Context, creds pages.Credentials) ([]string, error) {
	if err := creds.Validate(); err != nil {
		return nil, err
	}
	region := creds.DefaultRegion
	if region == "" {
		region = cloudFrontRegion
	}
	client, err := p.s3Client(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("build s3 client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("list buckets: %s", awsErrMessage(err))
	}
	names := make([]string, 0, len(out.Buckets))
	for _, b := range out.Buckets {
		names = append(names, awssdk.ToString(b.Name))
	}
	return names, nil
}

// BucketRegion resolves the AWS region a bucket lives in via GetBucketLocation.
// S3 returns an empty LocationConstraint for us-east-1, which we normalize.
func (p *Provider) BucketRegion(ctx context.Context, creds pages.Credentials, bucket string) (string, error) {
	if bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}
	region := creds.DefaultRegion
	if region == "" {
		region = cloudFrontRegion
	}
	client, err := p.s3Client(ctx, creds, region)
	if err != nil {
		return "", fmt.Errorf("build s3 client: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	out, err := client.GetBucketLocation(ctx, &s3.GetBucketLocationInput{Bucket: awssdk.String(bucket)})
	if err != nil {
		return "", fmt.Errorf("get bucket location: %s", awsErrMessage(err))
	}
	loc := string(out.LocationConstraint)
	if loc == "" {
		// us-east-1 returns an empty constraint.
		return "us-east-1", nil
	}
	return loc, nil
}
