package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
)

// ecrRegistry implements RegistryProvider for AWS Elastic Container Registry.
// ECR auth tokens are short-lived (~12h) and must be refreshed periodically.
type ecrRegistry struct{}

func newECRRegistry() *ecrRegistry { return &ecrRegistry{} }

func (p *ecrRegistry) Name() string { return "ecr" }

func (p *ecrRegistry) ShortLived() bool { return true }

type ecrConfig struct {
	Region    string `json:"region"`
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

func (p *ecrRegistry) DockerAuth(ctx context.Context, cfg json.RawMessage) (host, username, password string, err error) {
	var c ecrConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return "", "", "", fmt.Errorf("invalid ecr config: %w", err)
	}
	return ecrAuthToken(ctx, c.Region, c.AccessKey, c.SecretKey)
}

func (p *ecrRegistry) TestConnection(ctx context.Context, cfg json.RawMessage) (bool, string, error) {
	var c ecrConfig
	if err := json.Unmarshal(cfg, &c); err != nil {
		return false, "invalid config", nil
	}
	tctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	host, _, _, err := ecrAuthToken(tctx, c.Region, c.AccessKey, c.SecretKey)
	if err != nil {
		return false, "ecr authentication failed: " + err.Error(), nil
	}
	return true, "authenticated to " + host, nil
}

// ecrAuthToken fetches a short-lived ECR authorization token using static IAM
// credentials. It returns the registry host, username ("AWS") and password.
func ecrAuthToken(ctx context.Context, region, accessKey, secretKey string) (host, username, password string, err error) {
	if region == "" {
		return "", "", "", fmt.Errorf("ecr region is required")
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
		),
	)
	if err != nil {
		return "", "", "", fmt.Errorf("load aws config: %w", err)
	}

	client := ecr.NewFromConfig(cfg)
	out, err := client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return "", "", "", fmt.Errorf("ecr get authorization token: %w", err)
	}
	if len(out.AuthorizationData) == 0 || out.AuthorizationData[0].AuthorizationToken == nil {
		return "", "", "", fmt.Errorf("ecr returned no authorization data")
	}

	data := out.AuthorizationData[0]
	decoded, err := base64.StdEncoding.DecodeString(*data.AuthorizationToken)
	if err != nil {
		return "", "", "", fmt.Errorf("decode ecr token: %w", err)
	}
	user, pass, ok := strings.Cut(string(decoded), ":")
	if !ok {
		return "", "", "", fmt.Errorf("malformed ecr token")
	}

	host = ""
	if data.ProxyEndpoint != nil {
		host = stripScheme(*data.ProxyEndpoint)
	}
	return host, user, pass, nil
}

func init() {
	registerRegistry(func(d factoryDeps) RegistryProvider {
		return newECRRegistry()
	})
}
