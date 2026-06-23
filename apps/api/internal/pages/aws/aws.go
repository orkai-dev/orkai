// Package aws implements the pages.PagesProvider interface on AWS CloudFront +
// S3. One Page maps to one private S3 bucket (origin) fronted by one CloudFront
// distribution using an Origin Access Control (OAC) — the bucket itself stays
// fully private.
package aws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go"

	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
)

// cloudFrontRegion is the single region CloudFront's control plane lives in. All
// CloudFront (and ACM, later) API calls must target us-east-1.
const cloudFrontRegion = "us-east-1"

// cachingOptimizedPolicyID is the AWS-managed "CachingOptimized" cache policy.
const cachingOptimizedPolicyID = "658327ea-f89d-4fab-a63d-7e88639e58f6"

// Provider implements pages.PagesProvider for AWS.
type Provider struct{}

// New returns a new AWS pages provider.
func New() *Provider { return &Provider{} }

// Name implements pages.PagesProvider.
func (p *Provider) Name() string { return string(model.PageProviderAWSCloudFront) }

func (p *Provider) regionOf(page *model.Page) string {
	if page.Region != "" {
		return page.Region
	}
	return cloudFrontRegion
}

// s3Client builds an S3 client in the Page's bucket region.
func (p *Provider) s3Client(ctx context.Context, creds pages.Credentials, region string) (*s3.Client, error) {
	cfg, err := loadConfig(ctx, creds, region)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg), nil
}

// cfClient builds a CloudFront client (always us-east-1).
func (p *Provider) cfClient(ctx context.Context, creds pages.Credentials) (*cloudfront.Client, error) {
	cfg, err := loadConfig(ctx, creds, cloudFrontRegion)
	if err != nil {
		return nil, err
	}
	return cloudfront.NewFromConfig(cfg), nil
}

func loadConfig(ctx context.Context, creds pages.Credentials, region string) (awssdk.Config, error) {
	return creds.AWSConfig(ctx, region)
}

// TestConnection validates credentials via STS GetCallerIdentity, which requires
// no special IAM permissions.
func (p *Provider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	creds, err := pages.ParseCredentials(cfg)
	if err != nil {
		return false, err.Error(), nil
	}
	return p.testConnection(ctx, creds)
}

func (p *Provider) testConnection(ctx context.Context, creds pages.Credentials) (bool, string, error) {
	if err := creds.Validate(); err != nil {
		return false, err.Error(), nil
	}
	region := creds.DefaultRegion
	if region == "" {
		region = cloudFrontRegion
	}
	cfg, err := loadConfig(ctx, creds, region)
	if err != nil {
		return false, "failed to build AWS config: " + err.Error(), nil
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := sts.NewFromConfig(cfg).GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return false, "AWS credentials rejected: " + awsErrMessage(err), nil
	}
	return true, fmt.Sprintf("Connected to AWS account %s", awssdk.ToString(out.Account)), nil
}

// Provision creates (or reuses) the S3 bucket, OAC, and CloudFront distribution
// for the Page. It is idempotent: anything already recorded in page.Runtime is
// reused, and runtime is persisted incrementally via save so a mid-provision
// crash resumes instead of duplicating resources.
func (p *Provider) Provision(ctx context.Context, page *model.Page, cfg json.RawMessage, tags map[string]string, save pages.SaveRuntime) (*model.PageRuntime, error) {
	creds, err := pages.ParseCredentials(cfg)
	if err != nil {
		return nil, err
	}
	return p.provision(ctx, page, creds, tags, save)
}

func (p *Provider) provision(ctx context.Context, page *model.Page, creds pages.Credentials, tags map[string]string, save pages.SaveRuntime) (*model.PageRuntime, error) {
	region := p.regionOf(page)
	rt := page.Runtime
	if rt == nil {
		rt = &model.PageRuntime{}
	}

	s3c, err := p.s3Client(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("build S3 client: %w", err)
	}
	cfc, err := p.cfClient(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("build CloudFront client: %w", err)
	}

	// 1. S3 bucket (private, public access blocked).
	if rt.BucketName == "" {
		bucket := bucketName(page.Name)
		if err := p.createBucket(ctx, s3c, bucket, region); err != nil {
			return nil, err
		}
		rt.BucketName = bucket
		// Tag only on first creation; PutBucketTagging replaces the entire tag set.
		if len(tags) > 0 {
			if err := p.putBucketTagging(ctx, s3c, rt.BucketName, tags); err != nil {
				return nil, err
			}
		}
		if err := save(ctx, rt); err != nil {
			return nil, fmt.Errorf("persist bucket runtime: %w", err)
		}
	}
	if err := p.blockPublicAccess(ctx, s3c, rt.BucketName); err != nil {
		return nil, err
	}

	// 2. Origin Access Control.
	if rt.OACID == "" {
		oacID, err := p.createOAC(ctx, cfc, rt.BucketName)
		if err != nil {
			return nil, err
		}
		rt.OACID = oacID
		if err := save(ctx, rt); err != nil {
			return nil, fmt.Errorf("persist OAC runtime: %w", err)
		}
	}

	// 3. CloudFront distribution.
	if rt.DistributionID == "" {
		distID, distARN, domain, err := p.createDistribution(ctx, cfc, page, rt, region, tags)
		if err != nil {
			return nil, err
		}
		rt.DistributionID = distID
		rt.DistributionARN = distARN
		rt.DefaultURL = "https://" + domain
		if err := save(ctx, rt); err != nil {
			return nil, fmt.Errorf("persist distribution runtime: %w", err)
		}
	}

	// Wait outside the create block so a retry after a waiter timeout (DistributionID
	// already persisted) still blocks until Deployed before we apply the bucket
	// policy and return success. Already-deployed distributions pass on the first poll.
	if rt.DistributionID != "" {
		w := cloudfront.NewDistributionDeployedWaiter(cfc)
		if err := w.Wait(ctx, &cloudfront.GetDistributionInput{
			Id: awssdk.String(rt.DistributionID),
		}, 30*time.Minute); err != nil {
			return nil, fmt.Errorf("wait for distribution deployed: %s", awsErrMessage(err))
		}
	}

	// 4. Bucket policy granting the distribution (via OAC) read access. Applied
	// on every provision (PutBucketPolicy is idempotent) rather than only when
	// the distribution is first created: otherwise a transient failure here —
	// after DistributionID is persisted — would be skipped forever on retry,
	// leaving CloudFront serving 403s with no recovery short of editing the DB.
	if rt.DistributionARN != "" {
		if err := p.putBucketPolicy(ctx, s3c, rt.BucketName, rt.DistributionARN); err != nil {
			return nil, err
		}
	}

	return rt, nil
}

// Deploy mirrors filesDir to the bucket root (with delete of removed objects)
// and invalidates the distribution.
func (p *Provider) Deploy(ctx context.Context, page *model.Page, cfg json.RawMessage, filesDir string, onLog func(string)) (*pages.DeployResult, error) {
	creds, err := pages.ParseCredentials(cfg)
	if err != nil {
		return nil, err
	}
	return p.deploy(ctx, page, creds, filesDir, onLog)
}

func (p *Provider) deploy(ctx context.Context, page *model.Page, creds pages.Credentials, filesDir string, onLog func(string)) (*pages.DeployResult, error) {
	if page.Runtime == nil || page.Runtime.BucketName == "" || page.Runtime.DistributionID == "" {
		return nil, fmt.Errorf("page not provisioned (missing bucket or distribution)")
	}
	region := p.regionOf(page)
	bucket := page.Runtime.BucketName

	s3c, err := p.s3Client(ctx, creds, region)
	if err != nil {
		return nil, fmt.Errorf("build S3 client: %w", err)
	}

	local, err := pages.CollectFiles(filesDir)
	if err != nil {
		return nil, err
	}
	onLog(fmt.Sprintf("Found %d file(s) under publish folder", len(local)))

	// Upload every local file.
	uploaded := 0
	for key, fullPath := range local {
		if err := p.uploadObject(ctx, s3c, bucket, key, fullPath); err != nil {
			return nil, fmt.Errorf("upload %s: %s", key, awsErrMessage(err))
		}
		uploaded++
	}
	onLog(fmt.Sprintf("Uploaded %d file(s) to s3://%s", uploaded, bucket))

	// Delete remote objects no longer present locally (--delete mirror).
	remote, err := p.listObjects(ctx, s3c, bucket)
	if err != nil {
		return nil, fmt.Errorf("list bucket: %s", awsErrMessage(err))
	}
	var stale []string
	for _, key := range remote {
		if _, ok := local[key]; !ok {
			stale = append(stale, key)
		}
	}
	deleted, err := p.deleteObjects(ctx, s3c, bucket, stale)
	if err != nil {
		return nil, fmt.Errorf("delete stale objects: %s", awsErrMessage(err))
	}
	if deleted > 0 {
		onLog(fmt.Sprintf("Removed %d stale object(s)", deleted))
	}

	// Invalidate the CDN cache.
	cfc, err := p.cfClient(ctx, creds)
	if err != nil {
		return nil, fmt.Errorf("build CloudFront client: %w", err)
	}
	invID, err := p.invalidate(ctx, cfc, page.Runtime.DistributionID)
	if err != nil {
		return nil, fmt.Errorf("create invalidation: %s", awsErrMessage(err))
	}
	onLog("CloudFront invalidation created: " + invID)

	return &pages.DeployResult{
		ProviderRef:   invID,
		DefaultURL:    page.Runtime.DefaultURL,
		UploadedCount: uploaded,
		DeletedCount:  deleted,
	}, nil
}

// Delete tears down the bucket and distribution (best-effort). CloudFront
// requires a distribution to be disabled and fully deployed before it can be
// deleted (~15 min), so full distribution deletion is left to Phase 4; here we
// empty + delete the bucket (the main orphaned-cost) and disable the
// distribution so it stops serving.
func (p *Provider) Delete(ctx context.Context, page *model.Page, cfg json.RawMessage) error {
	creds, err := pages.ParseCredentials(cfg)
	if err != nil {
		return err
	}
	return p.delete(ctx, page, creds)
}

func (p *Provider) delete(ctx context.Context, page *model.Page, creds pages.Credentials) error {
	if page.Runtime == nil {
		return nil
	}
	region := p.regionOf(page)
	var errs []string

	if page.Runtime.BucketName != "" {
		if s3c, err := p.s3Client(ctx, creds, region); err == nil {
			if err := p.emptyAndDeleteBucket(ctx, s3c, page.Runtime.BucketName); err != nil {
				errs = append(errs, "delete bucket: "+awsErrMessage(err))
			}
		} else {
			errs = append(errs, err.Error())
		}
	}

	if page.Runtime.DistributionID != "" {
		if cfc, err := p.cfClient(ctx, creds); err == nil {
			if err := p.disableDistribution(ctx, cfc, page.Runtime.DistributionID); err != nil {
				errs = append(errs, "disable distribution: "+awsErrMessage(err))
			}
		} else {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// --- S3 helpers ---

func (p *Provider) createBucket(ctx context.Context, c *s3.Client, bucket, region string) error {
	in := &s3.CreateBucketInput{Bucket: awssdk.String(bucket)}
	// us-east-1 must NOT set a LocationConstraint; every other region must.
	if region != "us-east-1" {
		in.CreateBucketConfiguration = &s3types.CreateBucketConfiguration{
			LocationConstraint: s3types.BucketLocationConstraint(region),
		}
	}
	_, err := c.CreateBucket(ctx, in)
	if err == nil {
		return nil
	}
	// Reusing a bucket we already own is fine (idempotent re-provision).
	var owned *s3types.BucketAlreadyOwnedByYou
	if errors.As(err, &owned) {
		return nil
	}
	var exists *s3types.BucketAlreadyExists
	if errors.As(err, &exists) {
		return fmt.Errorf("S3 bucket name %q is already taken globally — retry the deploy to generate a new name", bucket)
	}
	return fmt.Errorf("create bucket: %s", awsErrMessage(err))
}

func (p *Provider) blockPublicAccess(ctx context.Context, c *s3.Client, bucket string) error {
	_, err := c.PutPublicAccessBlock(ctx, &s3.PutPublicAccessBlockInput{
		Bucket: awssdk.String(bucket),
		PublicAccessBlockConfiguration: &s3types.PublicAccessBlockConfiguration{
			BlockPublicAcls:       awssdk.Bool(true),
			BlockPublicPolicy:     awssdk.Bool(true),
			IgnorePublicAcls:      awssdk.Bool(true),
			RestrictPublicBuckets: awssdk.Bool(true),
		},
	})
	if err != nil {
		return fmt.Errorf("block public access: %s", awsErrMessage(err))
	}
	return nil
}

func (p *Provider) putBucketPolicy(ctx context.Context, c *s3.Client, bucket, distARN string) error {
	policy := fmt.Sprintf(`{
  "Version": "2008-10-17",
  "Statement": [{
    "Sid": "AllowCloudFrontServicePrincipal",
    "Effect": "Allow",
    "Principal": {"Service": "cloudfront.amazonaws.com"},
    "Action": "s3:GetObject",
    "Resource": "arn:aws:s3:::%s/*",
    "Condition": {"StringEquals": {"AWS:SourceArn": "%s"}}
  }]
}`, bucket, distARN)
	_, err := c.PutBucketPolicy(ctx, &s3.PutBucketPolicyInput{
		Bucket: awssdk.String(bucket),
		Policy: awssdk.String(policy),
	})
	if err != nil {
		return fmt.Errorf("put bucket policy: %s", awsErrMessage(err))
	}
	return nil
}

func (p *Provider) uploadObject(ctx context.Context, c *s3.Client, bucket, key, fullPath string) error {
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = c.PutObject(ctx, &s3.PutObjectInput{
		Bucket:      awssdk.String(bucket),
		Key:         awssdk.String(key),
		Body:        f,
		ContentType: awssdk.String(pages.ContentType(key)),
	})
	return err
}

func (p *Provider) listObjects(ctx context.Context, c *s3.Client, bucket string) ([]string, error) {
	var keys []string
	pager := s3.NewListObjectsV2Paginator(c, &s3.ListObjectsV2Input{Bucket: awssdk.String(bucket)})
	for pager.HasMorePages() {
		out, err := pager.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, obj := range out.Contents {
			keys = append(keys, awssdk.ToString(obj.Key))
		}
	}
	return keys, nil
}

func (p *Provider) deleteObjects(ctx context.Context, c *s3.Client, bucket string, keys []string) (int, error) {
	deleted := 0
	for start := 0; start < len(keys); start += 1000 {
		end := start + 1000
		if end > len(keys) {
			end = len(keys)
		}
		objs := make([]s3types.ObjectIdentifier, 0, end-start)
		for _, k := range keys[start:end] {
			objs = append(objs, s3types.ObjectIdentifier{Key: awssdk.String(k)})
		}
		out, err := c.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: awssdk.String(bucket),
			Delete: &s3types.Delete{Objects: objs, Quiet: awssdk.Bool(true)},
		})
		if err != nil {
			return deleted, err
		}
		if len(out.Errors) > 0 {
			msgs := make([]string, 0, len(out.Errors))
			for _, e := range out.Errors {
				msgs = append(msgs, fmt.Sprintf("%s: %s", awssdk.ToString(e.Key), awssdk.ToString(e.Message)))
			}
			return deleted, fmt.Errorf("partial delete failure (%d object(s)): %s", len(out.Errors), strings.Join(msgs, "; "))
		}
		deleted += end - start
	}
	return deleted, nil
}

func (p *Provider) emptyAndDeleteBucket(ctx context.Context, c *s3.Client, bucket string) error {
	keys, err := p.listObjects(ctx, c, bucket)
	if err != nil {
		// A missing bucket means there's nothing to delete — treat as success.
		// Any other error (e.g. AccessDenied from revoked IAM permissions) must
		// propagate so the caller records it instead of silently orphaning the
		// bucket.
		var noBucket *s3types.NoSuchBucket
		if errors.As(err, &noBucket) {
			return nil
		}
		return fmt.Errorf("list objects: %s", awsErrMessage(err))
	}
	if _, err := p.deleteObjects(ctx, c, bucket, keys); err != nil {
		return err
	}
	_, err = c.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: awssdk.String(bucket)})
	return err
}

// --- CloudFront helpers ---

func (p *Provider) createOAC(ctx context.Context, c *cloudfront.Client, bucket string) (string, error) {
	out, err := c.CreateOriginAccessControl(ctx, &cloudfront.CreateOriginAccessControlInput{
		OriginAccessControlConfig: &cftypes.OriginAccessControlConfig{
			Name:                          awssdk.String("orkai-oac-" + bucket),
			OriginAccessControlOriginType: cftypes.OriginAccessControlOriginTypesS3,
			SigningBehavior:               cftypes.OriginAccessControlSigningBehaviorsAlways,
			SigningProtocol:               cftypes.OriginAccessControlSigningProtocolsSigv4,
		},
	})
	if err != nil {
		return "", fmt.Errorf("create OAC: %s", awsErrMessage(err))
	}
	return awssdk.ToString(out.OriginAccessControl.Id), nil
}

func (p *Provider) putBucketTagging(ctx context.Context, c *s3.Client, bucket string, tags map[string]string) error {
	items := s3TagSet(tags)
	if len(items) == 0 {
		return nil
	}
	_, err := c.PutBucketTagging(ctx, &s3.PutBucketTaggingInput{
		Bucket: awssdk.String(bucket),
		Tagging: &s3types.Tagging{
			TagSet: items,
		},
	})
	if err != nil {
		return fmt.Errorf("put bucket tagging: %s", awsErrMessage(err))
	}
	return nil
}

func s3TagSet(tags map[string]string) []s3types.Tag {
	items := make([]s3types.Tag, 0, len(tags))
	for k, v := range tags {
		items = append(items, s3types.Tag{
			Key:   awssdk.String(k),
			Value: awssdk.String(v),
		})
	}
	return items
}

func cfTagSet(tags map[string]string) []cftypes.Tag {
	items := make([]cftypes.Tag, 0, len(tags))
	for k, v := range tags {
		items = append(items, cftypes.Tag{
			Key:   awssdk.String(k),
			Value: awssdk.String(v),
		})
	}
	return items
}

func (p *Provider) createDistribution(ctx context.Context, c *cloudfront.Client, page *model.Page, rt *model.PageRuntime, region string, tags map[string]string) (id, arn, domain string, err error) {
	originDomain := fmt.Sprintf("%s.s3.%s.amazonaws.com", rt.BucketName, region)
	const originID = "s3-origin"
	callerRef := page.ID.String() + "-" + randomHex(4)

	cfg := &cftypes.DistributionConfig{
		CallerReference:   awssdk.String(callerRef),
		Comment:           awssdk.String("Orkai Page: " + page.Name),
		Enabled:           awssdk.Bool(true),
		DefaultRootObject: awssdk.String("index.html"),
		Origins: &cftypes.Origins{
			Quantity: awssdk.Int32(1),
			Items: []cftypes.Origin{{
				Id:                    awssdk.String(originID),
				DomainName:            awssdk.String(originDomain),
				OriginAccessControlId: awssdk.String(rt.OACID),
				S3OriginConfig: &cftypes.S3OriginConfig{
					OriginAccessIdentity: awssdk.String(""),
				},
			}},
		},
		DefaultCacheBehavior: &cftypes.DefaultCacheBehavior{
			TargetOriginId:       awssdk.String(originID),
			ViewerProtocolPolicy: cftypes.ViewerProtocolPolicyRedirectToHttps,
			Compress:             awssdk.Bool(true),
			CachePolicyId:        awssdk.String(cachingOptimizedPolicyID),
			AllowedMethods: &cftypes.AllowedMethods{
				Quantity: awssdk.Int32(2),
				Items:    []cftypes.Method{cftypes.MethodGet, cftypes.MethodHead},
				CachedMethods: &cftypes.CachedMethods{
					Quantity: awssdk.Int32(2),
					Items:    []cftypes.Method{cftypes.MethodGet, cftypes.MethodHead},
				},
			},
		},
	}

	customDomain := strings.TrimSpace(page.CustomDomain)
	if customDomain != "" && rt.CertificateARN != "" {
		cfg.Aliases = &cftypes.Aliases{
			Quantity: awssdk.Int32(1),
			Items:    []string{customDomain},
		}
		cfg.ViewerCertificate = &cftypes.ViewerCertificate{
			ACMCertificateArn:      awssdk.String(rt.CertificateARN),
			SSLSupportMethod:       cftypes.SSLSupportMethodSniOnly,
			MinimumProtocolVersion: cftypes.MinimumProtocolVersionTLSv122021,
		}
		// Enable IPv6 so the AAAA alias records created for the custom domain
		// resolve. CloudFront defaults IsIPV6Enabled to false via the API, which
		// would make AAAA queries return NXDOMAIN for IPv6-only clients.
		cfg.IsIPV6Enabled = awssdk.Bool(true)
	}

	if len(tags) > 0 {
		out, err := c.CreateDistributionWithTags(ctx, &cloudfront.CreateDistributionWithTagsInput{
			DistributionConfigWithTags: &cftypes.DistributionConfigWithTags{
				DistributionConfig: cfg,
				Tags:               cfTags(tags),
			},
		})
		if err != nil {
			var cnameExists *cftypes.CNAMEAlreadyExists
			if errors.As(err, &cnameExists) {
				return "", "", "", fmt.Errorf("domain %q is already used by another CloudFront distribution", customDomain)
			}
			return "", "", "", fmt.Errorf("create distribution: %s", awsErrMessage(err))
		}
		return awssdk.ToString(out.Distribution.Id),
			awssdk.ToString(out.Distribution.ARN),
			awssdk.ToString(out.Distribution.DomainName),
			nil
	}

	out, err := c.CreateDistribution(ctx, &cloudfront.CreateDistributionInput{
		DistributionConfig: cfg,
	})
	if err != nil {
		var cnameExists *cftypes.CNAMEAlreadyExists
		if errors.As(err, &cnameExists) {
			return "", "", "", fmt.Errorf("domain %q is already used by another CloudFront distribution", customDomain)
		}
		return "", "", "", fmt.Errorf("create distribution: %s", awsErrMessage(err))
	}
	return awssdk.ToString(out.Distribution.Id),
		awssdk.ToString(out.Distribution.ARN),
		awssdk.ToString(out.Distribution.DomainName),
		nil
}

func cfTags(tags map[string]string) *cftypes.Tags {
	if len(tags) == 0 {
		return nil
	}
	return &cftypes.Tags{Items: cfTagSet(tags)}
}

func (p *Provider) invalidate(ctx context.Context, c *cloudfront.Client, distID string) (string, error) {
	out, err := c.CreateInvalidation(ctx, &cloudfront.CreateInvalidationInput{
		DistributionId: awssdk.String(distID),
		InvalidationBatch: &cftypes.InvalidationBatch{
			CallerReference: awssdk.String(randomHex(8)),
			Paths: &cftypes.Paths{
				Quantity: awssdk.Int32(1),
				Items:    []string{"/*"},
			},
		},
	})
	if err != nil {
		return "", err
	}
	return awssdk.ToString(out.Invalidation.Id), nil
}

func (p *Provider) disableDistribution(ctx context.Context, c *cloudfront.Client, distID string) error {
	cur, err := c.GetDistributionConfig(ctx, &cloudfront.GetDistributionConfigInput{Id: awssdk.String(distID)})
	if err != nil {
		// Already gone (e.g. deleted out-of-band) — nothing to disable, not a
		// failure. Without this, teardown logs a misleading "disable
		// distribution failed" for a distribution that no longer exists.
		var noDist *cftypes.NoSuchDistribution
		if errors.As(err, &noDist) {
			return nil
		}
		return err
	}
	if !awssdk.ToBool(cur.DistributionConfig.Enabled) {
		return nil // already disabled
	}
	cur.DistributionConfig.Enabled = awssdk.Bool(false)
	_, err = c.UpdateDistribution(ctx, &cloudfront.UpdateDistributionInput{
		Id:                 awssdk.String(distID),
		IfMatch:            cur.ETag,
		DistributionConfig: cur.DistributionConfig,
	})
	return err
}

// --- naming + errors ---

var nonAlnum = regexp.MustCompile(`[^a-z0-9-]+`)

// bucketName builds a globally-unique, DNS-compliant S3 bucket name. S3 bucket
// names are unique across ALL AWS accounts, so a random suffix is required.
func bucketName(pageName string) string {
	base := strings.ToLower(pageName)
	base = nonAlnum.ReplaceAllString(base, "-")
	base = strings.Trim(base, "-")
	if base == "" {
		base = "page"
	}
	// Leave room for the "orkai-" prefix (8) and "-" + 12 hex suffix (13).
	if len(base) > 40 {
		base = strings.Trim(base[:40], "-")
	}
	return fmt.Sprintf("orkai-%s-%s", base, randomHex(6))
}

func randomHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// collectFiles walks dir and returns a map of S3 key (forward-slash relative
// path) -> absolute file path. Directories and symlinks are skipped.
func collectFiles(dir string) (map[string]string, error) { return pages.CollectFiles(dir) }

// awsErrMessage extracts a clear, actionable message from an AWS SDK error,
// surfacing the API error code (e.g. AccessDenied) when present.
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
