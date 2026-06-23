package providers

import (
	"encoding/json"
	"testing"

	"github.com/orkai-dev/orkai/apps/api/internal/orchestrator"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAWSS3Bucket(t *testing.T) {
	p := newAWSS3Provider()
	cfg, err := json.Marshal(orchestrator.S3Config{Bucket: "my-bucket", Endpoint: "https://s3.example.com"})
	require.NoError(t, err)

	bucket, err := p.Bucket(cfg)
	require.NoError(t, err)
	assert.Equal(t, "my-bucket", bucket)
}

func TestAWSS3UploadJob(t *testing.T) {
	p := newAWSS3Provider()
	cfg, err := json.Marshal(orchestrator.S3Config{
		Endpoint:  "https://s3.example.com",
		Bucket:    "my-bucket",
		AccessKey: "AKIA",
		SecretKey: "secret",
		Region:    "us-east-1",
	})
	require.NoError(t, err)

	transfer, err := p.UploadJob(cfg, "/backup/abc.sql", "orkai/db-backups/abc.sql")
	require.NoError(t, err)
	assert.Equal(t, awsCLIImage, transfer.Image)
	assert.Equal(t, "AKIA", transfer.Env["AWS_ACCESS_KEY_ID"])
	assert.Equal(t, "my-bucket", transfer.Env["S3_BUCKET"])
	require.Len(t, transfer.Command, 3)
	assert.Contains(t, transfer.Command[2], "aws s3 cp /backup/abc.sql s3://$S3_BUCKET/orkai/db-backups/abc.sql")
}

func TestAWSS3DownloadJob(t *testing.T) {
	p := newAWSS3Provider()
	cfg, err := json.Marshal(orchestrator.S3Config{
		Endpoint:  "https://s3.example.com",
		Bucket:    "my-bucket",
		AccessKey: "AKIA",
		SecretKey: "secret",
	})
	require.NoError(t, err)

	transfer, err := p.DownloadJob(cfg, "orkai/db-backups/abc.sql", "/backup/abc.sql")
	require.NoError(t, err)
	assert.Equal(t, awsCLIImage, transfer.Image)
	require.Len(t, transfer.Command, 3)
	assert.Contains(t, transfer.Command[2], "aws s3 cp s3://$S3_BUCKET/orkai/db-backups/abc.sql /backup/abc.sql")
}

func TestAWSS3TestConnectionInvalidConfig(t *testing.T) {
	p := newAWSS3Provider()
	ok, msg, err := p.TestConnection(t.Context(), json.RawMessage(`{"bucket":"x"}`))
	require.NoError(t, err)
	assert.False(t, ok)
	assert.Contains(t, msg, "endpoint")
}
