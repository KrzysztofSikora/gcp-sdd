package analyzer

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func jwtExpiring(exp time.Time) string {
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"RS256"}`)) + "." + enc([]byte(fmt.Sprintf(`{"exp":%d}`, exp.Unix()))) + ".sig"
}

type metadataServer struct {
	*httptest.Server
	fetches  atomic.Int32
	audience atomic.Value
	token    func(n int32) string
	status   int
}

func newMetadataServer(t *testing.T, token func(n int32) string) *metadataServer {
	t.Helper()
	m := &metadataServer{token: token, status: http.StatusOK}
	m.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Metadata-Flavor") != "Google" {
			http.Error(w, "missing Metadata-Flavor", http.StatusForbidden)
			return
		}
		n := m.fetches.Add(1)
		m.audience.Store(r.URL.Query().Get("audience"))
		w.WriteHeader(m.status)
		_, _ = io.WriteString(w, m.token(n))
	}))
	t.Cleanup(m.Close)
	return m
}

// analyzerEcho answers the contract response and records the Authorization headers it saw.
func analyzerEcho(t *testing.T, seen *[]string) *httptest.Server {
	return server(t, func(w http.ResponseWriter, r *http.Request) {
		*seen = append(*seen, r.Header.Get("Authorization"))
		_, _ = io.WriteString(w, contractResponse)
	})
}

func TestIDTokenAddedAndCached(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	meta := newMetadataServer(t, func(n int32) string { return jwtExpiring(now.Add(time.Hour)) + fmt.Sprint(n) })
	var seen []string
	srv := analyzerEcho(t, &seen)

	tr := NewIDTokenTransport(srv.URL, nil)
	tr.MetadataURL, tr.Now = meta.URL, func() time.Time { return now }
	client := New(srv.URL, time.Second, tr)

	for i := 0; i < 3; i++ {
		if _, err := client.Analyze(context.Background(), "a"); err != nil {
			t.Fatal(err)
		}
	}
	if got := meta.fetches.Load(); got != 1 {
		t.Errorf("token fetches = %d, want 1 (cached)", got)
	}
	if got := meta.audience.Load(); got != srv.URL {
		t.Errorf("audience = %v, want %s", got, srv.URL)
	}
	want := "Bearer " + jwtExpiring(now.Add(time.Hour)) + "1"
	for _, h := range seen {
		if h != want {
			t.Errorf("Authorization = %q, want %q", h, want)
		}
	}
}

func TestIDTokenRefreshedBeforeExpiry(t *testing.T) {
	now := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	meta := newMetadataServer(t, func(int32) string { return jwtExpiring(now.Add(time.Hour)) })
	var seen []string
	srv := analyzerEcho(t, &seen)

	clock := now
	tr := NewIDTokenTransport(srv.URL, nil)
	tr.MetadataURL, tr.Now = meta.URL, func() time.Time { return clock }
	client := New(srv.URL, time.Second, tr)

	_, _ = client.Analyze(context.Background(), "a")
	clock = now.Add(56 * time.Minute) // within 5 minutes of expiry
	_, _ = client.Analyze(context.Background(), "a")
	if got := meta.fetches.Load(); got != 2 {
		t.Errorf("token fetches = %d, want 2", got)
	}
}

func TestIDTokenFailureIsUnavailable(t *testing.T) {
	meta := newMetadataServer(t, func(int32) string { return "denied" })
	meta.status = http.StatusForbidden
	var seen []string
	srv := analyzerEcho(t, &seen)

	tr := NewIDTokenTransport(srv.URL, nil)
	tr.MetadataURL = meta.URL
	_, err := New(srv.URL, time.Second, tr).Analyze(context.Background(), "a")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if len(seen) != 0 {
		t.Errorf("analyzer was called without a token")
	}
}

func TestNoTokenWithoutTransport(t *testing.T) {
	var seen []string
	srv := analyzerEcho(t, &seen)
	if _, err := New(srv.URL, time.Second, nil).Analyze(context.Background(), "a"); err != nil {
		t.Fatal(err)
	}
	if seen[0] != "" {
		t.Errorf("Authorization = %q, want none", seen[0])
	}
}

func TestTokenExpiryFallback(t *testing.T) {
	if _, ok := tokenExpiry("not-a-jwt"); ok {
		t.Error("opaque token should have no readable expiry")
	}
	exp := time.Unix(1_800_000_000, 0)
	if got, ok := tokenExpiry(jwtExpiring(exp)); !ok || !got.Equal(exp) {
		t.Errorf("expiry = %v %v, want %v", got, ok, exp)
	}
}
