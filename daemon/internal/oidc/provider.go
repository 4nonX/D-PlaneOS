package oidc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// Reasonable bounds so a hostile or broken IdP cannot exhaust memory or stall
// the login path indefinitely.
const (
	maxDiscoveryBody = 1 << 20 // 1 MiB
	maxJWKSBody      = 1 << 20 // 1 MiB
	httpTimeout      = 10 * time.Second
	discoveryTTL     = 1 * time.Hour
	jwksTTL          = 15 * time.Minute
	// jwksMinRefreshInterval throttles forced refreshes triggered by an unknown
	// kid, so a stream of tokens with bogus kids cannot hammer the JWKS URI.
	jwksMinRefreshInterval = 1 * time.Minute
)

// DiscoveryDocument is the subset of the OpenID Provider Metadata that
// DPlaneOS consumes. Unknown fields are ignored.
type DiscoveryDocument struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	EndSessionEndpoint    string `json:"end_session_endpoint"`
}

// KeySource resolves a verified signing key by its key id. The verifier depends
// only on this interface, which keeps the signature-verification logic testable
// without network access.
type KeySource interface {
	KeyForKID(ctx context.Context, kid string) (*jose.JSONWebKey, error)
}

// Provider performs discovery against a single OIDC issuer and caches both the
// discovery document and the provider's JWKS, refreshing the key set on
// rotation. It is safe for concurrent use.
type Provider struct {
	issuer     string
	httpClient *http.Client

	mu sync.RWMutex

	disco       *DiscoveryDocument
	discoExpiry time.Time

	keys           []jose.JSONWebKey
	keysExpiry     time.Time
	keysLastFetch  time.Time
	keysJWKSURIRef string
}

// NewProvider creates a Provider for the given issuer. If httpClient is nil a
// client with a sane timeout is used. The issuer must not have a trailing
// slash unless the IdP's published issuer string actually contains one; it is
// compared verbatim against the discovery document.
func NewProvider(issuer string, httpClient *http.Client) (*Provider, error) {
	issuer = strings.TrimSpace(issuer)
	if issuer == "" {
		return nil, ErrConfigInvalid
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: httpTimeout}
	}
	return &Provider{issuer: issuer, httpClient: httpClient}, nil
}

// Discovery returns the cached discovery document, fetching it if absent or
// expired. The returned pointer must be treated as read-only by callers.
func (p *Provider) Discovery(ctx context.Context) (*DiscoveryDocument, error) {
	p.mu.RLock()
	if p.disco != nil && time.Now().Before(p.discoExpiry) {
		d := p.disco
		p.mu.RUnlock()
		return d, nil
	}
	p.mu.RUnlock()
	return p.refreshDiscovery(ctx)
}

func (p *Provider) refreshDiscovery(ctx context.Context) (*DiscoveryDocument, error) {
	url := strings.TrimRight(p.issuer, "/") + "/.well-known/openid-configuration"
	body, err := p.getJSON(ctx, url, maxDiscoveryBody)
	if err != nil {
		return nil, fmt.Errorf("oidc: discovery fetch: %w", err)
	}

	var doc DiscoveryDocument
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("oidc: discovery decode: %w", err)
	}
	// RFC 8414 / OIDC Discovery 1.0: the issuer in the document MUST match the
	// issuer used to construct the request. This prevents a malicious metadata
	// host from impersonating a different issuer.
	if doc.Issuer != p.issuer {
		return nil, ErrDiscoveryIssuer
	}
	if doc.JWKSURI == "" || doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return nil, fmt.Errorf("oidc: discovery document missing required endpoints")
	}

	p.mu.Lock()
	p.disco = &doc
	p.discoExpiry = time.Now().Add(discoveryTTL)
	p.mu.Unlock()
	return &doc, nil
}

// KeyForKID resolves a signing key by kid. On a cache miss it refreshes the
// JWKS once (subject to a throttle) to handle key rotation, then retries. If
// kid is empty and the provider publishes exactly one key, that key is
// returned; otherwise an empty kid is an error to avoid ambiguous selection.
func (p *Provider) KeyForKID(ctx context.Context, kid string) (*jose.JSONWebKey, error) {
	if key := p.lookupKey(kid); key != nil {
		return key, nil
	}
	if err := p.refreshKeysThrottled(ctx); err != nil {
		return nil, err
	}
	if key := p.lookupKey(kid); key != nil {
		return key, nil
	}
	return nil, ErrKeyNotFound
}

// lookupKey returns a matching key from the cache, or nil. A nil result also
// occurs when the cache is empty or expired, forcing the caller to refresh.
func (p *Provider) lookupKey(kid string) *jose.JSONWebKey {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.keys) == 0 || time.Now().After(p.keysExpiry) {
		return nil
	}
	if kid == "" {
		if len(p.keys) == 1 {
			k := p.keys[0]
			return &k
		}
		return nil
	}
	for i := range p.keys {
		if p.keys[i].KeyID == kid {
			k := p.keys[i]
			return &k
		}
	}
	return nil
}

// refreshKeysThrottled refreshes the JWKS, but no more often than
// jwksMinRefreshInterval to bound the cost of unknown-kid floods.
func (p *Provider) refreshKeysThrottled(ctx context.Context) error {
	p.mu.RLock()
	last := p.keysLastFetch
	cacheValid := len(p.keys) > 0 && time.Now().Before(p.keysExpiry)
	p.mu.RUnlock()

	if cacheValid && time.Since(last) < jwksMinRefreshInterval {
		// Cache is still fresh and we refreshed very recently; do not hammer
		// the IdP. The caller will get ErrKeyNotFound, which is correct.
		return nil
	}
	return p.refreshKeys(ctx)
}

func (p *Provider) refreshKeys(ctx context.Context) error {
	doc, err := p.Discovery(ctx)
	if err != nil {
		return err
	}
	body, err := p.getJSON(ctx, doc.JWKSURI, maxJWKSBody)
	if err != nil {
		return fmt.Errorf("oidc: jwks fetch: %w", err)
	}
	var set jose.JSONWebKeySet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("oidc: jwks decode: %w", err)
	}
	if len(set.Keys) == 0 {
		return fmt.Errorf("oidc: jwks contained no keys")
	}

	// Keep only public keys usable for signature verification. Drop anything
	// without a usable public key so verification never sees a private or
	// malformed key.
	usable := make([]jose.JSONWebKey, 0, len(set.Keys))
	for _, k := range set.Keys {
		if k.Key == nil {
			continue
		}
		if k.Use != "" && k.Use != "sig" {
			continue
		}
		usable = append(usable, k.Public())
	}
	if len(usable) == 0 {
		return fmt.Errorf("oidc: jwks contained no usable signing keys")
	}

	p.mu.Lock()
	p.keys = usable
	p.keysExpiry = time.Now().Add(jwksTTL)
	p.keysLastFetch = time.Now()
	p.keysJWKSURIRef = doc.JWKSURI
	p.mu.Unlock()
	return nil
}

// getJSON performs a bounded GET and returns the body, enforcing a 2xx status
// and a body size limit. It does not require a JSON content type because some
// IdPs serve discovery/JWKS with imprecise types, but the body is parsed as
// JSON by callers.
func (p *Provider) getJSON(ctx context.Context, url string, maxBytes int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("unexpected status %d from %s", resp.StatusCode, url)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(body)) > maxBytes {
		return nil, fmt.Errorf("response body from %s exceeds %d bytes", url, maxBytes)
	}
	return body, nil
}
