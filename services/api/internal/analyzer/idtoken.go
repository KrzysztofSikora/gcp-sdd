package analyzer

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// DefaultMetadataURL is the Cloud Run metadata server endpoint that issues Google-signed ID
// tokens for the service's own service account.
const DefaultMetadataURL = "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/identity"

const (
	refreshBefore    = 5 * time.Minute  // fetch a new token this long before the old one expires
	fallbackTokenTTL = 10 * time.Minute // used when the token's expiry cannot be read
)

// IDTokenTransport adds "Authorization: Bearer <ID token>" to every request, so that a Cloud
// Run service deployed with --no-allow-unauthenticated accepts it (Q-01 = A, FR-07 AC4).
// Tokens come from the metadata server and are reused until shortly before they expire.
type IDTokenTransport struct {
	Base        http.RoundTripper // transport for both the token fetch and the request
	Audience    string            // the analyzer's URL
	MetadataURL string
	Now         func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

// NewIDTokenTransport returns a transport for audience using the default metadata server.
func NewIDTokenTransport(audience string, base http.RoundTripper) *IDTokenTransport {
	if base == nil {
		base = http.DefaultTransport
	}
	return &IDTokenTransport{Base: base, Audience: audience, MetadataURL: DefaultMetadataURL, Now: time.Now}
}

// RoundTrip implements http.RoundTripper.
func (t *IDTokenTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	token, err := t.currentToken(req.Context())
	if err != nil {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, fmt.Errorf("fetch ID token: %w", err)
	}
	authed := req.Clone(req.Context())
	authed.Header.Set("Authorization", "Bearer "+token)
	return t.Base.RoundTrip(authed)
}

func (t *IDTokenTransport) currentToken(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := t.Now()
	if t.token != "" && now.Before(t.expires.Add(-refreshBefore)) {
		return t.token, nil
	}
	token, err := t.fetch(ctx)
	if err != nil {
		return "", err
	}
	expires, ok := tokenExpiry(token)
	if !ok {
		expires = now.Add(fallbackTokenTTL + refreshBefore)
	}
	t.token, t.expires = token, expires
	return token, nil
}

func (t *IDTokenTransport) fetch(ctx context.Context) (string, error) {
	u := t.MetadataURL + "?audience=" + url.QueryEscape(t.Audience)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<10))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("metadata server status %d", resp.StatusCode)
	}
	token := strings.TrimSpace(string(body))
	if token == "" {
		return "", fmt.Errorf("metadata server returned an empty token")
	}
	return token, nil
}

// tokenExpiry reads the "exp" claim of a JWT without verifying it (the analyzer's platform
// verifies the token; we only need to know when to refresh it).
func tokenExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if json.Unmarshal(payload, &claims) != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}
