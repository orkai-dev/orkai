package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/orkai-dev/orkai/apps/api/internal/dns"
	"github.com/orkai-dev/orkai/apps/api/internal/model"
	"github.com/orkai-dev/orkai/apps/api/internal/store"
)

const acmCertWaitTimeout = 10 * time.Minute

// normalizeDomain lowercases and strips trailing dots/spaces from a domain name.
func normalizeDomain(domain string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(domain)), ".")
}

// recordExistsForDomain reports whether zone already has a record for domain.
func recordExistsForDomain(records []dns.Record, domain string) bool {
	want := normalizeDomain(domain)
	for _, rec := range records {
		if normalizeDomain(rec.Name) == want {
			return true
		}
	}
	return false
}

// validateCustomDomain checks domain availability at page creation/update time.
func (s *PageService) validateCustomDomain(
	ctx context.Context,
	orgID uuid.UUID,
	pageProvider model.PageProvider,
	domain string,
	manageDNS bool,
	dnsAccountID *uuid.UUID,
	dnsZoneID string,
) error {
	domain = normalizeDomain(domain)
	if domain == "" {
		return nil
	}

	// Intentionally not checking public DNS resolution here — domains that
	// already point elsewhere are valid; users migrate by cutting over DNS
	// after provisioning. The Route53-zone duplicate check below covers the
	// manage_dns=true path.
	if !manageDNS {
		return nil
	}
	if dnsAccountID == nil || strings.TrimSpace(dnsZoneID) == "" {
		return fmt.Errorf("dns_account_id and dns_zone_id are required when manage_dns is enabled")
	}
	if err := s.validateResource(ctx, orgID, *dnsAccountID, model.ResourceCloudAccount); err != nil {
		return err
	}
	if err := validatePagesDNSAccount(ctx, s.store, pageProvider, *dnsAccountID); err != nil {
		return err
	}

	prov, cfg, err := resolveDNSProvider(ctx, s.store, *dnsAccountID)
	if err != nil {
		return err
	}
	records, err := prov.ListRecords(ctx, cfg, dnsZoneID)
	if err != nil {
		return fmt.Errorf("list DNS records: %w", err)
	}
	if recordExistsForDomain(records, domain) {
		return fmt.Errorf("domain %q is not available — a DNS record already exists in the selected zone", domain)
	}
	return nil
}

// validatePagesDNSAccount ensures a cloud_account can manage DNS for a Page.
// AWS CloudFront pages need Route53 (alias records); Cloudflare Pages use CNAME.
func validatePagesDNSAccount(ctx context.Context, st store.Store, pageProvider model.PageProvider, dnsAccountID uuid.UUID) error {
	res, err := st.SharedResources().GetByID(ctx, dnsAccountID)
	if err != nil {
		return fmt.Errorf("dns account not found: %w", err)
	}
	switch pageProvider {
	case model.PageProviderCloudflarePages:
		if res.Provider == "cloudflare" {
			return nil
		}
		return fmt.Errorf(
			"managed DNS for Cloudflare Pages requires a Cloudflare cloud account; the %q provider is not supported",
			res.Provider,
		)
	default:
		switch res.Provider {
		case "", "aws", "route53":
			return nil
		default:
			return fmt.Errorf(
				"managed DNS for pages requires an AWS (Route53) cloud account; the %q provider is not supported because CloudFront-hosted pages use Route53 alias records",
				res.Provider,
			)
		}
	}
}

// resolveDNSProvider loads DNS credentials from a cloud_account resource.
func resolveDNSProvider(ctx context.Context, st store.Store, dnsAccountID uuid.UUID) (dns.Provider, json.RawMessage, error) {
	res, err := st.SharedResources().GetByID(ctx, dnsAccountID)
	if err != nil {
		return nil, nil, fmt.Errorf("dns account not found: %w", err)
	}
	if res.Type != model.ResourceCloudAccount {
		return nil, nil, fmt.Errorf("resource %s is not a cloud account", res.Name)
	}
	prov, err := dns.For(res.Provider)
	if err != nil {
		return nil, nil, err
	}
	return prov, res.Config, nil
}

// pagesDevHostFromURL strips scheme from a default hosting URL.
func pagesDevHostFromURL(defaultURL string) string {
	u := strings.TrimSpace(defaultURL)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	return strings.TrimSuffix(u, "/")
}

func cloudFrontDomainFromURL(defaultURL string) string { return pagesDevHostFromURL(defaultURL) }
