package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
)

const awsCLIImage = "amazon/aws-cli:latest"

// awsS3Provider implements ObjectStorageProvider for S3-compatible endpoints.
type awsS3Provider struct{}

func newAWSS3Provider() *awsS3Provider { return &awsS3Provider{} }

func init() {
	registerObjectStorage(func(d factoryDeps) ObjectStorageProvider {
		return newAWSS3Provider()
	})
}

func (p *awsS3Provider) Name() string { return "aws_s3" }

func (p *awsS3Provider) Bucket(cfg json.RawMessage) (string, error) {
	c, err := parseS3Config(cfg)
	if err != nil {
		return "", err
	}
	if c.Bucket == "" {
		return "", fmt.Errorf("bucket is required")
	}
	return c.Bucket, nil
}

func parseS3Config(cfg json.RawMessage) (orchestrator.S3Config, error) {
	var c orchestrator.S3Config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return orchestrator.S3Config{}, fmt.Errorf("invalid config")
	}
	return c, nil
}

func s3Region(region string) string {
	if region != "" {
		return region
	}
	return "auto"
}

func (p *awsS3Provider) newClient(ctx context.Context, cfg json.RawMessage) (*s3.Client, orchestrator.S3Config, error) {
	c, err := parseS3Config(cfg)
	if err != nil {
		return nil, orchestrator.S3Config{}, err
	}
	if c.Endpoint == "" {
		return nil, orchestrator.S3Config{}, fmt.Errorf("endpoint is required")
	}
	if c.Bucket == "" {
		return nil, orchestrator.S3Config{}, fmt.Errorf("bucket is required")
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(s3Region(c.Region)),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(c.AccessKey, c.SecretKey, c.SessionToken)),
	)
	if err != nil {
		return nil, orchestrator.S3Config{}, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = &c.Endpoint
		o.UsePathStyle = true
	})
	return client, c, nil
}

func (p *awsS3Provider) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	client, c, err := p.newClient(ctx, cfg)
	if err != nil {
		return false, err.Error(), nil
	}

	_, err = client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  &c.Bucket,
		MaxKeys: int32Ptr(1),
	})
	if err != nil {
		return false, "S3 connection failed: " + err.Error(), nil
	}
	return true, "S3 bucket accessible", nil
}

func (p *awsS3Provider) Upload(ctx context.Context, cfg json.RawMessage, localPath, key string) error {
	client, c, err := p.newClient(ctx, cfg)
	if err != nil {
		return err
	}

	f, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open upload file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: &c.Bucket,
		Key:    &key,
		Body:   f,
	}); err != nil {
		return fmt.Errorf("s3 upload: %w", err)
	}
	return nil
}

func (p *awsS3Provider) Download(ctx context.Context, cfg json.RawMessage, key, localPath string) error {
	client, c, err := p.newClient(ctx, cfg)
	if err != nil {
		return err
	}

	f, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("create download file: %w", err)
	}
	defer func() { _ = f.Close() }()

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &c.Bucket,
		Key:    &key,
	})
	if err != nil {
		return fmt.Errorf("s3 download: %w", err)
	}
	defer func() { _ = out.Body.Close() }()
	if _, err := io.Copy(f, out.Body); err != nil {
		return fmt.Errorf("s3 download: %w", err)
	}
	return nil
}

func (p *awsS3Provider) List(ctx context.Context, cfg json.RawMessage, prefix string) ([]ObjectInfo, error) {
	client, c, err := p.newClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}

	var objects []ObjectInfo
	paginator := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{
		Bucket: &c.Bucket,
		Prefix: stringPtr(prefix),
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("s3 list: %w", err)
		}
		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}
			key := *obj.Key
			info := ObjectInfo{
				Key:      key,
				FileName: filepath.Base(key),
			}
			if obj.Size != nil {
				info.SizeBytes = *obj.Size
			}
			if obj.LastModified != nil {
				info.LastModified = obj.LastModified.UTC().Format("2006-01-02 15:04:05")
			}
			objects = append(objects, info)
		}
	}

	// Most recent first (matches legacy aws-cli ls sort order used by callers).
	for i, j := 0, len(objects)-1; i < j; i, j = i+1, j-1 {
		objects[i], objects[j] = objects[j], objects[i]
	}
	return objects, nil
}

func (p *awsS3Provider) Delete(ctx context.Context, cfg json.RawMessage, key string) error {
	client, c, err := p.newClient(ctx, cfg)
	if err != nil {
		return err
	}

	if _, err := client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &c.Bucket,
		Key:    &key,
	}); err != nil {
		return fmt.Errorf("s3 delete: %w", err)
	}
	return nil
}

func (p *awsS3Provider) jobEnv(cfg json.RawMessage) (map[string]string, error) {
	c, err := parseS3Config(cfg)
	if err != nil {
		return nil, err
	}
	env := map[string]string{
		"AWS_ACCESS_KEY_ID":     c.AccessKey,
		"AWS_SECRET_ACCESS_KEY": c.SecretKey,
		"AWS_DEFAULT_REGION":    s3Region(c.Region),
		"S3_ENDPOINT":           c.Endpoint,
		"S3_BUCKET":             c.Bucket,
	}
	// Temporary credentials (instance role / assumed role) require the session
	// token alongside the access keys for the in-cluster aws-cli Job to auth.
	if c.SessionToken != "" {
		env["AWS_SESSION_TOKEN"] = c.SessionToken
	}
	return env, nil
}

func (p *awsS3Provider) UploadJob(cfg json.RawMessage, srcPath, key string) (orchestrator.ObjectTransfer, error) {
	env, err := p.jobEnv(cfg)
	if err != nil {
		return orchestrator.ObjectTransfer{}, err
	}
	uploadCmd := fmt.Sprintf("aws s3 cp %s s3://$S3_BUCKET/%s --endpoint-url $S3_ENDPOINT", srcPath, key)
	return orchestrator.ObjectTransfer{
		Image:   awsCLIImage,
		Env:     env,
		Command: []string{"sh", "-c", uploadCmd},
	}, nil
}

func (p *awsS3Provider) DownloadJob(cfg json.RawMessage, key, destPath string) (orchestrator.ObjectTransfer, error) {
	env, err := p.jobEnv(cfg)
	if err != nil {
		return orchestrator.ObjectTransfer{}, err
	}
	downloadCmd := fmt.Sprintf("aws s3 cp s3://$S3_BUCKET/%s %s --endpoint-url $S3_ENDPOINT", key, destPath)
	return orchestrator.ObjectTransfer{
		Image:   awsCLIImage,
		Env:     env,
		Command: []string{"sh", "-c", downloadCmd},
	}, nil
}

func int32Ptr(v int32) *int32 { return &v }

func stringPtr(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

var _ ObjectStorageProvider = (*awsS3Provider)(nil)
