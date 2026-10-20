package config

import (
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) string {
	return func(key string) string { return vars[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(env(map[string]string{"ANALYZER_URL": "http://localhost:8081/"}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Port: "8080", AnalyzerURL: "http://localhost:8081", AnalyzerTimeout: 10 * time.Second, AnalyzerAuth: AuthNone}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadOverrides(t *testing.T) {
	cfg, err := Load(env(map[string]string{
		"PORT":             "9000",
		"ANALYZER_URL":     "https://text-analyzer-abc-ew.a.run.app",
		"ANALYZER_TIMEOUT": "2500ms",
		"ANALYZER_AUTH":    "google-id-token",
	}))
	if err != nil {
		t.Fatal(err)
	}
	want := Config{Port: "9000", AnalyzerURL: "https://text-analyzer-abc-ew.a.run.app", AnalyzerTimeout: 2500 * time.Millisecond, AnalyzerAuth: AuthGoogleIDToken}
	if cfg != want {
		t.Fatalf("got %+v, want %+v", cfg, want)
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]struct {
		vars    map[string]string
		mention string
	}{
		"missing url":      {map[string]string{}, "ANALYZER_URL"},
		"relative url":     {map[string]string{"ANALYZER_URL": "localhost:8081"}, "ANALYZER_URL"},
		"ftp url":          {map[string]string{"ANALYZER_URL": "ftp://host"}, "ANALYZER_URL"},
		"no host":          {map[string]string{"ANALYZER_URL": "http://"}, "ANALYZER_URL"},
		"bad port":         {map[string]string{"ANALYZER_URL": "http://a", "PORT": "http"}, "PORT"},
		"port range":       {map[string]string{"ANALYZER_URL": "http://a", "PORT": "70000"}, "PORT"},
		"bad timeout":      {map[string]string{"ANALYZER_URL": "http://a", "ANALYZER_TIMEOUT": "10"}, "ANALYZER_TIMEOUT"},
		"negative timeout": {map[string]string{"ANALYZER_URL": "http://a", "ANALYZER_TIMEOUT": "-1s"}, "ANALYZER_TIMEOUT"},
		"unknown auth":     {map[string]string{"ANALYZER_URL": "http://a", "ANALYZER_AUTH": "basic"}, "ANALYZER_AUTH"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := Load(env(tc.vars))
			if err == nil || !strings.Contains(err.Error(), tc.mention) {
				t.Fatalf("got %v, want an error naming %s", err, tc.mention)
			}
		})
	}
}
