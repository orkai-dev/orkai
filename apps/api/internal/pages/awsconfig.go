package pages

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// assumeRoleSessionName identifies orka'i sessions in CloudTrail when assuming a role.
const assumeRoleSessionName = "orkai"

// AWSConfig builds an aws.Config for the given region honouring the credential
// auth mode:
//   - access_key: static AccessKeyID/SecretAccessKey.
//   - instance_role: the AWS default chain (env vars + EC2 instance profile).
//   - assume_role: assume RoleARN via STS, using the static keys as the base
//     credential when provided, otherwise the default chain.
func (c Credentials) AWSConfig(ctx context.Context, region string) (awssdk.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	// Static keys are the base credentials for access-key mode, and an optional
	// base for assume-role mode. instance_role mode omits them so the default
	// chain resolves environment variables and the EC2 instance's IAM role.
	if c.UseStaticKeys() {
		if !c.HasStaticBaseKeys() {
			return awssdk.Config{}, fmt.Errorf("access key ID and secret access key are required for access-key mode")
		}
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, ""),
		))
	} else if c.UseAssumeRole() && c.HasStaticBaseKeys() {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(c.AccessKeyID, c.SecretAccessKey, ""),
		))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awssdk.Config{}, err
	}

	if c.UseAssumeRole() {
		stsClient := sts.NewFromConfig(cfg)
		provider := stscreds.NewAssumeRoleProvider(stsClient, c.RoleARN, func(o *stscreds.AssumeRoleOptions) {
			o.RoleSessionName = assumeRoleSessionName
			if c.ExternalID != "" {
				o.ExternalID = awssdk.String(c.ExternalID)
			}
		})
		cfg.Credentials = awssdk.NewCredentialsCache(provider)
	}

	return cfg, nil
}

// ResolveCredentials resolves concrete credentials for the configured auth mode.
// For instance_role/assume_role this triggers an IMDS/STS call and returns
// short-lived keys plus a session token; for static access keys it returns those
// keys with an empty token. It is used to inject usable credentials into
// environments that cannot resolve a role themselves (e.g. an in-cluster
// aws-cli backup Job).
func (c Credentials) ResolveCredentials(ctx context.Context, region string) (accessKeyID, secretAccessKey, sessionToken string, err error) {
	cfg, err := c.AWSConfig(ctx, region)
	if err != nil {
		return "", "", "", err
	}
	creds, err := cfg.Credentials.Retrieve(ctx)
	if err != nil {
		return "", "", "", err
	}
	return creds.AccessKeyID, creds.SecretAccessKey, creds.SessionToken, nil
}
