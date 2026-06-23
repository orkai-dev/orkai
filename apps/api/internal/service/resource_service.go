package service

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"

	"github.com/orkai-dev/orkai/apps/api/internal/apierr"
	"github.com/orkai-dev/orkai/apps/api/internal/cloudflare"
	"github.com/orkai-dev/orkai/apps/api/internal/dns"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/pages"
	pagesaws "github.com/orkai-dev/orkai/apps/api/internal/pages/aws"
	"github.com/orkai-dev/orkai/apps/api/internal/providers"
	"github.com/orkai-dev/orkai/apps/api/internal/secret"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

type ResourceService struct {
	store     store.Store
	providers *providers.Registry
	logger    *slog.Logger
	notifSvc  *NotificationService
}

func NewResourceService(s store.Store, prov *providers.Registry, logger *slog.Logger, notifSvc *NotificationService) *ResourceService {
	return &ResourceService{store: s, providers: prov, logger: logger, notifSvc: notifSvc}
}

// GitRepo represents a repository from a git provider.
type GitRepo = providers.GitRepo

// DNSZone and DNSRecord are DNS provider types exposed to handlers.
type DNSZone = dns.Zone
type DNSRecord = dns.Record

type CreateResourceInput struct {
	Name     string             `json:"name"`
	Type     model.ResourceType `json:"type" binding:"required"`
	Provider string             `json:"provider"`
	Config   json.RawMessage    `json:"config"`
}

// isSecretConfigKey reports whether a shared-resource config field holds a
// credential that must never be returned to clients in plaintext.
func isSecretConfigKey(key string) bool {
	return secret.IsResourceConfigKey(key)
}

// RedactResourceConfig returns a copy of the resource with secret config fields
// replaced by the mask sentinel so responses never leak stored credentials.
func RedactResourceConfig(r model.SharedResource) model.SharedResource {
	if len(r.Config) == 0 {
		return r
	}
	var cfg map[string]any
	if err := json.Unmarshal(r.Config, &cfg); err != nil {
		// Unparseable config — drop it rather than risk leaking secrets.
		r.Config = json.RawMessage(`{}`)
		return r
	}
	changed := false
	for k, v := range cfg {
		if !isSecretConfigKey(k) {
			continue
		}
		if s, ok := v.(string); ok && s == "" {
			continue
		}
		cfg[k] = model.SettingSecretMask
		changed = true
	}
	if !changed {
		return r
	}
	if masked, err := json.Marshal(cfg); err == nil {
		r.Config = masked
	}
	return r
}

// mergeSecretConfig restores stored secret values for any secret field whose
// incoming value is still the mask sentinel, so editing a resource without
// re-entering its secrets does not wipe them.
func mergeSecretConfig(existing, incoming json.RawMessage) json.RawMessage {
	var incomingMap map[string]any
	if err := json.Unmarshal(incoming, &incomingMap); err != nil {
		return incoming
	}
	var existingMap map[string]any
	_ = json.Unmarshal(existing, &existingMap)

	changed := false
	for k, v := range incomingMap {
		if !isSecretConfigKey(k) {
			continue
		}
		if s, ok := v.(string); ok && s == model.SettingSecretMask {
			// Only restore when the key actually exists in the stored config;
			// otherwise drop it so we never persist a JSON null credential.
			if existingVal, exists := existingMap[k]; exists {
				incomingMap[k] = existingVal
			} else {
				delete(incomingMap, k)
			}
			changed = true
		}
	}
	if !changed {
		return incoming
	}
	if merged, err := json.Marshal(incomingMap); err == nil {
		return merged
	}
	return incoming
}

func (s *ResourceService) Create(ctx context.Context, orgID uuid.UUID, input CreateResourceInput) (*model.SharedResource, error) {
	if input.Type == model.ResourceObjectStorage && input.Provider == "aws_s3" {
		expanded, err := s.expandS3FromCloudAccount(ctx, orgID, input.Config)
		if err != nil {
			return nil, err
		}
		if expanded != nil {
			input.Config = expanded
		}
	}
	if input.Name == "" {
		input.Name = s.autoName(input)
	}

	resource := &model.SharedResource{
		OrgID:    orgID,
		Name:     input.Name,
		Type:     input.Type,
		Provider: input.Provider,
		Config:   input.Config,
		Status:   "active",
	}
	if err := s.store.SharedResources().Create(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

// GenerateSSHKey creates a new SSH key pair and stores it as a shared resource.
func (s *ResourceService) GenerateSSHKey(ctx context.Context, orgID uuid.UUID, algorithm string, name string) (*model.SharedResource, error) {
	var privateKeyPEM []byte
	var publicKeyStr string

	switch algorithm {
	case "ed25519":
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate ed25519 key: %w", err)
		}
		privPEM, err := ssh.MarshalPrivateKey(privKey, "")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(privPEM)
		sshPub, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create public key: %w", err)
		}
		publicKeyStr = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	case "rsa-4096":
		privKey, err := rsa.GenerateKey(rand.Reader, 4096)
		if err != nil {
			return nil, fmt.Errorf("failed to generate RSA key: %w", err)
		}
		privPEM, err := ssh.MarshalPrivateKey(privKey, "")
		if err != nil {
			return nil, fmt.Errorf("failed to marshal private key: %w", err)
		}
		privateKeyPEM = pem.EncodeToMemory(privPEM)
		sshPub, err := ssh.NewPublicKey(&privKey.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create public key: %w", err)
		}
		publicKeyStr = strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPub)))

	default:
		return nil, fmt.Errorf("unsupported algorithm: %s (use ed25519 or rsa-4096)", algorithm)
	}

	if name == "" {
		// Generate a unique short ID from the public key fingerprint
		shortID := make([]byte, 3)
		_, _ = rand.Read(shortID)
		name = fmt.Sprintf("key-%s-%x", time.Now().Format("0102"), shortID)
	}

	config, _ := json.Marshal(map[string]string{
		"private_key": string(privateKeyPEM),
		"public_key":  publicKeyStr,
		"algorithm":   algorithm,
	})

	resource := &model.SharedResource{
		OrgID:    orgID,
		Name:     name,
		Type:     model.ResourceSSHKey,
		Provider: algorithm,
		Config:   config,
		Status:   "active",
	}

	if err := s.store.SharedResources().Create(ctx, resource); err != nil {
		return nil, err
	}

	s.logger.Info("generated SSH key", slog.String("name", name), slog.String("algorithm", algorithm))
	return resource, nil
}

// shortHex returns a random hex suffix for unique naming.
func shortHex(n int) string {
	b := make([]byte, n)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// autoName generates a unique, descriptive name for a resource.
//
// Naming convention: {provider}-{identifier}-{MMDD}-{hex}
// Examples:
//
//	github-alan-0325-a1f3
//	dockerhub-myuser-0325-b2e4
//	key-ed25519-0325-c3d5
//	r2-my-bucket-0325-d4e6
func (s *ResourceService) autoName(input CreateResourceInput) string {
	dateSuffix := time.Now().Format("0102")
	hex := shortHex(2)
	provider := strings.ToLower(input.Provider)
	if provider == "" {
		provider = "custom"
	}

	switch input.Type {
	case model.ResourceGitProvider:
		var cfg struct {
			Username string `json:"username"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Username != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Username, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-git-%s-%s", provider, dateSuffix, hex)

	case model.ResourceRegistry:
		var cfg struct {
			Username string `json:"username"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Username != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Username, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-registry-%s-%s", provider, dateSuffix, hex)

	case model.ResourceSSHKey:
		return fmt.Sprintf("key-%s-%s-%s", provider, dateSuffix, hex)

	case model.ResourceCloudAccount:
		return fmt.Sprintf("%s-account-%s-%s", provider, dateSuffix, hex)

	case model.ResourceObjectStorage:
		var cfg struct {
			Bucket string `json:"bucket"`
		}
		if input.Config != nil {
			_ = json.Unmarshal(input.Config, &cfg)
		}
		if cfg.Bucket != "" {
			return fmt.Sprintf("%s-%s-%s-%s", provider, cfg.Bucket, dateSuffix, hex)
		}
		return fmt.Sprintf("%s-storage-%s-%s", provider, dateSuffix, hex)

	default:
		return fmt.Sprintf("resource-%s-%s", dateSuffix, hex)
	}
}

func (s *ResourceService) GetByID(ctx context.Context, id uuid.UUID) (*model.SharedResource, error) {
	return s.store.SharedResources().GetByID(ctx, id)
}

func (s *ResourceService) List(ctx context.Context, orgID uuid.UUID, resourceType string) ([]model.SharedResource, error) {
	return s.store.SharedResources().ListByOrg(ctx, orgID, resourceType)
}

type UpdateResourceInput struct {
	Name     *string          `json:"name"`
	Provider *string          `json:"provider"`
	Config   *json.RawMessage `json:"config"`
}

// getOwnedResource fetches a resource and verifies it belongs to the given org.
func (s *ResourceService) getOwnedResource(ctx context.Context, orgID, id uuid.UUID) (*model.SharedResource, error) {
	resource, err := s.store.SharedResources().GetByID(ctx, id)
	if err != nil {
		return nil, apierr.ErrNotFound.WithDetail("resource not found")
	}
	if resource.OrgID != orgID {
		return nil, apierr.ErrNotFound.WithDetail("resource not found")
	}
	return resource, nil
}

func (s *ResourceService) Update(ctx context.Context, orgID, id uuid.UUID, input UpdateResourceInput) (*model.SharedResource, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if input.Name != nil {
		resource.Name = *input.Name
	}
	if input.Provider != nil {
		resource.Provider = *input.Provider
	}
	if input.Config != nil {
		incoming := *input.Config
		// For S3 object storage backed by a cloud account, re-resolve the
		// account credentials so an edit (e.g. switching bucket) keeps a complete
		// config without the operator re-entering keys.
		if resource.Type == model.ResourceObjectStorage && resource.Provider == "aws_s3" {
			if expanded, err := s.expandS3FromCloudAccount(ctx, orgID, incoming); err != nil {
				return nil, err
			} else if expanded != nil {
				incoming = expanded
			}
		}
		resource.Config = mergeSecretConfig(resource.Config, incoming)
	}
	if err := s.store.SharedResources().Update(ctx, resource); err != nil {
		return nil, err
	}
	return resource, nil
}

func (s *ResourceService) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	// Verify ownership
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return err
	}

	// Each check below is a single indexed lookup keyed on this resource's ID
	// (backed by the FK indexes in migration 20260615013) rather than a global
	// scan of every app/node/database/page. All are fail-closed: a query error
	// aborts the delete so we never drop a credential a workload still needs.
	//
	// These lookups are intentionally not org-scoped (the DB/page guards used to
	// be, via a per-project loop). A reference is always by this resource's UUID
	// and resources can only be linked within their own org (enforced on write),
	// so an org filter would return identical results while adding a projects
	// join. Staying org-wide also fails safe: were a cross-org reference to ever
	// exist, we still refuse to delete a resource something depends on.

	// Applications referencing as git provider or registry.
	if app, err := s.store.Applications().FindByResource(ctx, id); err != nil {
		return fmt.Errorf("cannot verify resource references: %w", err)
	} else if app != nil {
		role := "registry"
		if app.GitProviderID != nil && *app.GitProviderID == id {
			role = "git provider"
		}
		return fmt.Errorf("resource is in use by application %q as %s", app.Name, role)
	}

	// Server nodes referencing as SSH key.
	if node, err := s.store.ServerNodes().FindBySSHKey(ctx, id); err != nil {
		return fmt.Errorf("cannot verify resource references: %w", err)
	} else if node != nil {
		return fmt.Errorf("resource is in use by node %q as SSH key", node.Name)
	}

	// Managed databases referencing as backup S3.
	if db, err := s.store.ManagedDatabases().FindByBackupS3(ctx, id); err != nil {
		return fmt.Errorf("cannot verify resource references: %w", err)
	} else if db != nil {
		return fmt.Errorf("resource is in use by database %q as backup storage", db.Name)
	}

	// Pages referencing as cloud account or git provider.
	if pg, err := s.store.Pages().FindByResource(ctx, id); err != nil {
		return fmt.Errorf("cannot verify resource references: %w", err)
	} else if pg != nil {
		role := "git provider"
		if pg.CloudAccountID != nil && *pg.CloudAccountID == id {
			role = "cloud account"
		}
		return fmt.Errorf("resource is in use by page %q as %s", pg.Name, role)
	}

	// Check system backup settings
	s3IDStr, settErr := s.store.Settings().Get(ctx, "system_backup_s3_id")
	if settErr == nil && s3IDStr == id.String() {
		return fmt.Errorf("resource is in use as system backup storage — change backup config first")
	}

	if err := s.store.SharedResources().Delete(ctx, id); err != nil {
		return err
	}

	s.notifSvc.NotifyResourceDeleted(orgID, model.EventResourceDeleted,
		resource.Name, fmt.Sprintf("Shared resource %q (%s) was deleted", resource.Name, resource.Type))
	return nil
}

// TestConnection validates the credentials for a shared resource.
func (s *ResourceService) TestConnection(ctx context.Context, orgID, id uuid.UUID) (bool, string, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return false, "", err
	}

	switch resource.Type {
	case model.ResourceGitProvider:
		gp, err := s.providers.Git(resource.Provider)
		if err != nil {
			return false, err.Error(), nil
		}
		return gp.TestConnection(ctx, resource.Config)
	case model.ResourceRegistry:
		return s.providers.Registry(resource.Provider).TestConnection(ctx, resource.Config)
	case model.ResourceSSHKey:
		return true, "SSH key stored", nil // SSH keys are validated on use
	case model.ResourceObjectStorage:
		return s.testObjectStorage(resource)
	case model.ResourceCloudAccount:
		return s.testCloudAccount(ctx, resource)
	default:
		return false, "unknown resource type", nil
	}
}

// testCloudAccount validates cloud-account credentials for the stored provider.
func (s *ResourceService) testCloudAccount(ctx context.Context, resource *model.SharedResource) (bool, string, error) {
	switch resource.Provider {
	case "aws", "":
		return pagesaws.New().TestConnection(ctx, resource.Config)
	case "cloudflare":
		var creds cloudflare.Credentials
		if err := json.Unmarshal(resource.Config, &creds); err != nil {
			return false, "invalid config", nil
		}
		return cloudflare.TestConnection(ctx, creds)
	default:
		return false, fmt.Sprintf("unsupported cloud account provider %q", resource.Provider), nil
	}
}

func (s *ResourceService) testObjectStorage(resource *model.SharedResource) (bool, string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cfg, err := resolveObjectStorageConfig(ctx, s.store, resource.OrgID, resource.Provider, resource.Config)
	if err != nil {
		return false, err.Error(), nil
	}
	return s.providers.ObjectStorage(resource.Provider).TestConnection(ctx, cfg)
}

// ListBuckets returns the S3 bucket names visible to a cloud_account resource's
// credentials, so an operator can pick a backup-target bucket instead of
// re-typing keys and a bucket name.
func (s *ResourceService) ListBuckets(ctx context.Context, orgID, id uuid.UUID) ([]string, error) {
	resource, creds, err := s.cloudAccountCredentials(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if resource.Provider != "aws" {
		return nil, apierr.ErrValidation.WithDetail("listing buckets is only supported for AWS cloud accounts")
	}
	buckets, err := pagesaws.New().ListBuckets(ctx, creds)
	if err != nil {
		// Surface the underlying AWS error (e.g. AccessDenied on
		// s3:ListAllMyBuckets, or missing instance-role credentials) as a 4xx so
		// the UI shows the cause instead of an opaque 500. The operator can still
		// type a known bucket name by hand when listing is denied.
		return nil, apierr.ErrBadRequest.WithDetail(err.Error())
	}
	return buckets, nil
}

// expandS3FromCloudAccount normalizes an aws_s3 object-storage config that
// references a cloud account, persisting only the reference plus the derived
// bucket/region/endpoint — never the account's credentials. The keys are
// resolved fresh at use time (see resolveObjectStorageConfig) so rotating the
// cloud account's IAM keys propagates to every derived resource automatically.
// It returns nil when the config does not reference a cloud account (manual
// entry), leaving the caller's config untouched.
func (s *ResourceService) expandS3FromCloudAccount(ctx context.Context, orgID uuid.UUID, cfg json.RawMessage) (json.RawMessage, error) {
	if len(cfg) == 0 {
		return nil, nil
	}
	var in struct {
		CloudAccountID string `json:"cloud_account_id"`
		Bucket         string `json:"bucket"`
		Region         string `json:"region"`
	}
	if err := json.Unmarshal(cfg, &in); err != nil {
		return nil, apierr.ErrValidation.WithDetail("invalid config")
	}
	if in.CloudAccountID == "" {
		return nil, nil
	}
	if in.Bucket == "" {
		return nil, apierr.ErrValidation.WithDetail("bucket is required")
	}
	accountID, err := uuid.Parse(in.CloudAccountID)
	if err != nil {
		return nil, apierr.ErrValidation.WithDetail("invalid cloud_account_id")
	}
	resource, creds, err := s.cloudAccountCredentials(ctx, orgID, accountID)
	if err != nil {
		return nil, err
	}
	if resource.Provider != "aws" {
		return nil, apierr.ErrValidation.WithDetail("selected cloud account is not an AWS account")
	}

	region := in.Region
	if region == "" {
		// Resolve the bucket's real region so the regional endpoint is correct.
		// This call needs s3:GetBucketLocation; under least-privilege IAM it may
		// be denied. Fall back to the account's default region, but never guess a
		// region silently — a wrong region/endpoint here would be persisted and
		// later break every backup against this bucket.
		r, lerr := pagesaws.New().BucketRegion(ctx, creds, in.Bucket)
		switch {
		case lerr == nil:
			region = r
		case creds.DefaultRegion != "":
			region = creds.DefaultRegion
		default:
			return nil, apierr.ErrValidation.WithDetail(fmt.Sprintf(
				"could not determine the region for bucket %q (%v); specify a region explicitly or set a default region on the cloud account",
				in.Bucket, lerr))
		}
	}

	out := map[string]string{
		"cloud_account_id": in.CloudAccountID,
		"bucket":           in.Bucket,
		"region":           region,
		"endpoint":         fmt.Sprintf("https://s3.%s.amazonaws.com", region),
	}
	expanded, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return expanded, nil
}

// resolveObjectStorageConfig returns a provider-ready object-storage config. For
// aws_s3 resources that reference a cloud account, it injects that account's
// current credentials at call time rather than relying on a stale copy, so IAM
// key rotation on the account takes effect without re-saving every derived
// resource. Configs without a cloud_account_id (manual entry, other providers)
// pass through unchanged.
//
// orgID is the org that owns the object-storage resource being resolved. The
// referenced cloud account MUST belong to that same org: this is the only
// runtime guard against a cross-org cloud_account_id (the write path validates
// ownership, but a forged or future-written reference would otherwise inject
// another tenant's IAM keys here, since SharedResources.GetByID is not
// org-scoped). A mismatch is treated as "not found" so this never leaks the
// existence of another tenant's account.
func resolveObjectStorageConfig(ctx context.Context, st store.Store, orgID uuid.UUID, provider string, cfg json.RawMessage) (json.RawMessage, error) {
	if provider != "aws_s3" || len(cfg) == 0 {
		return cfg, nil
	}
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return cfg, nil
	}
	idStr, _ := m["cloud_account_id"].(string)
	if idStr == "" {
		return cfg, nil
	}
	accountID, err := uuid.Parse(idStr)
	if err != nil {
		return nil, fmt.Errorf("invalid cloud_account_id")
	}
	account, err := st.SharedResources().GetByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("load cloud account: %w", err)
	}
	if account.OrgID != orgID {
		return nil, fmt.Errorf("cloud account not found")
	}
	var creds pages.Credentials
	if err := json.Unmarshal(account.Config, &creds); err != nil {
		return nil, fmt.Errorf("invalid cloud account config")
	}
	if creds.UseStaticKeys() {
		m["access_key"] = creds.AccessKeyID
		m["secret_key"] = creds.SecretAccessKey
	} else {
		// Role-based account (EC2 instance role / assume role): resolve concrete,
		// short-lived credentials now so both the SDK path and the in-cluster
		// aws-cli Job get usable keys plus a session token. Never persisted —
		// resolution happens fresh on every use.
		region, _ := m["region"].(string)
		if region == "" {
			region = creds.DefaultRegion
		}
		ak, sk, token, rerr := creds.ResolveCredentials(ctx, region)
		if rerr != nil {
			return nil, fmt.Errorf("resolve credentials for cloud account %q: %w", account.Name, rerr)
		}
		m["access_key"] = ak
		m["secret_key"] = sk
		if token != "" {
			m["session_token"] = token
		}
	}
	out, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListRepos fetches repositories from a git provider using its stored token,
// refreshing the token first when the provider supports it.
func (s *ResourceService) ListRepos(ctx context.Context, orgID, resourceID uuid.UUID) ([]GitRepo, error) {
	resource, err := s.getOwnedResource(ctx, orgID, resourceID)
	if err != nil {
		return nil, err
	}
	if resource.Type != model.ResourceGitProvider {
		return nil, fmt.Errorf("resource is not a git provider")
	}

	gp, err := s.providers.Git(resource.Provider)
	if err != nil {
		return nil, err
	}

	// Refresh credentials if the provider rotated them (e.g. OAuth token nearing
	// expiry) and persist the updated config.
	if updated, changed, rerr := gp.Refresh(ctx, resource.Config); rerr != nil {
		s.logger.Error("failed to refresh git token", slog.Any("error", rerr), slog.String("resource", resource.Name))
	} else if changed {
		resource.Config = updated
		if uerr := s.store.SharedResources().Update(ctx, resource); uerr != nil {
			s.logger.Error("failed to persist refreshed git token", slog.Any("error", uerr), slog.String("resource", resource.Name))
		}
	}

	return gp.ListRepos(ctx, resource.Config)
}

// SearchRepos queries repositories from a git provider by name (server-side
// search). Token refresh logic mirrors ListRepos.
func (s *ResourceService) SearchRepos(ctx context.Context, orgID, resourceID uuid.UUID, query string) ([]GitRepo, error) {
	resource, err := s.getOwnedResource(ctx, orgID, resourceID)
	if err != nil {
		return nil, err
	}
	if resource.Type != model.ResourceGitProvider {
		return nil, fmt.Errorf("resource is not a git provider")
	}

	gp, err := s.providers.Git(resource.Provider)
	if err != nil {
		return nil, err
	}

	if updated, changed, rerr := gp.Refresh(ctx, resource.Config); rerr != nil {
		s.logger.Error("failed to refresh git token", slog.Any("error", rerr), slog.String("resource", resource.Name))
	} else if changed {
		resource.Config = updated
		if uerr := s.store.SharedResources().Update(ctx, resource); uerr != nil {
			s.logger.Error("failed to persist refreshed git token", slog.Any("error", uerr), slog.String("resource", resource.Name))
		}
	}

	return gp.SearchRepos(ctx, resource.Config, query)
}

func (s *ResourceService) cloudAccountCredentials(ctx context.Context, orgID, id uuid.UUID) (*model.SharedResource, pages.Credentials, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return nil, pages.Credentials{}, err
	}
	if resource.Type != model.ResourceCloudAccount {
		return nil, pages.Credentials{}, fmt.Errorf("resource is not a cloud account")
	}
	var creds pages.Credentials
	if err := json.Unmarshal(resource.Config, &creds); err != nil {
		return nil, pages.Credentials{}, fmt.Errorf("invalid config")
	}
	return resource, creds, nil
}

// dnsProvider resolves the DNS provider for a cloud_account resource based on
// its stored provider key, so unsupported accounts fail with a clear message
// instead of an opaque credential error.
func (s *ResourceService) dnsProvider(ctx context.Context, orgID, id uuid.UUID) (dns.Provider, json.RawMessage, error) {
	resource, err := s.getOwnedResource(ctx, orgID, id)
	if err != nil {
		return nil, nil, err
	}
	if resource.Type != model.ResourceCloudAccount {
		return nil, nil, fmt.Errorf("resource is not a cloud account")
	}
	prov, err := dns.For(resource.Provider)
	if err != nil {
		return nil, nil, err
	}
	return prov, resource.Config, nil
}

// DNSZones lists hosted zones for a cloud_account resource.
func (s *ResourceService) DNSZones(ctx context.Context, orgID, id uuid.UUID) ([]dns.Zone, error) {
	prov, cfg, err := s.dnsProvider(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return prov.ListZones(ctx, cfg)
}

// DNSRecords lists DNS records in a hosted zone.
func (s *ResourceService) DNSRecords(ctx context.Context, orgID, id uuid.UUID, zoneID string) ([]dns.Record, error) {
	prov, cfg, err := s.dnsProvider(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	return prov.ListRecords(ctx, cfg, zoneID)
}

// DNSUpsertRecordInput is the body for creating or updating a DNS record.
type DNSUpsertRecordInput struct {
	ZoneID string   `json:"zone_id" binding:"required"`
	Name   string   `json:"name" binding:"required"`
	Type   string   `json:"type" binding:"required"`
	TTL    int64    `json:"ttl"`
	Values []string `json:"values" binding:"required,min=1"`
}

// DNSUpsertRecord creates or updates a DNS record.
func (s *ResourceService) DNSUpsertRecord(ctx context.Context, orgID, id uuid.UUID, input DNSUpsertRecordInput) error {
	prov, cfg, err := s.dnsProvider(ctx, orgID, id)
	if err != nil {
		return err
	}
	rec := dns.Record{
		Name:   input.Name,
		Type:   input.Type,
		TTL:    input.TTL,
		Values: input.Values,
	}
	return prov.UpsertRecord(ctx, cfg, input.ZoneID, rec)
}

// DNSDeleteRecordInput is the body for deleting a DNS record.
type DNSDeleteRecordInput struct {
	ZoneID string   `json:"zone_id" binding:"required"`
	Name   string   `json:"name" binding:"required"`
	Type   string   `json:"type" binding:"required"`
	TTL    int64    `json:"ttl"`
	Values []string `json:"values" binding:"required,min=1"`
}

// DNSDeleteRecord removes a DNS record.
func (s *ResourceService) DNSDeleteRecord(ctx context.Context, orgID, id uuid.UUID, input DNSDeleteRecordInput) error {
	prov, cfg, err := s.dnsProvider(ctx, orgID, id)
	if err != nil {
		return err
	}
	rec := dns.Record{
		Name:   input.Name,
		Type:   input.Type,
		TTL:    input.TTL,
		Values: input.Values,
	}
	if err := prov.DeleteRecord(ctx, cfg, input.ZoneID, rec); err != nil {
		return err
	}

	s.notifSvc.NotifyResourceDeleted(orgID, model.EventDNSRecordDeleted,
		input.Name, fmt.Sprintf("DNS record %q (%s) was deleted", input.Name, input.Type))
	return nil
}
